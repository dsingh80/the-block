# Guardrails

This app has no LLM output to ground and no external attack surface (no auth, no user-supplied content beyond client-side-only search text), so this doc is reframed from what a backend/agent project would need: it covers this app's actual trust boundaries — input validation and state-mutation correctness — treated as non-negotiable gates, not best-effort checks.

## Input validation (the front gate)

- **Bid amount.** `BidPanel`'s submit handler strips non-numeric characters (tolerating a user typing `$` or commas) and passes the result to `bidsStore.placeBid(id, amount)`. The **store action**, not the component, is the real gate: it re-derives the minimum acceptable amount via `getBidIncrement(priceValue)` and rejects anything below `priceValue + increment`. Component-level validation is a UX nicety (instant feedback without a round trip) — it is not the thing that actually protects state correctness, because nothing stops a future caller from invoking the store action directly.
- **Listing still active at submit time.** `placeBid` also re-checks `deriveLifecycle(vehicle.auction_start, clock.effectiveNow) === 'active'` before accepting a bid — not just at the moment the panel rendered. The clock ticks on its own 30-second interval independent of whether a `BidPanel` happens to be mounted, so a listing can legitimately flip `active → ended` while a user is mid-interaction. Reject, don't silently accept, a bid on a listing that's no longer active.
- **Route params.** `/inventory/:id` for an id that doesn't resolve to any vehicle in the dataset renders a typed "Vehicle not found" state. It never assumes the lookup succeeded and lets a component read a property off `undefined`.
- **Selection cardinality.** Compare's max-2 cap is enforced in `stores/compare.ts`'s `toggle()` (a no-op past 2 selections), not just by disabling a checkbox in `VehicleCard`. A UI-only cap can be bypassed by any other future caller of the store; a store-level cap can't.

Validate once, at the point closest to where invalid data would actually cause harm (a store mutation), and trust that value everywhere downstream from there.

## State-mutation correctness (this app's analog of output grounding)

The thing most likely to be subtly wrong in this app isn't a security hole — it's a visual state that's inconsistent with the data that's supposed to back it. Two rules exist specifically to prevent that:

- **Every derived visual field comes from `augment()`.** A component must never compute a grade color, a badge label, or a "time remaining" string inline from raw vehicle/override data — always through `useListingPresentation`. This is enforced by code review convention, not a runtime check (same caveat the reference guideline set this project was modeled on makes about its own analogous convention-enforced rule) — when reviewing a PR, check specifically whether a new piece of UI reads from `AugmentedListing` or reaches around it.
- **Lifecycle-dependent reads go through the reactive clock store.** Any code that needs "is this active/upcoming/ended right now" reads `clock.effectiveNow` (reactive) via `augment()`/`deriveLifecycle` — never a direct `Date.now()`/`new Date()` call inline in a component. A direct call freezes at whatever the component's last render was and silently stops updating as time passes; going through the store is what makes badges and countdowns actually live.

## Fail-safe guardrails (lighter weight)

- **Image loading.** Every vehicle image gets `loading="lazy"` (200 cards' worth of `placehold.co` requests shouldn't all fire on first paint) and an `@error` handler that swaps to a plain placeholder box — a flaky image host response shouldn't leave a broken-image icon on screen during a walkthrough.
- **Modal focus management tolerates mount/unmount races.** `PreviewModal`/`CompareModal`'s focus-on-open and `inert`-on-background logic should no-op safely if a ref is momentarily null, rather than throwing during a fast open/close sequence.
- **No secrets to worry about at v1** (no API keys, no auth) — noted here so the habit is established before `server/` becomes real and that changes.

## Where this fits the review process

Guardrail code (the store-level bid/lifecycle re-checks, the compare cap) gets the same review scrutiny as any other logic — specifically ask "does this actually block the bad case" (e.g. does `placeBid` really re-check `active`, or only the amount?), not just "does a check exist." A guardrail that's trivially satisfiable by the obvious happy-path input is a silent failure waiting to happen.
