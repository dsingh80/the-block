import { ref } from 'vue'
import { defineStore } from 'pinia'
import { CLOCK_TICK_MS } from '@/utils/constants'

/**
 * The one reactive source of "now" in the app. Every lifecycle-dependent
 * read must go through this store, never a direct Date.now()/new Date()
 * call — see guidelines/03-guardrails.md. Real wall-clock time, no
 * synthetic/dataset-relative offset (confirmed decision, see the README's
 * Assumptions and Scope section for what that means against the committed
 * dataset).
 */
export const useClockStore = defineStore('clock', () => {
  const effectiveNow = ref(Date.now())
  let timer: ReturnType<typeof setInterval> | undefined

  function start() {
    if (timer) return
    timer = setInterval(() => {
      effectiveNow.value = Date.now()
    }, CLOCK_TICK_MS)
  }

  function stop() {
    clearInterval(timer)
    timer = undefined
  }

  return { effectiveNow, start, stop }
})
