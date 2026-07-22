import { ref } from 'vue'
import { defineStore } from 'pinia'

/** Backs the Preview Modal only — not URL-addressable, matching the mock. */
export const usePreviewStore = defineStore('preview', () => {
  const id = ref<string | null>(null)
  const imageIndex = ref(0)

  function open(vehicleId: string) {
    id.value = vehicleId
    imageIndex.value = 0
  }

  function close() {
    id.value = null
  }

  return { id, imageIndex, open, close }
})
