import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { prependPreservingScroll, scrollToResults, waitForScrollSettle } from './useScrollAnchor'

describe('prependPreservingScroll', () => {
  it('adjusts scrollTop by the scrollHeight added once mutate settles', async () => {
    const scroller = { scrollHeight: 1000, scrollTop: 200 }

    await prependPreservingScroll(scroller, async () => {
      scroller.scrollHeight = 1500
    })

    expect(scroller.scrollTop).toBe(700)
  })

  it('leaves scrollTop unchanged when mutate does not change scrollHeight', async () => {
    const scroller = { scrollHeight: 1000, scrollTop: 200 }

    await prependPreservingScroll(scroller, async () => {})

    expect(scroller.scrollTop).toBe(200)
  })

  it('propagates a rejection from mutate without adjusting scrollTop', async () => {
    const scroller = { scrollHeight: 1000, scrollTop: 200 }

    await expect(
      prependPreservingScroll(scroller, async () => {
        throw new Error('fetch failed')
      }),
    ).rejects.toThrow('fetch failed')
    expect(scroller.scrollTop).toBe(200)
  })
})

describe('scrollToResults', () => {
  let scrollIntoView: ReturnType<typeof vi.fn>

  beforeEach(() => {
    scrollIntoView = vi.fn()
    Element.prototype.scrollIntoView = scrollIntoView as unknown as typeof Element.prototype.scrollIntoView
  })

  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('scrolls the target into view smoothly, aligned to the top, by default', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: false }))

    scrollToResults(document.createElement('div'))

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'smooth' })
  })

  it('scrolls instantly when the user prefers reduced motion', () => {
    vi.stubGlobal('matchMedia', vi.fn().mockReturnValue({ matches: true }))

    scrollToResults(document.createElement('div'))

    expect(scrollIntoView).toHaveBeenCalledWith({ block: 'start', behavior: 'auto' })
  })
})

describe('waitForScrollSettle', () => {
  it('resolves once a scrollend event fires on the target', async () => {
    const target = document.createElement('div')

    const promise = waitForScrollSettle(target, 5000)
    target.dispatchEvent(new Event('scrollend'))

    await expect(promise).resolves.toBeUndefined()
  })

  it('resolves via the fallback timeout if scrollend never fires', async () => {
    vi.useFakeTimers()
    const target = document.createElement('div')

    const promise = waitForScrollSettle(target, 1000)
    await vi.advanceTimersByTimeAsync(1000)

    await expect(promise).resolves.toBeUndefined()
    vi.useRealTimers()
  })
})
