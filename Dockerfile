# --- build stage ---
FROM golang:1.22-alpine AS build
WORKDIR /src

# No external dependencies, so just copy the source and build a static binary.
COPY go.mod ./
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /whatwhen .

# --- final stage ---
FROM scratch
COPY --from=build /whatwhen /whatwhen

ENV PORT=8080
ENV DATA_FILE=/data/whatwhen.json
VOLUME ["/data"]
EXPOSE 8080

ENTRYPOINT ["/whatwhen"]
