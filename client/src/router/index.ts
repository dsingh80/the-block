import { createRouter, createWebHistory } from 'vue-router'

/**
 * '/inventory/:id' is reachable both via the Preview Modal's "View Auction"
 * action and by a direct URL — the mock enforces "preview modal first" at
 * the click-handler level (VehicleCard opens the modal, never navigates
 * directly), not by blocking the route, so a bookmarked/shared link still
 * resolves.
 */
const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    { path: '/', redirect: '/inventory' },
    {
      path: '/inventory',
      name: 'inventory',
      component: () => import('@/views/InventoryView.vue'),
    },
    {
      path: '/inventory/:id',
      name: 'listing-details',
      component: () => import('@/views/ListingDetailsView.vue'),
      props: true,
    },
    { path: '/:pathMatch(.*)*', redirect: '/inventory' },
  ],
})

export default router
