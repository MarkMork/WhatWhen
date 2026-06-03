package main

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed web
var webFS embed.FS

const maxLabelLen = 100

func registerRoutes(mux *http.ServeMux, store *Store) {
	// Static UI, served from the embedded web/ directory.
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("GET /", http.FileServer(http.FS(sub)))

	// API
	mux.HandleFunc("GET /api/items", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, store.List())
	})

	mux.HandleFunc("POST /api/items", func(w http.ResponseWriter, r *http.Request) {
		label, ok := decodeLabel(w, r)
		if !ok {
			return
		}
		item, err := store.Add(label)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusCreated, item)
	})

	mux.HandleFunc("PATCH /api/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		label, ok := decodeLabel(w, r)
		if !ok {
			return
		}
		item, err := store.Edit(r.PathValue("id"), label)
		if respondMaybeNotFound(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("POST /api/items/{id}/reset", func(w http.ResponseWriter, r *http.Request) {
		item, err := store.Reset(r.PathValue("id"))
		if respondMaybeNotFound(w, err) {
			return
		}
		writeJSON(w, http.StatusOK, item)
	})

	mux.HandleFunc("DELETE /api/items/{id}", func(w http.ResponseWriter, r *http.Request) {
		err := store.Delete(r.PathValue("id"))
		if respondMaybeNotFound(w, err) {
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// decodeLabel reads and validates a {"label": "..."} body.
func decodeLabel(w http.ResponseWriter, r *http.Request) (string, bool) {
	var body struct {
		Label string `json:"label"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, errors.New("invalid JSON body"))
		return "", false
	}
	label := strings.TrimSpace(body.Label)
	if label == "" {
		writeError(w, http.StatusBadRequest, errors.New("label is required"))
		return "", false
	}
	if len(label) > maxLabelLen {
		label = label[:maxLabelLen]
	}
	return label, true
}

// respondMaybeNotFound writes an error response for err and reports whether the
// handler should stop. It returns false only when err is nil.
func respondMaybeNotFound(w http.ResponseWriter, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, err)
	} else {
		writeError(w, http.StatusInternalServerError, err)
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
