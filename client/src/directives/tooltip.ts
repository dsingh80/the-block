import type { Directive } from 'vue'

/**
 * Single shared tooltip node reused by every `v-tooltip` trigger on the
 * page, imperatively shown/moved/hidden — avoids a Teleport + v-if per
 * trigger (there can be dozens on the inventory grid).
 */
let bubble: HTMLElement | null = null
let activeEl: HTMLElement | null = null

const text = new WeakMap<HTMLElement, string>()
const cleanup = new WeakMap<HTMLElement, () => void>()

function ensureBubble(): HTMLElement {
  if (!bubble) {
    bubble = document.createElement('span')
    bubble.className = 'v-tooltip-bubble'
    bubble.setAttribute('role', 'tooltip')
    bubble.setAttribute('aria-hidden', 'true')
    document.body.appendChild(bubble)
  }
  return bubble
}

function position(el: HTMLElement, el2: HTMLElement) {
  const anchor = el.getBoundingClientRect()
  const rect = el2.getBoundingClientRect()
  const margin = 8
  const half = rect.width / 2
  const center = anchor.left + anchor.width / 2
  const left = Math.min(Math.max(center, half + margin), window.innerWidth - half - margin)
  const top = Math.max(anchor.top - margin, rect.height + margin)
  el2.style.left = `${left}px`
  el2.style.top = `${top}px`
}

function reposition() {
  if (activeEl && bubble) position(activeEl, bubble)
}

function show(el: HTMLElement) {
  const value = text.get(el)
  if (!value) return
  const el2 = ensureBubble()
  el2.textContent = value
  el2.style.display = 'block'
  activeEl = el
  position(el, el2)
  window.addEventListener('resize', reposition)
}

function hide(el: HTMLElement) {
  if (activeEl !== el) return
  activeEl = null
  if (bubble) bubble.style.display = 'none'
  window.removeEventListener('resize', reposition)
}

// Dismiss on an outside tap — the trigger's own click handler always shows
// (never toggles), so a real click doesn't race the focus event it also
// fires and immediately re-close itself.
document.addEventListener(
  'click',
  (event) => {
    if (activeEl && !activeEl.contains(event.target as Node)) hide(activeEl)
  },
  true,
)

export const vTooltip: Directive<HTMLElement, string> = {
  mounted(el, binding) {
    text.set(el, binding.value)
    if (el.tabIndex < 0) el.tabIndex = 0

    const onShow = () => show(el)
    const onHide = () => hide(el)

    el.addEventListener('mouseenter', onShow)
    el.addEventListener('mouseleave', onHide)
    el.addEventListener('focus', onShow)
    el.addEventListener('blur', onHide)
    el.addEventListener('click', onShow)

    cleanup.set(el, () => {
      el.removeEventListener('mouseenter', onShow)
      el.removeEventListener('mouseleave', onHide)
      el.removeEventListener('focus', onShow)
      el.removeEventListener('blur', onHide)
      el.removeEventListener('click', onShow)
    })
  },
  updated(el, binding) {
    text.set(el, binding.value)
  },
  unmounted(el) {
    hide(el)
    cleanup.get(el)?.()
    cleanup.delete(el)
    text.delete(el)
  },
}
