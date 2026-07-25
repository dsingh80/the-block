import { nextTick } from 'vue'

/**
 * Runs a store action that prepends content above the viewport and adjusts
 * scrollTop by exactly the height it added, so whatever the user is looking
 * at doesn't visually jump. Takes a minimal structural type rather than
 * Element so it's trivially unit-testable with a plain object.
 */
export async function prependPreservingScroll(
  scroller: { scrollHeight: number; scrollTop: number },
  mutate: () => Promise<void>,
): Promise<void> {
  const heightBefore = scroller.scrollHeight
  const topBefore = scroller.scrollTop
  await mutate()
  await nextTick()
  scroller.scrollTop = topBefore + (scroller.scrollHeight - heightBefore)
}

function prefersReducedMotion(): boolean {
  return window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
}

/**
 * The initial "fake scroll-down" past the skeleton placeholders to the real,
 * already-loaded results, on a resumed mid-list mount.
 */
export function scrollToResults(target: HTMLElement): void {
  target.scrollIntoView({ block: 'start', behavior: prefersReducedMotion() ? 'auto' : 'smooth' })
}

/**
 * Resolves once `target` fires `scrollend`, or after `timeoutMs` regardless
 * -- the fallback matters because a zero-distance scroll may not fire it at
 * all in every browser, and this must never hang the mount sequence.
 */
export function waitForScrollSettle(target: EventTarget, timeoutMs = 1000): Promise<void> {
  return new Promise((resolve) => {
    let settled = false
    const finish = () => {
      if (settled) return
      settled = true
      target.removeEventListener('scrollend', finish)
      resolve()
    }
    target.addEventListener('scrollend', finish, { once: true })
    setTimeout(finish, timeoutMs)
  })
}
