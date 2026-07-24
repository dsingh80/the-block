# server

The real backend for The Block: a Go REST + WebSocket API backed by Redis (atomic bidding, realtime fan-out) and Postgres (durable system of record, audit trail, filter/sort/pagination).

See [`guidelines/06-backend-architecture.md`](../guidelines/06-backend-architecture.md) for the full design and the reasoning behind it.

## How to Run

Docker Compose setup (Redis, Postgres, the app, and Caddy for local HTTPS) lands later in this build sequence — this section will be filled in once `docker-compose.yml` exists. Until then, this directory is under active construction; individual packages build and test independently (`go build ./...`, `go test ./...`) but there is no runnable server yet.
