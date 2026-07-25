# server

The real backend for The Block: a Go REST + WebSocket API backed by Redis (atomic bidding, realtime fan-out) and Postgres (durable system of record, audit trail, filter/sort/pagination).

See [`guidelines/06-backend-architecture.md`](../guidelines/06-backend-architecture.md) for the full design and the reasoning behind it.

## How to Run

Requires Docker Desktop (WSL2 backend, on Windows). From this directory:

```
./deploy/docker/setup-ssl-windows.sh   # once per machine
docker compose up --build
```

> App will be accessible on https://localhost (no port)


`docker compose up` builds and starts, in order: `postgres` and `redis`; the one-shot `reconcile` service (applies Postgres migrations, then syncs `../data/vehicles.json` into Postgres and Redis, insert-only-new by id — `guidelines/06-backend-architecture.md`, "Data lifecycle"); then `app` (the API) and `client` (the existing Vite dev server); then `caddy`, which is the only service that actually publishes a host port. Open `https://localhost` — Caddy reverse-proxies `/v1/*` (REST and the `/v1/ws` WebSocket upgrade alike — `gorilla/websocket` upgrades transparently through `reverse_proxy`) and `/healthcheck` to `app`, and everything else to `client`, all on one origin (this is what keeps the session cookie's `SameSite=Lax` protection meaningful — a split origin would force `SameSite=None`).

Re-running `docker compose up` at any point — including after regenerating `../data/vehicles.json` (`node ../scripts/generate_vehicles.mjs`, which randomizes its seed by default) — is safe: `reconcile` only ever inserts listings Postgres doesn't already have, so a listing with real bids against it is never touched or deleted.

### SSL setup (Windows)

`deploy/docker/setup-ssl-windows.sh` requires [mkcert](https://github.com/FiloSottile/mkcert) (`winget install --id=FiloSottile.mkcert -e` if you don't have it — the script checks and prints this itself). It runs `mkcert -install` (installs mkcert's local CA into the Windows + browser trust stores — a host-level step Compose can't perform itself) and writes `deploy/docker/certs/localhost.pem`/`localhost-key.pem`, which Caddy loads read-only. Safe to re-run any time.

`deploy/docker/cleanup-ssl-windows.sh` removes the generated cert/key. It does **not** run `mkcert -uninstall` by default, since that would remove the local CA from the entire machine's trust store, possibly affecting other local-HTTPS projects — pass `--uninstall-ca` to opt into that too.

## Running the Go tests without Docker

Each package builds and tests independently:

```
gofmt -l .          # expect no output
go vet ./...
go test ./...
```

The `test/integration` suite (`//go:build integration`) exercises real Redis and Postgres via `testcontainers-go`, so it needs a running Docker daemon but not `docker compose up` itself — it starts and tears down its own throwaway containers per test run:

```
go test -tags=integration ./test/integration/...
```

This is what proves concurrent-bid correctness (N goroutines racing bids on one listing resolve to a consistent serial ordering, exactly one Postgres row per accepted bid, no duplicates or gaps after the stream tailer drains), the cursor-pagination walk (forward/backward round-trips, including across the "ending soonest" sort's bucket boundaries), and the WebSocket upgrade path end-to-end.
