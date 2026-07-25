# Auto Auction Buyer App

A submission for OPENLANE's **"The Block"** coding challenge — the buyer side of a vehicle auction platform, backed by a real Go + Redis + Postgres backend and seeded from the 200-vehicle dataset in [`data/vehicles.json`](data/vehicles.json).

## How to Run

Requires **Docker Desktop** (WSL2 backend, on Windows). The whole stack — Postgres, Redis, the Go API, the Vite dev server, and Caddy for local HTTPS — runs from `server/`:

```
cd server
./deploy/docker/setup-ssl-windows.sh   # once per machine -- see server/README.md
docker compose up
```

Open `https://localhost`. Caddy serves the client and reverse-proxies the API under the same origin, so there's nothing else to configure — the first request issues a session cookie and every bid/watch/compare action from then on talks to the real backend. `https://localhost/healthcheck` reports Redis/Postgres reachability directly.

`setup-ssl-windows.sh` is a one-time step: it installs [mkcert](https://github.com/FiloSottile/mkcert)'s local CA into the Windows + browser trust stores and writes a browser-trusted cert for `https://localhost` (`mkcert` itself needs installing first if you don't have it — the script prints the `winget` command if it's missing). See [`server/README.md`](server/README.md) for what each step does, `cleanup-ssl-windows.sh`, and running the Go test suites (including the real-Redis/Postgres integration tests) directly without Docker.

The client can also be iterated on in isolation, without the backend running — `npm test`/`npm run lint`/`npm run build` all use a mocked API layer, so they need only `client/`:

```
cd client
npm install
npm test
```

(Requires **Node 24+**, enforced via `engine-strict`.) Running `npm run dev` standalone starts Vite on its own origin (`http://localhost:5173`) with no backend behind it — useful for pure UI iteration, but bidding/watchlist/inventory calls will fail without the Compose stack running and reached through Caddy's `https://localhost` instead.

## Assumptions and Scope

- **24-hour auction duration.** The dataset's `auction_start` has no matching end time, and the source requirements only specify a start. 24 hours is an explicit, unconfirmed assumption — now stored as data (`listings.auction_duration_sec`, default 86400), not hardcoded, so changing it later is an `UPDATE`, not a migration (`guidelines/06-backend-architecture.md`, "Data layer").
- **Bidding, listings, and the audit trail persist for real**; watchlist, compare selection, and filter/sort/search inputs are still deliberately client-only and reset on refresh — those never had a stated need to survive a reload, unlike a placed bid.
- **Rival bidding is real, but nothing bids on its own.** Any two sessions (two browsers, or a normal tab + incognito) are genuinely independent bidders now — the Lua accept path (`guidelines/06-backend-architecture.md`, "Data layer") resolves concurrent bids atomically and correctly. There's still no automated opponent placing bids for you, so seeing `Outbid` in a single session means opening a second one and bidding there.
- **Sessions are anonymous.** An opaque cookie-backed session id, not a real account — no login, no password, no identity that survives switching browsers/devices. This is a stated non-goal, not an oversight (`guidelines/06-backend-architecture.md`, "Non-goals").
- **`reserve_price` is in the dataset but intentionally not shown** — buyers don't see reserve amounts in a real auction either. This is now an actual trust boundary, not just client-side discipline: the server's buyer-facing `ListingSummary` DTO has no such field to serialize in the first place (`guidelines/06-backend-architecture.md`, "API design"), so nothing hitting the API directly can read it either.
- **Watchlist starts empty.** No hand-picked seed data.
- **Money is a whole number everywhere** (the `server/` backend and its Postgres schema included) — every cost in this domain is flat, with no decimal-prone fees or taxes, so there are no cents anywhere: `starting_bid`, `current_bid`, bid increments, all whole integers. If fractional currency is ever needed, the two migration paths are (a) switch the column/application type to a decimal type, or (b) keep integers and scale by 100, treating the last two digits as cents — either way, deliberately not a silent default.

## Stack

- **Frontend:** Vue 3 (Composition API, `<script setup>`) + TypeScript + Vite + Pinia + vue-router. A `services/api/` layer wraps native `fetch` and `WebSocket` — no HTTP client library.
- **Backend:** Go, stdlib `net/http` (1.22+ pattern routing) + `gorilla/websocket`, Redis (`go-redis/v9`, Lua-scripted atomic bid accept, Streams for the realtime/audit fan-out) and Postgres (`pgx/v5`, `golang-migrate`) as a hybrid CQRS-lite system of record.
- **Deployment:** Docker Compose (Postgres, Redis, the API, the Vite dev server) behind Caddy, which terminates TLS and puts the client and API on one origin locally (`https://localhost`).
- **Styling:** hand-authored `<style scoped>` per component + a shared `tokens.css` of CSS custom properties — no inline styles anywhere, no Tailwind
- **Tooling:** ESLint + Prettier (hand-configured), Vitest + `@vue/test-utils` + `@pinia/testing` on the client; `gofmt` + `go vet` + `go test` (unit and `testcontainers-go`-backed Redis/Postgres integration tests) on the server

See [`guidelines/`](guidelines/) for the full engineering-conventions writeup behind these choices on both sides of the repo — locked-in stack decisions, design patterns, guardrails, and testing strategy for the client (`00`–`05`), and the equivalent decision log for the backend (`06-backend-architecture.md`: datastore choice, cursor pagination, sessions/CSRF, the WebSocket protocol, and more) — written before the code they govern, so later work in this repo (by me or by an AI agent) has a stated default instead of re-deciding things per feature.

## What I Built

Every screen and interaction actually wired up in the approved design (`Auto Auction Buyer App.dc.html`, ported here from a Claude Design mock): an Inventory grid with search/filter/sort, a Preview Modal reachable from every card, a full Listing Details page (also reachable by direct URL) with condition/vehicle-data/seller sections and a real bidding form, a Compare flow capped at two listings, and a Watchlist drawer with live quick-bid actions. The mock's one unwired button — "Place Bid" on the details page — is fully implemented here, backed by a tiered bid-increment schedule and a store-level guard that revalidates the listing is still active at submit time, not just when the form rendered.

Since then, the buyer side has a real backend behind it (`server/`, `guidelines/06-backend-architecture.md`): a Redis-backed, Lua-scripted atomic bid/buy-now path so concurrent bids from different sessions resolve correctly with no lost updates; a Postgres system of record and audit trail; cursor-paginated, keyset-based infinite scroll over the inventory grid (with the scroll position resumable from the URL); and a live WebSocket feed so a bid landing on any listing you're looking at — from any session — updates its price and your `Winning`/`Outbid` badge in place, with no refresh. Bid submission is retry-safe: a request that's accepted but whose response never reaches the browser (a dropped connection, a backgrounded tab) can be safely resubmitted and returns the original acceptance rather than a confusing rejection. Everything runs behind HTTPS via Caddy in Docker Compose, with the seed dataset synced into Postgres/Redis on every startup without ever touching a listing that already has real bids against it.

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

## What I'd Do With More Time

- Split up the functionality that's currently grouped into `AugmentedListing`. This type is useful because I can guarantee consistency without redundancy and move fast but it's a single type that is way too extensive and likely hard to maintain.
- Add more data sanitization around values like VIN (used in links)
- True proxy/second-price bidding: a real bidder pool exists now, but a bid still becomes the new price outright rather than an auto-raised maximum competing against other bidders' stored maximums.
- Real auto-bidding (submit a ceiling once, let the server raise your bid for you as rivals bid, up to that ceiling) — Redis's atomic operations are already the actual bidding mechanism today, so the infrastructure this would build on already exists.
- An automated rival-bidder simulation, so `Outbid` is visible without manually opening a second session to bid against yourself.
- Persistence for watchlist and compare selection (bids and listings already persist for real; these two are still deliberately client-only — see Assumptions and Scope).
- Anti-sniping measures through time extension
- WebSocket gap-free reconnect — a reconnect today just re-subscribes to the same listing ids; an event missed while disconnected isn't replayed, and nothing proactively refetches to catch up (only the next unrelated state change that happens to touch that listing, e.g. a filter reset, incidentally corrects it) (`guidelines/06-backend-architecture.md`, "Realtime / WebSocket").
- Real user accounts and auth — today's sessions are anonymous and opaque by design (see Assumptions and Scope); a `Seller` entity and seller-facing endpoints are a related, larger non-goal (`guidelines/06-backend-architecture.md`, "Non-goals").
- Multi-instance deployment — the `Broadcaster` interface is built to support a Redis Pub/Sub swap for horizontal scaling, but only a single `app` instance actually runs today.
- Broader test coverage — end-to-end and visual regression, beyond the current unit/component/integration focus
- "My Auctions" (Active/Upcoming/Won/Lost) for any SOPs that must occur after an auction is over
- Printer-friendly styling for listing details page
