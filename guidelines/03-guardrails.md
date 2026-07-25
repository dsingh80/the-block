# Guardrails

A real backend now exists (`server/`, `06-backend-architecture.md`) — that's where this app's actual trust-boundary enforcement lives now (session/CSRF, rate limiting, and the Lua accept path's own validation, which is the true, final gate on every bid). This doc stays scoped to what's still specific to the client: client-side checks here are explicitly a UX nicety for instant feedback, never the thing actually protecting correctness — and the state-mutation/presentation correctness below, which has no server-side equivalent at all since it's about what the client *shows*, not what it's allowed to *do*.

## Input validation (a UX nicety, not the real gate)

- **Bid amount.** `BidPanel`'s submit handler strips non-numeric characters (tolerating a user typing `$` or commas) and passes the result to `bidsStore.placeBid(id, amount)`. The store action re-derives the minimum acceptable amount via `getBidIncrement(priceValue)` and rejects anything below `priceValue + increment` locally, before ever calling the API — so a doomed request never reaches the network. But the **server's Lua accept path is the real, final gate** (`06-backend-architecture.md`, "Idempotency"): it re-validates everything from scratch against the live price, and its response is what `overrides[id]` actually gets populated from, not what the client computed. Client-side validation here is purely "don't make an obviously-doomed round trip," not the thing keeping state correct.
- **Listing still active at submit time.** `placeBid` also re-checks `deriveLifecycle(vehicle.auction_start, clock.effectiveNow) === 'active'` before even calling the API — not just at the moment the panel rendered, since the clock ticks on its own 30-second interval independent of whether a `BidPanel` happens to be mounted. Same caveat as above: this only saves a doomed request; the server's own `auction_not_started`/`auction_ended` checks are what actually reject a bid on a listing that's no longer active, regardless of what the client believes.
- **Route params.** `/inventory/:id` for an id the API doesn't have (a 404 from `ensureVehicleLoaded`, not a static-dataset miss anymore) renders a typed "Vehicle not found" state. It never assumes the lookup succeeded and lets a component read a property off `undefined`.
- **Selection cardinality.** Compare's max-2 cap is enforced in `stores/compare.ts`'s `toggle()` (a no-op past 2 selections), not just by disabling a checkbox in `VehicleCard`. A UI-only cap can be bypassed by any other future caller of the store. This one genuinely has no server-side equivalent — compare selection is still client-only state (see the root `README.md`'s Assumptions and Scope), so the store-level cap is the only gate that exists at all, not just the closest one.

Validate once client-side, at the point closest to where an obviously-doomed request would otherwise waste a round trip (a store action, before it calls `services/api/`) — and let the server's own response, not the client's own check, be what state downstream actually trusts.

## State-mutation correctness (this app's analog of output grounding)

The thing most likely to be subtly wrong in this app isn't a security hole — it's a visual state that's inconsistent with the data that's supposed to back it. Two rules exist specifically to prevent that:

- **Every derived visual field comes from `augment()`.** A component must never compute a grade color, a badge label, or a "time remaining" string inline from raw vehicle/override data — always through `useListingPresentation`. This is enforced by code review convention, not a runtime check (same caveat the reference guideline set this project was modeled on makes about its own analogous convention-enforced rule) — when reviewing a PR, check specifically whether a new piece of UI reads from `AugmentedListing` or reaches around it.
- **Lifecycle-dependent reads go through the reactive clock store.** Any code that needs "is this active/upcoming/ended right now" reads `clock.effectiveNow` (reactive) via `augment()`/`deriveLifecycle` — never a direct `Date.now()`/`new Date()` call inline in a component. A direct call freezes at whatever the component's last render was and silently stops updating as time passes; going through the store is what makes badges and countdowns actually live.

## Fail-safe guardrails (lighter weight)

- **Image loading.** Every vehicle image gets `loading="lazy"` (200 cards' worth of `placehold.co` requests shouldn't all fire on first paint) and an `@error` handler that swaps to a plain placeholder box — a flaky image host response shouldn't leave a broken-image icon on screen during a walkthrough.
- **Modal focus management tolerates mount/unmount races.** `PreviewModal`/`CompareModal`'s focus-on-open and `inert`-on-background logic should no-op safely if a ref is momentarily null, rather than throwing during a fast open/close sequence.
- **Still no client-side secrets to worry about.** `server/` is real now, but sessions stay anonymous (an opaque cookie, not a password or API key — `06-backend-architecture.md`, "Sessions & security"), and the client never handles a credential of any kind. The one sensitive artifact is the session cookie itself, and it's `HttpOnly` specifically so client-side code (this app's own JS included) can't read it even if it wanted to.

## Where this fits the review process

Guardrail code (the store-level bid/lifecycle re-checks, the compare cap) gets the same review scrutiny as any other logic — specifically ask "does this actually block the bad case" (e.g. does `placeBid` really re-check `active`, or only the amount?), not just "does a check exist." A guardrail that's trivially satisfiable by the obvious happy-path input is a silent failure waiting to happen.
