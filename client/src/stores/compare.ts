import { ref } from 'vue'
import { defineStore } from 'pinia'

/** Max-2 selection is enforced here, not just a disabled checkbox in the UI. */
export const useCompareStore = defineStore('compare', () => {
  const ids = ref<string[]>([])
  const open = ref(false)
  const detailsOpen = ref(false)

  function toggle(id: string) {
    const index = ids.value.indexOf(id)
    if (index !== -1) {
      ids.value.splice(index, 1)
      return
    }
    if (ids.value.length >= 2) return
    ids.value.push(id)
  }

  function remove(id: string) {
    ids.value = ids.value.filter((existing) => existing !== id)
  }

  function clear() {
    ids.value = []
    open.value = false
  }

  function openModal() {
    if (ids.value.length === 2) open.value = true
  }

  function closeModal() {
    open.value = false
  }

  function toggleDetails() {
    detailsOpen.value = !detailsOpen.value
  }

  return { ids, open, detailsOpen, toggle, remove, clear, openModal, closeModal, toggleDetails }
})
