import { ref } from 'vue'
import { defineStore } from 'pinia'
import { WATCHLIST_HIGHLIGHT_MS } from '@/utils/constants'

/** Starts empty — no hand-picked seed data, an unexplained pre-populated watchlist on first load would read as more confusing than a clean start. */
export const useWatchlistStore = defineStore('watchlist', () => {
  const ids = ref<Set<string>>(new Set())
  const drawerOpen = ref(false)
  const recentlyAddedId = ref<string | null>(null)

  let highlightTimer: ReturnType<typeof setTimeout> | undefined

  function toggle(id: string) {
    const next = new Set(ids.value)
    const adding = !next.has(id)

    if (adding) {
      next.add(id)
    } else {
      next.delete(id)
    }
    ids.value = next

    if (adding) {
      drawerOpen.value = true
      recentlyAddedId.value = id
      clearTimeout(highlightTimer)
      highlightTimer = setTimeout(() => {
        if (recentlyAddedId.value === id) recentlyAddedId.value = null
      }, WATCHLIST_HIGHLIGHT_MS)
    }
  }

  function toggleDrawer() {
    drawerOpen.value = !drawerOpen.value
  }

  return { ids, drawerOpen, recentlyAddedId, toggle, toggleDrawer }
})
