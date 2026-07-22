<script setup lang="ts">
import { storeToRefs } from 'pinia'
import { useInventoryFiltersStore, type StatusFilter } from '@/stores/inventoryFilters'
import { MAKE_OPTIONS } from '@/data/vehicles'

const filters = useInventoryFiltersStore()
const { search, makeFilter, statusFilter, sortBy } = storeToRefs(filters)

const statusChips: { value: StatusFilter; label: string }[] = [
  { value: 'all', label: 'All' },
  { value: 'active', label: 'Active' },
  { value: 'upcoming', label: 'Upcoming' },
  { value: 'ended', label: 'Ended' },
]
</script>

<template>
  <div class="filter-bar">
    <div class="filter-bar__field filter-bar__field--search">
      <label class="sr-only" for="inventory-search">Search make, model, VIN, or seller</label>
      <input
        id="inventory-search"
        v-model="search"
        type="text"
        placeholder="Search make, model, VIN..."
      />
    </div>

    <div class="filter-bar__field">
      <label class="sr-only" for="inventory-make">Make</label>
      <select id="inventory-make" v-model="makeFilter">
        <option value="all">All Makes</option>
        <option v-for="make in MAKE_OPTIONS" :key="make" :value="make">{{ make }}</option>
      </select>
    </div>

    <div class="filter-bar__field">
      <label class="sr-only" for="inventory-sort">Sort</label>
      <select id="inventory-sort" v-model="sortBy">
        <option value="ending">Sort: Ending Soonest</option>
        <option value="price-low">Sort: Price Low to High</option>
        <option value="price-high">Sort: Price High to Low</option>
        <option value="year">Sort: Newest Year</option>
      </select>
    </div>

    <div class="filter-bar__chips">
      <button
        v-for="chip in statusChips"
        :key="chip.value"
        type="button"
        class="filter-bar__chip"
        :class="{ 'filter-bar__chip--active': statusFilter === chip.value }"
        :aria-pressed="statusFilter === chip.value"
        @click="statusFilter = chip.value"
      >
        {{ chip.label }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.filter-bar {
  display: flex;
  flex-wrap: wrap;
  gap: 12px;
  align-items: center;
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 12px;
  padding: 14px 16px;
}

.filter-bar__field {
  flex: 1 1 140px;
  min-width: 0;
}

.filter-bar__field--search {
  flex: 2 1 200px;
}

.filter-bar input,
.filter-bar select {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 10px 12px;
  font-size: 14px;
  background: var(--color-surface);
  outline: none;
}

.filter-bar__chips {
  display: flex;
  gap: 8px;
  flex-wrap: wrap;
}

.filter-bar__chip {
  border: 1px solid var(--color-border);
  background: var(--color-surface);
  color: var(--color-navy);
  font-weight: 600;
  font-size: 13px;
  padding: 9px 14px;
  border-radius: 20px;
  cursor: pointer;
}

.filter-bar__chip--active {
  border-color: var(--color-navy);
  background: var(--color-navy);
  color: var(--color-surface);
}
</style>
