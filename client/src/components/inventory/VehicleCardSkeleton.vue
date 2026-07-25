<template>
  <div class="vehicle-card-skeleton" aria-hidden="true">
    <div class="vehicle-card-skeleton__media"></div>
    <div class="vehicle-card-skeleton__body">
      <div class="vehicle-card-skeleton__line vehicle-card-skeleton__line--title"></div>
      <div class="vehicle-card-skeleton__line vehicle-card-skeleton__line--trim"></div>
      <div class="vehicle-card-skeleton__line vehicle-card-skeleton__line--meta"></div>
      <div class="vehicle-card-skeleton__line vehicle-card-skeleton__line--price"></div>
      <div class="vehicle-card-skeleton__line vehicle-card-skeleton__line--actions"></div>
    </div>
  </div>
</template>

<style scoped>
.vehicle-card-skeleton {
  background: var(--color-surface);
  border: 1px solid var(--color-border);
  border-radius: 14px;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.vehicle-card-skeleton__media {
  height: 170px;
  background: var(--color-page-bg);
}

.vehicle-card-skeleton__body {
  padding: 14px 16px 16px;
  display: flex;
  flex-direction: column;
  gap: 10px;
  flex: 1;
}

.vehicle-card-skeleton__line {
  border-radius: 4px;
  background: var(--color-divider);
}

.vehicle-card-skeleton__line--title {
  height: 15px;
  width: 70%;
}

.vehicle-card-skeleton__line--trim {
  height: 12px;
  width: 40%;
}

.vehicle-card-skeleton__line--meta {
  height: 12px;
  width: 90%;
}

.vehicle-card-skeleton__line--price {
  height: 18px;
  width: 50%;
  margin-top: auto;
}

.vehicle-card-skeleton__line--actions {
  height: 34px;
  width: 100%;
}

@media (prefers-reduced-motion: no-preference) {
  .vehicle-card-skeleton__media,
  .vehicle-card-skeleton__line {
    animation: skeleton-pulse 1.6s ease-in-out infinite;
  }
}
</style>
