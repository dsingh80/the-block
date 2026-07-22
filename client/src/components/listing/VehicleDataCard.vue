<script setup lang="ts">
import { computed } from 'vue'
import { capitalize } from '@/utils/format'
import type { Vehicle } from '@/types/vehicle'

const props = defineProps<{ vehicle: Vehicle }>()

const specs = computed(() => [
  { label: 'Engine', value: props.vehicle.engine },
  { label: 'Transmission', value: capitalize(props.vehicle.transmission) },
  { label: 'Drivetrain', value: props.vehicle.drivetrain },
  { label: 'Exterior', value: props.vehicle.exterior_color },
  { label: 'Interior', value: props.vehicle.interior_color },
  { label: 'Fuel Type', value: capitalize(props.vehicle.fuel_type) },
])
</script>

<template>
  <div class="vehicle-data-card">
    <div class="vehicle-data-card__title">Vehicle Data</div>
    <div class="vehicle-data-card__grid">
      <div v-for="spec in specs" :key="spec.label" class="vehicle-data-card__item">
        <div class="vehicle-data-card__label">{{ spec.label }}</div>
        <div class="vehicle-data-card__value">{{ spec.value }}</div>
      </div>
      <div class="vehicle-data-card__item">
        <div class="vehicle-data-card__label">VIN</div>
        <a
          class="vehicle-data-card__vin"
          href="https://www.nicb.org/vincheck"
          target="_blank"
          rel="noopener noreferrer"
        >
          {{ vehicle.vin }} ↗
        </a>
      </div>
    </div>
  </div>
</template>

<style scoped>
.vehicle-data-card {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  padding: 20px;
}

.vehicle-data-card__title {
  font-size: 16px;
  font-weight: 800;
  color: var(--color-navy);
  margin-bottom: 14px;
}

.vehicle-data-card__grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 14px;
}

.vehicle-data-card__label {
  font-size: 11px;
  color: var(--color-faint);
  font-weight: 700;
  text-transform: uppercase;
}

.vehicle-data-card__value {
  font-size: 14px;
  font-weight: 600;
  margin-top: 2px;
}

.vehicle-data-card__vin {
  font-size: 14px;
  font-weight: 700;
  color: var(--color-accent);
  text-decoration: none;
  margin-top: 2px;
  display: inline-block;
}

.vehicle-data-card__vin:hover {
  text-decoration: underline;
}
</style>
