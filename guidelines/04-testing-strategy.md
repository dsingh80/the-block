# Testing Strategy

Built off the locked-in stack: TypeScript, Vue 3, Pinia, Vitest, `@vue/test-utils`, `@pinia/testing`. Vitest-only for now — no Playwright/e2e, no visual regression (see "Explicitly out of scope" below). The goal is confidence proportional to risk: heavy on the logic that's easy to get subtly wrong (the presentation derivation, bid/lifecycle boundary math), light on thin wrappers that just map a prop to a class.

## Layers

### 1. Unit tests (pure functions — cheapest, highest-value)

- `utils/lifecycle.ts`'s `deriveLifecycle` — the `upcoming`/`active`/`ended` boundaries, including the exact instant `auction_start` and `auction_start + 24h` fall on.
- `utils/bidding.ts`'s `getBidIncrement` — all three tiers, and the exact boundary values ($4,999 vs $5,000, $14,999 vs $15,000).
- `utils/grading.ts`'s bucket edges — grade exactly at 3.0 and exactly at 4.0, where the mock's `<` comparisons decide which side of the boundary a value falls on.
- `utils/format.ts`, `utils/time.ts` — currency/mileage/duration formatting.

No mocking needed for any of these — they're plain, synchronous, pure functions.

### 2. Store tests

Using `@pinia/testing`'s `createTestingPinia()` per test (a fresh store instance, not hand-rolled reset logic between tests):

- `stores/bids.ts` — `placeBid`/`buyNow` are async now, calling a mocked `services/api/listings` (`vi.mock('@/services/api/listings')`) rather than mutating state directly: a valid amount resolves and populates `overrides[id]` from the mocked response's `viewer` object (not from what the client computed), a too-low amount or an inactive listing is rejected locally *without the mock ever being called* (the client-side pre-check from `03-guardrails.md`), and a mocked API rejection surfaces its message through the same typed `{ok:false, error}` shape as a local rejection.
- `stores/compare.ts` — `toggle()` adds up to 2 ids and is a no-op on a 3rd.
- `stores/watchlist.ts` — `toggle()` flips membership and drives the highlight-timing state correctly.

### 3. Composable tests

`composables/useListingPresentation.ts`'s `augment()` is the single highest-value test target in the app — it's where a bug would be both easy to introduce and easy to miss visually. Cover:

- All four `priceLabelText` cases (`Opening Bid` / `Starting Bid` / `Current Bid` / `Winning Bid`), including the real-data case the mock's original 3-case table didn't have to handle (`current_bid == null` on an active listing).
- Every `badgeVariant` (winning/outbid/bidding/won/lost/none).
- `damageList` on both an empty and a non-empty `damage_notes` array.
- Reactivity to the `clock` store — a test that advances `effectiveNow` past a listing's end time and asserts the derived `lifecycle`/badge actually change, not just that the initial computation is correct.

### 4. Component tests

`@vue/test-utils`, targeted at components with actual logic rather than blanket coverage of every presentational component:

- `BidPanel` — the wired bid-submission gap: a too-low amount is rejected with an inline error (no `alert()`), a valid amount is accepted, a bid on a no-longer-active listing is rejected.
- `ImageCarousel` — prev/next wrap-around at both array boundaries, and using the real per-vehicle image count rather than an assumed fixed length.

A component like `GradePill` that only maps a prop to a class doesn't need its own test file — it's exercised indirectly by any component test that mounts it as a child.

### 5. Test-double patterns for the network/router/realtime surfaces

Established once `server/` became real and the client started talking to it (`02-design-patterns.md` #4) — the defaults for anything new that needs one of these:

- **The API layer**: `vi.mock('@/services/api/listings')` at the top of the file, then `vi.mocked(listingsApi.fetchListings).mockResolvedValue(...)`/`mockRejectedValue(...)` per test. Every store/component test that exercises a code path calling `services/api/` uses this — never a real `fetch`.
- **A router-dependent view** (URL-seeded filters, `router.replace` on a filter change): `createRouter({history: createMemoryHistory(), routes: [...]})`, `await router.push(url)`, `await router.isReady()`, then `mount(View, {global: {plugins: [router]}})`. First established for `InventoryView.test.ts`; no prior pattern existed for a router-dependent component before that.
- **`IntersectionObserver`** (the infinite-scroll sentinel): jsdom has no native implementation at all. A small `FakeIntersectionObserver` class capturing its constructor callback and exposing a manual `.intersect()` method, installed via `vi.stubGlobal('IntersectionObserver', FakeIntersectionObserver)`.
- **The WebSocket connection**: two different levels depending on what's under test. `services/api/ws.ts`'s own `RealtimeConnection` is tested against a `FakeWebSocket` class (`vi.stubGlobal('WebSocket', FakeWebSocket)`) that can `.open()`/`.close()`/`.message(payload)` on demand — this is what proved the ref-counted subscribe/unsubscribe behavior (two callers wanting the same listing id, one letting go must not kill the other's subscription) and the reconnect-with-backoff timing (`vi.useFakeTimers()`). Anything that just *consumes* the connection (`composables/useRealtimeSync.ts`) instead mocks the whole module (`vi.mock('@/services/api/ws', () => ({realtimeConnection: {subscribe: vi.fn(), ...}}))`) and drives `useRealtimeSubscription` inside a plain Vue `effectScope()` (no component needed) to assert the subscribe/unsubscribe calls a changing id list produces, including on `scope.stop()`.

## What "done" looks like for a new store/composable/component

Before a commit lands: pure logic has unit tests; a store with reject-able actions has both an accept-path and a reject-path test; anything that touches `augment()`'s output has at least one assertion against it. If a category is genuinely not applicable (e.g. a types-only commit, or a purely presentational component with no branching), that's fine — but say so in the commit/PR description rather than silently omitting it.

## Explicitly out of scope for now

- **End-to-end / browser tests (Playwright, etc.)** — noted as a "what I'd do with more time" item in the README rather than built now. Confirmed scope decision, not an oversight.
- **Visual regression / snapshot testing** — this app's fidelity to the design mock is verified by manual comparison during implementation, not an automated pixel-diff pipeline.
- **Load/concurrency tests, client-side** — nothing in `client/` shares mutable state across simultaneous requests the way a backend worker pool would, so there's no client-side equivalent to test. This is scoped to the client specifically: the server *does* have real concurrency now (multiple sessions racing a bid on one listing), and that's very much tested — see `06-backend-architecture.md`'s own test suite, particularly the dedicated `PlaceBid` concurrency-correctness test (N goroutines racing bids, asserting the outcome matches some serial ordering with no lost updates).

## CI gate (even before any actual CI config exists)

`npm run lint && npm test && npm run build`, all clean, before every commit in the build sequence — this is what "every commit leaves the app in a working state" (`01-code-conventions.md`) actually means in practice, and what a real CI workflow would run if `05-pr-review-process.md`'s deterministic-checks stage were ever stood up.
