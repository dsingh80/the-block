<script setup lang="ts">
import { computed } from 'vue'
import { gradeVariant, gradeLabel, gradeTooltip } from '@/utils/grading'
import { formatGrade } from '@/utils/format'

const props = defineProps<{ grade: number }>()

const variant = computed(() => gradeVariant(props.grade))
const label = computed(() => gradeLabel(props.grade))
const tooltip = computed(() => gradeTooltip(props.grade))
const formatted = computed(() => formatGrade(props.grade))
</script>

<template>
  <span
    class="grade-pill"
    :class="`grade-pill--${variant}`"
    :title="tooltip"
    :aria-label="`Condition grade ${formatted}, ${label}`"
  >
    <svg
      width="11"
      height="11"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      stroke-width="2"
      stroke-linecap="round"
      stroke-linejoin="round"
      aria-hidden="true"
    >
      <path d="M12 3l7 3v6c0 4.5-3 7.5-7 9-4-1.5-7-4.5-7-9V6l7-3Z" />
    </svg>
    {{ formatted }}
  </span>
</template>

<style scoped>
.grade-pill {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  font-size: 12px;
  font-weight: 800;
  padding: 3px 9px;
  border-radius: 7px;
  white-space: nowrap;
  cursor: help;
}

.grade-pill--good {
  background: var(--color-green-bg);
  color: var(--color-green-text);
}

.grade-pill--fair {
  background: var(--color-amber-bg);
  color: var(--color-amber-text);
}

.grade-pill--poor {
  background: var(--color-red-bg);
  color: var(--color-red-text);
}
</style>
