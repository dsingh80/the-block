# Auto Auction Buyer App — Engineering Guidelines: Overview

This is the entry point for a set of guideline docs that scope every future architecture and implementation decision on this project. They exist so that code written across sessions, contributors, and models stays consistent, legible, and free of "AI slop" — code that technically runs but is bloated, inconsistent, over-abstracted, or impossible to review.

This doc set is deliberately smaller than a typical reference set for a backend/agent project: this is a single-user, frontend-only Vue app with no external services, no LLM output to ground, and no protocol surface to design tools against. Docs that would exist for that kind of project (tool design & performance, a domain-FAQ) don't have a real analog here and are intentionally omitted rather than padded out.

Read this file first, then the others as relevant:

- `01-code-conventions.md` — language, style, naming, file layout, error handling.
- `02-design-patterns.md` — the layered architecture and the patterns that keep it decoupled and easy to extend.
- `03-guardrails.md` — this app's actual trust boundaries: input validation and state-mutation correctness.
- `04-testing-strategy.md` — what "tested" means for this project and how to verify it.
- `05-pr-review-process.md` — an automated PR review/fix loop, documented as a future option. **Not built yet** — read this when actually setting one up, not for day-to-day development.

## Locked-in stack decisions

These were decided explicitly, largely during the planning conversation for the initial build, to stop re-litigating them per-feature. Treat them as defaults; deviating requires a written tradeoff note (see Design Principles below), not a silent choice.

| Decision | Choice | Why |
|---|---|---|
| Framework | Vue 3, Composition API, `<script setup lang="ts">` | Specified in the project's own design-handoff doc (`openlane-design-handoff.md`, from the Claude Design project this app was built from). |
| Build tool | Vite | Vue-official tooling, fast dev server, native ESM, shares config with the test runner. |
| State management | Pinia, setup-store style | Official Vue state library. The handoff doc specifically calls for a "global, always-subscribed" bidding store that survives navigation between views — Pinia is what makes that a store-layer concern instead of something bolted onto routing. |
| Routing | vue-router | Listing details must be reachable by a direct URL (`/inventory/:id`), not just an in-app modal-to-page transition — that requires real routes. |
| Styling | Hand-authored `<style scoped>` per component + a shared `tokens.css` of CSS custom properties. **No inline styles, no `:style` bindings anywhere** — every dynamic visual state (grade, title status, badge, CTA, urgent timer, selection state) is a finite set of CSS modifier classes toggled via `:class`. | Vite/Vue's SFC compiler extracts `<style scoped>` blocks into cacheable, tree-shakeable CSS at build time — real, separate CSS output, not inline attributes. |
| CSS framework | None (not Tailwind) | **Deviation on record**: the handoff doc mentions Tailwind. The concrete design mock this app was built from (`Auto Auction Buyer App.dc.html`) uses zero utility classes — it's a later, more concrete, more authoritative artifact than the handoff doc, so styling follows what the mock actually is rather than what an earlier planning doc mentioned. Introducing Tailwind would mean translating away from the mock's own shape for no fidelity gain. |
| Test runner | Vitest + `@vue/test-utils` + `@pinia/testing` | Vite-native, near-zero config, covers pure functions, components, and stores from one tool. |
| Linting/formatting | ESLint (flat config, `typescript-eslint` + `eslint-plugin-vue` recommended rule sets) + Prettier (`eslint-config-prettier` to defer formatting to Prettier), hand-configured | Enforced as a real gate (`npm run lint`), not a suggestion. Hand-configured rather than via `create-vue`'s interactive scaffolding prompts, so the setup is deterministic regardless of that tool's current CLI flags. |
| Repo layout | Monorepo: `client/` (this app, a fully self-contained npm project) + `server/` (placeholder only, reserved for a future real backend) + `guidelines/` at the repo root, governing both. | Matches the handoff doc's intended shape. |
| Root npm workspace | **None.** `client/` has its own `package.json`/`node_modules`; there's no root `package.json`. | **Deviation on record**: an earlier draft of the implementation plan added a root `package.json` with an npm `workspaces` field pointing at `client/`, purely for `npm run dev -w client`-style convenience commands. Rejected in review: `server/` isn't an npm package (no `package.json`, just a README stub), so there was nothing else workspace-managed to justify the extra layer. `cd client && npm install && npm run dev` is simpler and exactly as capable for a one-member workspace. |
| Node version | 24+, enforced via `"engines"` in `client/package.json` **and** `engine-strict=true` in `client/.npmrc` | An `engines` field alone only warns; `engine-strict` makes `npm install` actually fail below the floor, which is what "pinned" means. |
| Bid increment | Tiered by current price: `< $5,000 → $100`, `< $15,000 → $250`, `>= $15,000 → $500` (`utils/bidding.ts`, `getBidIncrement`) | A deliberate design decision (not a default, not an assumption) — chosen over a flat increment because a fixed $250 step is a rounding error on a $2,000 beater and an odd granularity on an $80,000 truck. |
| Auction lifecycle clock | `upcoming`/`active`/`ended` is derived from real `Date.now()`. The dataset (`data/vehicles.json`) is never modified or regenerated, and the app never derives a synthetic/normalized "now" from it. | **Deviation on record**: an earlier draft of the implementation plan computed a synthetic reference time from the dataset's own timestamp range, specifically to guarantee a live mix of upcoming/active/ended listings during a demo. Rejected in review in favor of real time plus an accepted, documented consequence — see the app's own root `README.md` → Assumptions and Scope for what that consequence actually is (as of the dataset committed at build time, every listing reads as `ended`) and the no-code-changes workaround for seeing the live states locally. |
| Auction duration | 24 hours from `auction_start` | Unconfirmed assumption, carried over from the handoff doc — the source requirements only specify a start time, not an end time. Documented in the README, not silently baked in. |

