<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useRoute } from 'vue-router'
import AppHeader from '@/components/AppHeader.vue'
import PreviewModal from '@/components/overlays/PreviewModal.vue'
import CompareBar from '@/components/overlays/CompareBar.vue'
import CompareModal from '@/components/overlays/CompareModal.vue'
import WatchlistDrawer from '@/components/overlays/WatchlistDrawer.vue'
import { useClockStore } from '@/stores/clock'
import { usePreviewStore } from '@/stores/preview'
import { useCompareStore } from '@/stores/compare'

const route = useRoute()
const clock = useClockStore()
const preview = usePreviewStore()
const compare = useCompareStore()

const backgroundRef = ref<HTMLElement | null>(null)
const isModalOpen = computed(() => !!preview.id || compare.open)

/**
 * Everything except whichever true modal (Preview/Compare) is currently
 * open goes inert while one is — simpler and more robust than a hand-rolled
 * focus trap. The Watchlist drawer/tab and Compare bar are non-modal, but
 * still background relative to an open modal, so they're inside this
 * boundary too.
 */
watch(isModalOpen, (open) => {
  if (backgroundRef.value) backgroundRef.value.inert = open
})

onMounted(() => {
  clock.start()
})
</script>

<template>
  <div class="app-shell">
    <div ref="backgroundRef" class="app-shell__background">
      <AppHeader />
      <RouterView v-slot="{ Component }">
        <component :is="Component" :key="route.path" />
      </RouterView>
      <CompareBar />
      <WatchlistDrawer />
    </div>

    <PreviewModal />
    <CompareModal />
  </div>
</template>

<style scoped>
.app-shell {
  min-height: 100vh;
}

.app-shell__background {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
}
</style>
