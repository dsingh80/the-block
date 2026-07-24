# Auto Auction Buyer App

A submission for OPENLANE's **"The Block"** coding challenge — the buyer side of a vehicle auction platform, built against the 200-vehicle dataset in [`data/vehicles.json`](data/vehicles.json).

## How to Run

Requires **Node 24+** (enforced via `engine-strict` — `npm install` will refuse to run on an older Node).

```
cd client
npm install
npm run dev
```

Open the URL Vite prints (default `http://localhost:5173`).

Other scripts, run from `client/`: `npm test` (Vitest), `npm run lint` (ESLint), `npm run build` (typecheck + production build), `npm run format` (Prettier).

## Assumptions and Scope

- **24-hour auction duration.** The dataset's `auction_start` has no matching end time, and the source requirements only specify a start. 24 hours is an explicit, unconfirmed assumption.
- **No persistence.** Bids, watchlist, compare selection, and inventory filters all live in memory and reset on refresh.
- **No rival-bid simulation.** Price and bid count only change from your own actions — there's no other buyer to get outbid by. `server/` is a placeholder for a real backend later, which is where simulated competing bids belong instead of a client-side timer hack.
- **`reserve_price` is in the dataset but intentionally not shown** — buyers don't see reserve amounts in a real auction either.
- **Watchlist starts empty.** No hand-picked seed data.
- **Money is a whole number everywhere** (the new `server/` backend and its Postgres schema included) — every cost in this domain is flat, with no decimal-prone fees or taxes, so there are no cents anywhere: `starting_bid`, `current_bid`, bid increments, all whole integers. If fractional currency is ever needed, the two migration paths are (a) switch the column/application type to a decimal type, or (b) keep integers and scale by 100, treating the last two digits as cents — either way, deliberately not a silent default.

## Stack

- **Frontend:** Vue 3 (Composition API, `<script setup>`) + TypeScript + Vite + Pinia + vue-router
- **Backend:** none — `server/` is a placeholder for later; this is a frontend-only build against the static dataset
- **Styling:** hand-authored `<style scoped>` per component + a shared `tokens.css` of CSS custom properties — no inline styles anywhere, no Tailwind
- **Tooling:** ESLint + Prettier (hand-configured), Vitest + `@vue/test-utils` + `@pinia/testing`

See [`guidelines/`](guidelines/) for the full engineering-conventions writeup behind these choices — locked-in stack decisions, design patterns, guardrails, and testing strategy, written before any application code so later work in this repo (by me or by an AI agent) has a stated default instead of re-deciding things per feature.

## What I Built

Every screen and interaction actually wired up in the approved design (`Auto Auction Buyer App.dc.html`, ported here from a Claude Design mock): an Inventory grid with search/filter/sort over all 200 vehicles, a Preview Modal reachable from every card, a full Listing Details page (also reachable by direct URL) with condition/vehicle-data/seller sections and a real bidding form, a Compare flow capped at two listings, and a Watchlist drawer with live quick-bid actions. The mock's one unwired button — "Place Bid" on the details page — is fully implemented here, backed by a tiered bid-increment schedule and a store-level guard that revalidates the listing is still active at submit time, not just when the form rendered.

## Notable Decisions

- **Tiered bid increment** (`< $5,000 → $100`, `< $15,000 → $250`, `≥ $15,000 → $500`) instead of a flat increment — a fixed step is a rounding error on a cheap listing and an odd granularity on an expensive one.
- **No inline styles anywhere.** Every dynamic visual state (grade, title status, badge, CTA, urgent timer, selection) is a finite set of CSS modifier classes bound via `:class`, backed by `tokens.css`. Chosen over Tailwind because introducing a framework would mean translating away from the design mock's own shape for no fidelity gain.
- **`augment()` is the single source of presentation truth** (`composables/useListingPresentation.ts`) — every component reads a fully-derived `AugmentedListing`; nothing re-derives a grade color or badge label inline. It stays reactive to a global clock store, so badges/status genuinely flip live as time passes rather than freezing at whatever moment a component rendered.
- **Global Pinia stores**, not local component state, for bids/watchlist/compare/filters — all of them need to survive navigating away from and back to a view, and the bidding store specifically needs to be visible everywhere at once (a card, the preview modal, the details page, and the watchlist drawer all reflect the same bid instantly).
- **Simplified proxy bidding.** The form is labeled "max proxy bid" to match the design, but there's no real competing bidder pool to run genuine second-price logic against — a valid bid simply becomes the new current price.
- **Monorepo layout, no root npm workspace** — `client/` is a fully self-contained npm project; `server/` is a placeholder folder with nothing to manage, so a workspace layer would add nothing.

## Testing

`cd client && npm test` runs the automated Vitest suite: lifecycle/bidding/grading boundary cases, the `augment()` presentation derivation (including reactivity to the clock), store logic (bid accept/reject paths including the tiered minimum and the still-active-at-submit-time check, compare's max-2 cap, watchlist highlight timing), and component behavior for `BidPanel` and `ImageCarousel`.

## What I'd Do With More Time

- Split up the functionality that's currently grouped into `AugmentedListing`. This type is useful because I can guarantee consistency without redundancy and move fast but it's a single type that is way too extensive and likely hard to maintain.
- Add more data sanitization around values like VIN (used in links)
- A real backend in `server/`, including simulated rival bids so the `Outbid` state shows up without hand-editing data
- Persistence (bids/watchlist/compare survive a refresh)
- True proxy/second-price bidding against a real bidder pool
- Websockets for realtime bidding
- Anti-sniping measures through time extension
- Auto-bidding support with atomic operations (thinking Redis since its scalable and supports atomic operations natively)
- Broader test coverage — end-to-end and visual regression, beyond the current unit/component focus
- "My Auctions" (Active/Upcoming/Won/Lost) for any SOPs that must occur after an auction is over
- Printer-friendly styling for listing details page
