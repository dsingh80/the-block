# Design Patterns & Architecture

The app has four layers. Each layer only talks to the layer directly below it, never reaching through to internals. This is what keeps "add a new derived visual state" or "add a new cross-view concern" a one-file change instead of a template-by-template hunt.

```
┌─────────────────────────────────────────────┐
│ View Layer                                   │  InventoryView, ListingDetailsView — route
│                                               │  targets, compose components, minimal logic
├─────────────────────────────────────────────┤
│ Component Layer                              │  components/** — presentational components,
│                                               │  some store-bound, none re-deriving state
├─────────────────────────────────────────────┤
│ Composable / Derivation Layer                │  useListingPresentation (augment()),
│                                               │  useWatchlistSections
├─────────────────────────────────────────────┤
│ Store Layer                                  │  Pinia stores — the single source of truth
│                                               │  per concern (clock, bids, watchlist, compare,
│                                               │  preview, inventory filters)
└─────────────────────────────────────────────┘
   utils/ (pure functions) and types/ underpin every layer above.
```

## 1. Views stay thin

`InventoryView.vue` and `ListingDetailsView.vue` are route targets: they read from stores/composables and compose child components. Neither should contain business logic — if a view needs an `if` deciding *how* to compute a value rather than *which component to show*, that logic belongs in a composable or a store, one layer down.

## 2. `augment()` is the single source of presentation truth

Every visual thing a listing can show — its badge, price label, CTA button style, time-remaining color, whether it can be bid on — is derived exactly once, in `composables/useListingPresentation.ts`'s `augment(vehicle, ctx)`. Components consume the resulting `AugmentedListing` object's fields directly; they never re-derive a grade color from `condition_grade`, never re-check `lifecycle` against the clock themselves, never build their own badge text. This is the project's version of a "grounding" rule: if two components render the same listing and disagree about its badge, that's a bug in `augment()`, not something to patch around locally in whichever component is wrong.

`augment()` must stay reactive: it's wrapped in `computed()` by `useAugmentedListings()`/`useAugmentedListing()`, not called once into a plain variable. It reads the `clock` store's `effectiveNow`, which is what makes a badge silently flip from *Winning* to *Won* as time passes — that reactivity is load-bearing, not incidental.

## 3. Global Pinia stores for anything cross-cutting

Bids, watchlist, and compare selection all live in global Pinia stores, not component-local `ref`s or props threaded down from a view. This directly generalizes a requirement from the project's design-handoff doc: the bidding store must be "global, always-subscribed... not scoped to whichever view/tab happens to be mounted," because a user winning, losing, or getting outbid on a listing has to be reflected even while they're looking at an unrelated page (e.g. the watchlist drawer, which floats above every view). The same reasoning applies to watchlist and compare — both need to survive navigating from the inventory grid to a listing's detail page and back.

`inventoryFilters` is also a store rather than local view state, for a narrower but concrete reason: Vue Router unmounts `InventoryView` when navigating to `/inventory/:id`, so plain local `ref`s for search/filter/sort would silently reset on every round trip through a listing's detail page. A store survives that unmount.

## 4. `services/api/` is the one adapter layer

`server/` is a real backend now (`guidelines/06-backend-architecture.md`), reached through a thin `services/api/` layer — components and stores call a typed service function (`fetchListings`, `placeBid`, ...), never `fetch` directly, so a wire-format or endpoint change is a one-file change in `services/api/`, not a grep across every store. `stores/inventoryFilters.ts` is the store that actually calls it for reads (`reset`/`loadNextPage`/`ensureVehicleLoaded`) and doubles as the app's vehicle cache (`vehiclesById`, accumulated across every fetch, never cleared on a filter change) so `useAugmentedListings()`/`useAugmentedListing()` — and by extension watchlist/compare, which need to render a listing regardless of whether it's in the *current* filtered page — don't need their own fetch logic. `stores/bids.ts` calls it for writes (`placeBid`/`buyNow`).

## 5. `main.ts` is the composition root

`main.ts` is the one place `createApp`, Pinia, and the router are constructed and mounted. No component or store constructs its own router instance or a second Pinia instance. This is what makes the app trivially testable (inject a fresh test Pinia per test via `@pinia/testing`) and keeps "how is this wired together" answerable by reading one small file.

## 6. Variant-class styling, not inline styles

Every component with a dynamic visual state (grade, title status, badge, CTA, urgent timer, selection state) exposes that state as a small closed set of string variants on its `AugmentedListing` (e.g. `gradeVariant: 'good' | 'fair' | 'poor'`), and the component binds `:class` against those variants — never `:style`, never a raw hex value in a template. The actual colors live in exactly one place, `assets/styles/tokens.css`, as CSS custom properties. This is as much an architectural decision as a styling one: it's what keeps "the green used for a winning badge" and "the green used for a clean title" traceable to the same token instead of two hand-typed hex strings that can silently drift apart.

## When to deviate

Any deviation from these patterns (a new architectural layer, a store that isn't global when it probably should be, a component that reaches past its own layer) gets a short written tradeoff note before implementation — what the current pattern doesn't solve, what's being proposed instead, what it costs. Two examples already on record are in `00-overview.md`'s locked-in stack table (the root-npm-workspace deviation and the real-time-clock deviation) — that's the format to match, not a new one to invent per note.
