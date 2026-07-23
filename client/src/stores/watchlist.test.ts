import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useWatchlistStore } from './watchlist'
import { WATCHLIST_HIGHLIGHT_MS } from '@/utils/constants'

describe('useWatchlistStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('starts empty', () => {
    const watchlist = useWatchlistStore()
    expect(watchlist.ids.size).toBe(0)
  })

  it('adding opens the drawer and marks the id recently added', () => {
    const watchlist = useWatchlistStore()
    watchlist.toggle('a')
    expect(watchlist.ids.has('a')).toBe(true)
    expect(watchlist.drawerOpen).toBe(true)
    expect(watchlist.recentlyAddedId).toBe('a')
  })

  it('toggling an already-watched id removes it', () => {
    const watchlist = useWatchlistStore()
    watchlist.toggle('a')
    watchlist.toggle('a')
    expect(watchlist.ids.has('a')).toBe(false)
  })

  it('clears the highlight after the timeout', () => {
    const watchlist = useWatchlistStore()
    watchlist.toggle('a')
    expect(watchlist.recentlyAddedId).toBe('a')

    vi.advanceTimersByTime(WATCHLIST_HIGHLIGHT_MS)
    expect(watchlist.recentlyAddedId).toBeNull()
  })

  it('toggleDrawer flips drawerOpen independent of watchlist membership', () => {
    const watchlist = useWatchlistStore()
    expect(watchlist.drawerOpen).toBe(false)
    watchlist.toggleDrawer()
    expect(watchlist.drawerOpen).toBe(true)
    watchlist.toggleDrawer()
    expect(watchlist.drawerOpen).toBe(false)
  })
})
