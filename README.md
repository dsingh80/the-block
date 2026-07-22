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
- **Lifecycle is derived from real time, and the dataset is never modified or regenerated.** As committed, every `auction_start` in `data/vehicles.json` is now in the past (by months), so **every listing currently shows as `Ended`** — you'll see `Won`/`Lost` badges and the "Auction Ended" bid panel state throughout, not `Winning`/`Outbid`/`Bidding` or an active bid form. This was a deliberate choice over normalizing time to the dataset, made explicitly aware of this consequence. To see the live `upcoming`/`active` states and the bid form locally: temporarily edit one `auction_start` in `client/src/data/vehicles.json` to a near-future timestamp (don't commit the change), or set your OS clock back into early April 2026.
- **No persistence.** Bids, watchlist, compare selection, and inventory filters all live in memory and reset on refresh.
- **No rival-bid simulation.** Price and bid count only change from your own actions — there's no other buyer to get outbid by. `server/` is a placeholder for a real backend later, which is where simulated competing bids belong instead of a client-side timer hack.
- **`reserve_price` is in the dataset but intentionally not shown** — buyers don't see reserve amounts in a real auction either.
- **"N other listings" on the seller card is a name-text search against Inventory**, not a true seller-id filter (there's no seller-id concept in this dataset).
- **Watchlist starts empty.** No hand-picked seed data.

## Stack

- **Frontend:** Vue 3 (Composition API, `<script setup>`) + TypeScript + Vite + Pinia + vue-router
- **Backend:** none — `server/` is a placeholder for later; this is a frontend-only build against the static dataset
- **Styling:** hand-authored `<style scoped>` per component + a shared `tokens.css` of CSS custom properties — no inline styles anywhere, no Tailwind
- **Tooling:** ESLint + Prettier (hand-configured), Vitest + `@vue/test-utils` + `@pinia/testing`

See [`guidelines/`](guidelines/) for the full engineering-conventions writeup behind these choices — locked-in stack decisions, design patterns, guardrails, and testing strategy, written before any application code so later work in this repo (by me or by an AI agent) has a stated default instead of re-deciding things per feature.

## What I Built

Every screen and interaction actually wired up in the approved design (`Auto Auction Buyer App.dc.html`, ported here from a Claude Design mock): an Inventory grid with search/filter/sort over all 200 vehicles, a Preview Modal reachable from every card, a full Listing Details page (also reachable by direct URL) with condition/vehicle-data/seller sections and a real bidding form, a Compare flow capped at two listings, and a Watchlist drawer with live quick-bid actions. The mock's one unwired button — "Place Bid" on the details page — is fully implemented here, backed by a tiered bid-increment schedule and a store-level guard that revalidates the listing is still active at submit time, not just when the form rendered.

## Notable Decisions

- **"My Auctions" removed entirely**, not shipped disabled — it's in the design mock's nav as a greyed-out, unbuilt link; this build drops it and every reference to it rather than shipping a dead link.
- **Tiered bid increment** (`< $5,000 → $100`, `< $15,000 → $250`, `≥ $15,000 → $500`) instead of a flat increment — a fixed step is a rounding error on a cheap listing and an odd granularity on an expensive one.
- **No inline styles anywhere.** Every dynamic visual state (grade, title status, badge, CTA, urgent timer, selection) is a finite set of CSS modifier classes bound via `:class`, backed by `tokens.css`. Chosen over Tailwind specifically because the approved mock itself uses zero utility classes — introducing a framework would mean translating away from the mock's own shape for no fidelity gain.
- **`augment()` is the single source of presentation truth** (`composables/useListingPresentation.ts`) — every component reads a fully-derived `AugmentedListing`; nothing re-derives a grade color or badge label inline. It stays reactive to a global clock store, so badges/status genuinely flip live as time passes rather than freezing at whatever moment a component rendered.
- **Global Pinia stores**, not local component state, for bids/watchlist/compare/filters — all of them need to survive navigating away from and back to a view, and the bidding store specifically needs to be visible everywhere at once (a card, the preview modal, the details page, and the watchlist drawer all reflect the same bid instantly).
- **Simplified proxy bidding.** The form is labeled "max proxy bid" to match the design, but there's no real competing bidder pool to run genuine second-price logic against — a valid bid simply becomes the new current price.
- **Accessibility pass beyond the literal mock**: two of the mock's literal text colors (green/amber pill text) sit just under WCAG AA contrast on their own tinted backgrounds and were darkened one step; modals use real `role="dialog"`/`aria-modal`/focus-on-open/Escape-to-close, and everything except the currently-open modal goes `inert` while one is open.
- **Monorepo layout, no root npm workspace** — `client/` is a fully self-contained npm project; `server/` is a placeholder folder with nothing to manage, so a workspace layer would add nothing.

## Testing

`cd client && npm test` runs the automated Vitest suite: lifecycle/bidding/grading boundary cases, the `augment()` presentation derivation (including reactivity to the clock), store logic (bid accept/reject paths including the tiered minimum and the still-active-at-submit-time check, compare's max-2 cap, watchlist highlight timing), and component behavior for `BidPanel` and `ImageCarousel`.

Manually verified in a running dev server: search/filter/sort combinations and the empty-results state; card → Preview Modal → Listing Details navigation; the condition-report expand/collapse; watchlist add/remove and the drawer's sections; compare's 2-item cap, bar, and modal; direct-URL load of a listing (including an unknown id); refresh mid-session resetting state as documented; and no horizontal overflow at a mobile viewport width, including the details page's bid panel reordering above the rest of the content and the watchlist drawer staying within the viewport. The bid-placement flow itself (accept/reject/tiered minimums) is verified via the automated store and component tests above rather than a live demo, since the committed dataset has no currently-active auction to bid on without the local edit described in Assumptions.

## What I'd Do With More Time

- A real backend in `server/`, including simulated rival bids so the `Outbid` state shows up without hand-editing data
- Persistence (bids/watchlist/compare survive a refresh)
- True proxy/second-price bidding against a real bidder pool
- Broader test coverage — end-to-end and visual regression, beyond the current unit/component focus
- A deep-linkable Preview Modal and URL-reflected filters
- "My Auctions" (Active/Upcoming/Won/Lost), which the original design handoff scoped out for this pass
