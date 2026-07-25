# Auto Auction Buyer App

A submission for OPENLANE's **"The Block"** coding challenge — the buyer side of a vehicle auction platform, backed by a real Go + Redis + Postgres backend and seeded from the 200-vehicle dataset in [`data/vehicles.json`](data/vehicles.json).

## How to Run

Read SUBMISSION.md

## Stack

- **Frontend:** Vue 3 (Composition API, `<script setup>`) + TypeScript + Vite + Pinia + vue-router. A `services/api/` layer wraps native `fetch` and `WebSocket` — no HTTP client library.
- **Backend:** Go, stdlib `net/http` (1.22+ pattern routing) + `gorilla/websocket`, Redis (`go-redis/v9`, Lua-scripted atomic bid accept, Streams for the realtime/audit fan-out) and Postgres (`pgx/v5`, `golang-migrate`) as a hybrid CQRS-lite system of record.
- **Deployment:** Docker Compose (Postgres, Redis, the API, the Vite dev server) behind Caddy, which terminates TLS and puts the client and API on one origin locally (`https://localhost`).
- **Styling:** hand-authored `<style scoped>` per component + a shared `tokens.css` of CSS custom properties — no inline styles anywhere, no Tailwind
- **Tooling:** ESLint + Prettier (hand-configured), Vitest + `@vue/test-utils` + `@pinia/testing` on the client; `gofmt` + `go vet` + `go test` (unit and `testcontainers-go`-backed Redis/Postgres integration tests) on the server

See [`guidelines/`](guidelines/) for the full engineering-conventions writeup behind these choices on both sides of the repo — locked-in stack decisions, design patterns, guardrails, and testing strategy for the client (`00`–`05`), and the equivalent decision log for the backend (`06-backend-architecture.md`: datastore choice, cursor pagination, sessions/CSRF, the WebSocket protocol, and more) — written before the code they govern, so later work in this repo (by me or by an AI agent) has a stated default instead of re-deciding things per feature.

## Notable Decisions

- **Tiered bid increment** (`< $5,000 → $100`, `< $15,000 → $250`, `≥ $15,000 → $500`) instead of a flat increment — a fixed step is a rounding error on a cheap listing and an odd granularity on an expensive one.
- **No inline styles anywhere.** Every dynamic visual state (grade, title status, badge, CTA, urgent timer, selection) is a finite set of CSS modifier classes bound via `:class`, backed by `tokens.css`. Chosen over Tailwind because introducing a framework would mean translating away from the design mock's own shape for no fidelity gain.
- **`augment()` is the single source of presentation truth** (`composables/useListingPresentation.ts`) — every component reads a fully-derived `AugmentedListing`; nothing re-derives a grade color or badge label inline. It stays reactive to a global clock store, so badges/status genuinely flip live as time passes rather than freezing at whatever moment a component rendered.
- **Global Pinia stores**, not local component state, for bids/watchlist/compare/filters — all of them need to survive navigating away from and back to a view, and the bidding store specifically needs to be visible everywhere at once (a card, the preview modal, the details page, and the watchlist drawer all reflect the same bid instantly).
- **Simplified proxy bidding.** The form is labeled "max proxy bid" to match the design, but a valid bid still simply becomes the new current price — there's no auto-incrementing logic that raises it for you as rival bids come in, even though rival bids themselves are real now (see Assumptions and Scope).
- **Monorepo layout, no root npm workspace** — `client/` and `server/` are entirely separate dependency ecosystems (npm and Go modules), so an npm/pnpm workspace layer would only unify one of the two; each stays a fully self-contained project instead.

## Testing

`cd client && npm test` runs the automated Vitest suite: lifecycle/bidding/grading boundary cases, the `augment()` presentation derivation (including reactivity to the clock), store logic (bid accept/reject paths now driven by a mocked API layer, cursor-paginated fetch/URL-sync, compare's max-2 cap, watchlist highlight timing), realtime message handling against a fake WebSocket, and component behavior for `BidPanel`, `ImageCarousel`, and `InventoryView`'s infinite-scroll sentinel.

`cd server && go test ./...` runs the Go unit suite (domain logic, DTO mapping, middleware, the WS hub against fake connections). `go test -tags=integration ./test/integration/...` additionally runs the real-Redis/Postgres suite via `testcontainers-go` (requires a running Docker daemon) — concurrent-bid correctness (N goroutines racing one listing resolve to a consistent serial ordering with no lost updates), cursor-pagination round-trips across bucket boundaries, and the stream-tailer's drain-to-Postgres behavior.