"use strict";

const grid = document.getElementById("grid");
const empty = document.getElementById("empty");
const addForm = document.getElementById("add-form");
const addInput = document.getElementById("add-input");
const lockToggle = document.getElementById("lock-toggle");

// When locked (the default), edit/delete and timestamp editing are hidden so the
// everyday view stays clean. Preference is remembered in the browser.
let unlocked = localStorage.getItem("whatwhen-unlocked") === "1";

function applyLock() {
  document.body.classList.toggle("unlocked", unlocked);
  lockToggle.setAttribute("aria-pressed", String(unlocked));
  lockToggle.querySelector(".lock-icon").textContent = unlocked ? "🔓" : "🔒";
  lockToggle.querySelector(".lock-text").textContent = unlocked ? "Unlocked" : "Locked";
}

lockToggle.addEventListener("click", () => {
  unlocked = !unlocked;
  localStorage.setItem("whatwhen-unlocked", unlocked ? "1" : "0");
  applyLock();
});

// In-memory mirror of the server state. The DOM is rebuilt from this and the
// timers tick locally against each item's lastReset timestamp.
let items = [];

async function api(path, options) {
  const res = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...options,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const body = await res.json();
      if (body && body.error) msg = body.error;
    } catch (_) {}
    throw new Error(msg);
  }
  if (res.status === 204) return null;
  return res.json();
}

// Format milliseconds elapsed into an adaptive, human-friendly string.
function formatElapsed(ms) {
  if (ms < 0) ms = 0;
  const s = Math.floor(ms / 1000);
  const m = Math.floor(s / 60);
  const h = Math.floor(m / 60);
  const d = Math.floor(h / 24);
  const w = Math.floor(d / 7);

  if (s < 60) return `${s}s`;
  if (m < 60) return `${m}m ${s % 60}s`;
  if (h < 24) return `${h}h ${m % 60}m`;
  if (d < 7) return `${d}d ${h % 24}h`;
  return `${w}w ${d % 7}d`;
}

function fmtDate(iso) {
  const dt = new Date(iso);
  return dt.toLocaleString(undefined, {
    dateStyle: "medium",
    timeStyle: "short",
  });
}

// Convert an ISO timestamp to the local "YYYY-MM-DDTHH:MM" value a
// datetime-local input expects.
function toLocalInputValue(iso) {
  const d = new Date(iso);
  const pad = (n) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
}

function render() {
  empty.classList.toggle("hidden", items.length > 0);

  grid.replaceChildren(
    ...items.map((item) => {
      const card = document.createElement("article");
      card.className = "card";
      card.dataset.id = item.id;

      const top = document.createElement("div");
      top.className = "card-top";

      const label = document.createElement("div");
      label.className = "card-label";
      label.textContent = item.label;

      const menu = document.createElement("div");
      menu.className = "card-menu";

      const editBtn = iconButton("✎", "Rename", () => startEdit(card, item));
      const delBtn = iconButton("🗑", "Delete", () => remove(item));
      delBtn.classList.add("danger");
      menu.append(editBtn, delBtn);
      top.append(label, menu);

      const timer = document.createElement("div");
      timer.className = "card-timer";
      timer.dataset.reset = item.lastReset;

      const sub = document.createElement("div");
      sub.className = "card-sub editable";
      sub.textContent = `Last done ${fmtDate(item.lastReset)}`;
      sub.title = "Edit when this was last done";
      sub.addEventListener("click", () => {
        if (unlocked) startEditTime(card, item);
      });

      const reset = document.createElement("button");
      reset.className = "btn btn-reset";
      reset.textContent = "Did it now";
      reset.addEventListener("click", () => resetItem(item));

      card.append(top, timer, sub, reset);
      return card;
    })
  );

  tick();
}

function iconButton(symbol, title, onClick) {
  const b = document.createElement("button");
  b.className = "icon-btn";
  b.type = "button";
  b.title = title;
  b.setAttribute("aria-label", title);
  b.textContent = symbol;
  b.addEventListener("click", onClick);
  return b;
}

// Update every visible timer against the current clock.
function tick() {
  const now = Date.now();
  grid.querySelectorAll(".card-timer").forEach((el) => {
    const reset = new Date(el.dataset.reset).getTime();
    el.textContent = formatElapsed(now - reset);
  });
}

function startEdit(card, item) {
  const label = card.querySelector(".card-label");
  const input = document.createElement("input");
  input.className = "edit-input";
  input.value = item.label;
  input.maxLength = 100;
  label.replaceWith(input);
  input.focus();
  input.select();

  let done = false;
  const commit = async () => {
    if (done) return;
    done = true;
    const next = input.value.trim();
    if (next && next !== item.label) {
      try {
        const updated = await api(`/api/items/${item.id}`, {
          method: "PATCH",
          body: JSON.stringify({ label: next }),
        });
        item.label = updated.label;
      } catch (e) {
        alert("Could not rename: " + e.message);
      }
    }
    render();
  };

  input.addEventListener("blur", commit);
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") commit();
    if (e.key === "Escape") {
      done = true;
      render();
    }
  });
}

function startEditTime(card, item) {
  const sub = card.querySelector(".card-sub");
  const input = document.createElement("input");
  input.type = "datetime-local";
  input.className = "time-input";
  input.value = toLocalInputValue(item.lastReset);
  sub.replaceWith(input);
  input.focus();

  let done = false;
  const commit = async () => {
    if (done) return;
    done = true;
    if (input.value) {
      const iso = new Date(input.value).toISOString();
      if (iso !== new Date(item.lastReset).toISOString()) {
        try {
          const updated = await api(`/api/items/${item.id}`, {
            method: "PATCH",
            body: JSON.stringify({ lastReset: iso }),
          });
          Object.assign(item, updated);
        } catch (e) {
          alert("Could not update time: " + e.message);
        }
      }
    }
    render();
  };

  input.addEventListener("blur", commit);
  input.addEventListener("keydown", (e) => {
    if (e.key === "Enter") commit();
    if (e.key === "Escape") {
      done = true;
      render();
    }
  });
}

async function resetItem(item) {
  try {
    const updated = await api(`/api/items/${item.id}/reset`, { method: "POST" });
    Object.assign(item, updated);
    render();
  } catch (e) {
    alert("Could not reset: " + e.message);
  }
}

async function remove(item) {
  if (!confirm(`Delete "${item.label}"?`)) return;
  try {
    await api(`/api/items/${item.id}`, { method: "DELETE" });
    items = items.filter((i) => i.id !== item.id);
    render();
  } catch (e) {
    alert("Could not delete: " + e.message);
  }
}

addForm.addEventListener("submit", async (e) => {
  e.preventDefault();
  const label = addInput.value.trim();
  if (!label) return;
  try {
    const item = await api("/api/items", {
      method: "POST",
      body: JSON.stringify({ label }),
    });
    items.push(item);
    addInput.value = "";
    render();
  } catch (err) {
    alert("Could not add: " + err.message);
  }
});

async function load() {
  try {
    items = (await api("/api/items")) || [];
  } catch (e) {
    items = [];
  }
  render();
}

applyLock();
load();
setInterval(tick, 1000);