## Design principles (apply everywhere)

1. **Single responsibility, always.** One component, one store, one composable, one function does one job. If a component's `<script setup>` block is doing two unrelated things, split it.
2. **Decouple through props/emits and store interfaces, not conditionals.** Adding a new badge state or CTA variant should mean extending the `augment()` derivation's variant table (`composables/useListingPresentation.ts`), not adding an `if` branch inside a component template.
3. **Small functions and components, but not fragmented for its own sake.** Split when it improves reuse (the same logic is needed in two places) or legibility (a view composing several children should read like a table of contents, not have the actual logic inlined into it). Don't extract a component that just wraps a single prop-to-class mapping into its own file if it's only ever used once.
4. **Legibility over cleverness.** Prefer boring, explicit code over a clever one-liner. A reviewer unfamiliar with the file should understand intent within a minute.
5. **No inline styles, no raw hex/px scattered through templates.** Every dynamic visual state traces back to a named token in `tokens.css` and a modifier class defined in a component's own `<style scoped>` block — this project's analog of "no ungrounded output": a visual state that isn't backed by a named class/token is exactly as suspect as a claim with no citation.
6. **Validate at the boundary, trust internally.** The real boundary for bid submission is the `bids` store action (`placeBid`/`buyNow`), not the DOM `submit` handler — the store re-derives the minimum bid and re-checks the listing is still active, because a component-level check alone can't account for the clock ticking independently of whether a panel happens to be open. Once a value has passed through the store, downstream code trusts it.
7. **Fail loud in dev, fail safe in the UI.** Store actions that can be rejected return a typed `{ok:true} | {ok:false, error: string}` result — never throw past the store boundary into an unhandled promise rejection or a blank screen. Components render the error inline; nothing uses `alert()`.

## Non-goals (for now)

- A real backend, authentication, or WebSockets — `server/` is a placeholder. Rival-bid simulation is explicitly deferred there, not faked client-side with a timer.
- Persistence (`localStorage`, etc.) for bids/watchlist/compare/filters — everything resets on refresh, by design, documented in the README.
- The "My Auctions" page from the handoff doc — the concrete design mock ships it disabled/unbuilt, and this app removes it entirely rather than shipping a dead nav link.
- Pagination or virtualization on the inventory grid — 200 client-side records renders fine without either; don't add either speculatively.
- Tailwind or any other CSS framework — see the styling decision above.
- A built, running automated PR-review CI loop — see `05-pr-review-process.md`, which documents the *design* as an option without standing up the actual GitHub Actions workflows.

Keep this list short and revisit it as the project matures — premature infrastructure is its own form of AI slop.
