<script setup lang="ts">
import { ref } from 'vue'
import { formatGrade } from '@/utils/format'
import { vTooltip } from '@/directives/tooltip'
import type { AugmentedListing } from '@/types/listing'

const props = defineProps<{ listing: AugmentedListing }>()

const reportExpanded = ref(false)

function toggleReport() {
  reportExpanded.value = !reportExpanded.value
}

const hasRealDamageNotes = props.listing.vehicle.damage_notes.length > 0
</script>

<template>
  <div class="condition-card">
    <div class="condition-card__title">Condition</div>

    <div class="condition-card__boxes">
      <div
        v-tooltip="listing.gradeTooltip"
        class="condition-card__box"
        :class="`condition-card__box--${listing.gradeVariant}`"
      >
        <svg
          width="20"
          height="20"
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
        <div class="condition-card__grade-number">{{ formatGrade(listing.vehicle.condition_grade) }}</div>
        <div>
          <div class="condition-card__label">Condition Grade</div>
          <div class="condition-card__value">{{ listing.gradeLabel }}</div>
        </div>
      </div>

      <div class="condition-card__box condition-card__box--title" :class="`condition-card__box--${listing.titleVariant}`">
        <span class="condition-card__dot" aria-hidden="true" />
        <div>
          <div class="condition-card__label">Title Status</div>
          <div class="condition-card__value">{{ listing.titleLabel }}</div>
        </div>
      </div>
    </div>

    <p v-if="!hasRealDamageNotes" class="condition-card__damage-text">
      <strong>Damage report:</strong> {{ listing.damageList[0] }}
    </p>
    <template v-else>
      <div class="condition-card__damage-heading">Damage report:</div>
      <ul class="condition-card__damage-list">
        <li v-for="note in listing.damageList" :key="note">{{ note }}</li>
      </ul>
    </template>

    <button type="button" class="condition-card__toggle" @click="toggleReport">
      {{ reportExpanded ? 'Hide full condition report' : 'View full condition report' }} →
    </button>
    <p v-if="reportExpanded" class="condition-card__report">{{ listing.vehicle.condition_report }}</p>
  </div>
</template>

<style scoped>
.condition-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 20px;
}

.condition-card__title {
  font-size: 16px;
  font-weight: 800;
  color: var(--color-navy);
  margin-bottom: 14px;
}

.condition-card__boxes {
  display: flex;
  flex-wrap: wrap;
  gap: 16px;
  margin-bottom: 14px;
}

.condition-card__box {
  display: flex;
  align-items: center;
  gap: 12px;
  border-radius: 12px;
  padding: 12px 18px;
  cursor: help;
}

.condition-card__box--title {
  cursor: default;
}

.condition-card__box--good {
  background: var(--color-green-bg);
  color: var(--color-green-text);
}

.condition-card__box--fair {
  background: var(--color-amber-bg);
  color: var(--color-amber-text);
}

.condition-card__box--poor {
  background: var(--color-red-bg);
  color: var(--color-red-text);
}

.condition-card__box--clean {
  background: var(--color-green-bg);
  color: var(--color-green-text);
}

.condition-card__box--rebuilt {
  background: var(--color-amber-bg);
  color: var(--color-amber-text);
}

.condition-card__box--salvage {
  background: var(--color-red-bg);
  color: var(--color-red-text);
}

.condition-card__grade-number {
  font-size: 28px;
  font-weight: 800;
}

.condition-card__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: currentColor;
  flex-shrink: 0;
}

.condition-card__label {
  font-size: 11px;
  color: var(--color-faint);
  text-transform: uppercase;
  font-weight: 700;
  letter-spacing: 0.03em;
}

.condition-card__value {
  font-size: 14px;
  font-weight: 700;
}

.condition-card__damage-text {
  font-size: 13px;
  color: var(--color-muted);
  margin: 0 0 12px;
}

.condition-card__damage-text strong {
  color: var(--color-text);
}

.condition-card__damage-heading {
  font-size: 13px;
  font-weight: 700;
  color: var(--color-text);
  margin-bottom: 6px;
}

.condition-card__damage-list {
  margin: 0 0 12px;
  padding-left: 18px;
  font-size: 13px;
  color: var(--color-muted);
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.condition-card__toggle {
  border: none;
  background: none;
  padding: 0;
  font-size: 13px;
  font-weight: 700;
  color: var(--color-accent);
  cursor: pointer;
}

.condition-card__report {
  margin: 10px 0 0;
  font-size: 13px;
  color: var(--color-muted);
  line-height: 1.5;
}
</style>
