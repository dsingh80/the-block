<template>
  <header class="app-header">
    <div class="app-header__inner">
      <RouterLink to="/inventory" class="app-header__logo">
        <span class="app-header__logo-open">OPEN</span><span class="app-header__logo-lane">LANE</span>
      </RouterLink>
    </div>
  </header>
</template>

<style scoped>
.app-header {
  position: sticky;
  top: 0;
  z-index: var(--z-header);
  background: var(--color-navy);
}

.app-header__inner {
  display: flex;
  align-items: center;
  justify-content: center; /* should switch to space-between when we have actual links */
  gap: 20px;
  padding: 14px 28px;
  flex-wrap: wrap;
}

.app-header__logo {
  display: flex;
  align-items: center;
  gap: 2px;
  cursor: pointer;
  font-weight: 800;
  font-size: 19px;
  letter-spacing: -0.02em;
  text-decoration: none;
}

.app-header__logo-open {
  color: var(--color-surface);
}

.app-header__logo-lane {
  color: var(--color-logo-accent);
}

.app-header__nav {
  display: flex;
  gap: 22px;
  align-items: center;
  flex-wrap: wrap;
}

.app-header__nav-item {
  color: var(--color-surface);
  font-weight: 600;
  font-size: 14px;
  text-decoration: none;
  cursor: pointer;
  padding-bottom: 3px;
  border-bottom: 2px solid transparent;
}

.app-header__nav-item--active {
  border-bottom-color: var(--color-logo-accent);
}
</style>
