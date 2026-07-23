<script setup lang="ts">
import { computed } from 'vue'
import { titleLabel } from '@/utils/grading'
import type { TitleStatus } from '@/types/vehicle'

const props = defineProps<{ status: TitleStatus }>()

const label = computed(() => titleLabel(props.status))
</script>

<template>
  <span class="title-status-pill" :class="`title-status-pill--${status}`">
    <span class="title-status-pill__dot" aria-hidden="true" />
    {{ label }}
  </span>
</template>

<style scoped>
.title-status-pill {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-weight: 700;
}

.title-status-pill__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}

.title-status-pill--clean {
  color: var(--color-green-text);
}

.title-status-pill--rebuilt {
  color: var(--color-amber-text);
}

.title-status-pill--salvage {
  color: var(--color-red-text);
}
</style>
