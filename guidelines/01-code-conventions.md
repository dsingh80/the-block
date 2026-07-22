# Code Conventions

Applies to all TypeScript and Vue SFCs under `client/src/`. Goal: any two files in this repo should look like they were written by the same disciplined engineer, not stitched together across sessions.

## Language & tooling

- **TypeScript strict mode.** `"strict": true` in `tsconfig.json`, no exceptions. No `any` — use `unknown` and narrow, or a proper generic.
- **ESLint + Prettier**, enforced via `npm run lint` and treated as CI-grade, not just a local nicety. Formatting is never a matter of taste or a PR comment — Prettier is the single source of truth for it, and `eslint-config-prettier` turns off any ESLint rule that would fight it.
- **Named exports only, with one sanctioned exception.** Vue Single-File Components compile their `<script setup>` block to a default export — that's the framework's own convention, not a violation of this rule. Everything else (types, composables, store definitions, utility functions) uses named exports, since default exports make renames and grep-based navigation unreliable — which matters a lot across sessions and models editing the same repo.
- **`@` path alias over deep relative imports.** `@/stores/bids`, not `../../../stores/bids`.

## Naming

- **Component files**: `PascalCase.vue` (`VehicleCard.vue`, `BidPanel.vue`).
- **Everything else (composables, stores, utils, types)**: `camelCase.ts`, except composables always start with `use` (`useListingPresentation.ts`) and Pinia stores export a `useXStore` (e.g. `stores/bids.ts` exports `useBidsStore`).
- **Types/interfaces**: `PascalCase`. No `I` prefix (`AugmentedListing`, not `IAugmentedListing`) — TypeScript's structural typing makes the prefix noise.
- **Functions/variables**: `camelCase`. Booleans read as predicates: `isUpcoming`, `hasUserBid`, `canBid` — not `upcoming`, `userBidFlag`.
- **Constants that are truly fixed** (thresholds, timing values, the bid-increment schedule's boundaries): `UPPER_SNAKE_CASE`, defined once in `utils/constants.ts`, imported everywhere. No magic numbers inline in logic — a bare `3` or `24` in the middle of a function is unreviewable without a name attached.
- **CSS custom properties**: `--kebab-case`, defined in `assets/styles/tokens.css` (`--color-navy`, `--z-modal`).

## Component and function shape

- Target **under ~25 lines** per function body / a component's `<script setup>` logic block. Not a hard cap, but a smell — past it, ask whether it's doing more than one job.
- **View components are an exception in form, not in spirit.** `InventoryView.vue` and `ListingDetailsView.vue` compose several child components and read from stores/composables; they should look like a short table of contents — each section is a named child component, not inlined markup and inline computed logic duplicating what a composable already provides.
- **Split for reuse, not just line count.** If two components both need "format a price as `$12,345`," that's `utils/format.ts`, not copy-pasted logic.
- **Prefer pure functions wherever there's no reactivity/DOM involved.** Everything in `utils/` is a plain, pure, synchronous function — trivial to unit test, safe to call from anywhere.

## Error handling

- Store actions that can be rejected (`placeBid`, `buyNow`) return a typed result — `{ok: true} | {ok: false, error: string}` — never throw. The calling component reads `.ok` and renders `.error` inline; nothing in this app uses `alert()` or an unhandled promise rejection to communicate a failure.
- An unresolvable read (e.g. `/inventory/:id` for an id that doesn't match any vehicle) renders a typed "not found" UI state — it does not assume the lookup succeeded and crash on `undefined.make`.
- Never swallow an error silently. An empty `catch` block with no fallback UI or rethrow is a bug, not a guardrail.

## Comments & documentation

- Comments explain **why**, not **what**. `// re-check active here, not just in BidPanel — the clock ticks independently of whether the panel is open` is useful; `// place the bid` is not.
- No commented-out code committed to the repo. Delete it; git history is the archive.
- Exported functions in `utils/`, `composables/`, and `stores/` get a one-line comment only when the name and types don't already make the "why" obvious — not a restatement of the signature.

## Formatting specifics (Prettier-enforced, not manually reviewed)

- 2-space indentation, single quotes, no semicolons, `all` trailing commas in multiline literals.
- Max line length 100 characters.
- One exported "main" concept per file. A file named `stores/bids.ts` exports `useBidsStore` and its directly-related types, not an unrelated grab-bag of utilities.

## Commits & PRs

- Small, single-purpose commits. A commit message states intent, not a diff summary (`feat: add auction lifecycle and bidding utils`, not `update files in utils folder`).
- A commit that adds logic (anything in `utils/`, `stores/`, `composables/`, or a component with real behavior) includes that logic's tests in the same commit — not a separate, later "add tests" commit. See `04-testing-strategy.md`.
- Every commit leaves the app in a working, typechecking, lintable, buildable state (`npm run lint`, `npm run build`, `npm test` all clean). No "fix previous commit" follow-ups — that defeats the point of a clean, reviewable history.
