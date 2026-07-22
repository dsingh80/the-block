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

- `stores/bids.ts` — `placeBid` accepts a valid amount and updates `currentPrice`/`bidCount`/`hasUserBid`/`isUserHighBidder`; rejects an amount below the tiered minimum with a typed error; rejects a bid on a listing that's no longer active (this is the guardrail from `03-guardrails.md` — it needs a real test, not just a doc claiming it exists).
- `stores/compare.ts` — `toggle()` adds up to 2 ids and is a no-op on a 3rd.
- `stores/watchlist.ts` — `toggle()` flips membership and drives the highlight-timing state correctly.

### 3. Composable tests

`composables/useListingPresentation.ts`'s `augment()` is the single highest-value test target in the app — it's where a bug would be both easy to introduce and easy to miss visually. Cover:

- All four `priceLabelText` cases (`Opening Bid` / `Starting Bid` / `Current Bid` / `Winning Bid`), including the real-data case the mock's original 3-case table didn't have to handle (`current_bid == null` on an active listing).
- Every `badgeVariant` (winning/outbid/bidding/won/lost/none).
- `damageList` on both an empty and a non-empty `damage_notes` array.
- `sellerOtherListingsCount`, including the real 0-case (a dealership with exactly one listing).
- Reactivity to the `clock` store — a test that advances `effectiveNow` past a listing's end time and asserts the derived `lifecycle`/badge actually change, not just that the initial computation is correct.

### 4. Component tests

`@vue/test-utils`, targeted at components with actual logic rather than blanket coverage of every presentational component:

- `BidPanel` — the wired bid-submission gap: a too-low amount is rejected with an inline error (no `alert()`), a valid amount is accepted, a bid on a no-longer-active listing is rejected.
- `ImageCarousel` — prev/next wrap-around at both array boundaries, and using the real per-vehicle image count rather than an assumed fixed length.

A component like `GradePill` that only maps a prop to a class doesn't need its own test file — it's exercised indirectly by any component test that mounts it as a child.

## What "done" looks like for a new store/composable/component

Before a commit lands: pure logic has unit tests; a store with reject-able actions has both an accept-path and a reject-path test; anything that touches `augment()`'s output has at least one assertion against it. If a category is genuinely not applicable (e.g. a types-only commit, or a purely presentational component with no branching), that's fine — but say so in the commit/PR description rather than silently omitting it.

## Explicitly out of scope for now

- **End-to-end / browser tests (Playwright, etc.)** — noted as a "what I'd do with more time" item in the README rather than built now. Confirmed scope decision, not an oversight.
- **Visual regression / snapshot testing** — this app's fidelity to the design mock is verified by manual comparison during implementation, not an automated pixel-diff pipeline.
- **Load/concurrency tests** — there's no concurrency to test. This is a single-user client-side app; nothing here shares mutable state across simultaneous requests the way a backend worker pool would.

## CI gate (even before any actual CI config exists)

`npm run lint && npm test && npm run build`, all clean, before every commit in the build sequence — this is what "every commit leaves the app in a working state" (`01-code-conventions.md`) actually means in practice, and what a real CI workflow would run if `05-pr-review-process.md`'s deterministic-checks stage were ever stood up.
