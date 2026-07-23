import { computed } from 'vue'
import { useAugmentedListings } from './useListingPresentation'

/**
 * Drives the Watchlist drawer's two sections. An item never appears in
 * both — "Needs Your Attention" takes priority over "Watching" for any
 * listing that's both active and bid-on.
 */
export function useWatchlistSections() {
  const { list } = useAugmentedListings()

  const activeBidItems = computed(() =>
    list.value
      .filter((listing) => listing.lifecycle === 'active' && listing.hasUserBid)
      .sort((a, b) => a.hoursRemaining - b.hoursRemaining),
  )

  const watchingItems = computed(() =>
    list.value.filter(
      (listing) => listing.isWatched && !(listing.lifecycle === 'active' && listing.hasUserBid),
    ),
  )

  const drawerCount = computed(() => activeBidItems.value.length + watchingItems.value.length)

  return { activeBidItems, watchingItems, drawerCount }
}
