<script setup lang="ts">
import { ref, computed, watch } from 'vue'

const props = withDefaults(defineProps<{ images: string[]; heightPx?: number }>(), {
  heightPx: 320,
})

const activeIndex = ref(0)
const imageFailed = ref(false)

const activeImage = computed(() => props.images[activeIndex.value])

watch(activeIndex, () => {
  imageFailed.value = false
})

function prev() {
  activeIndex.value = (activeIndex.value - 1 + props.images.length) % props.images.length
}

function next() {
  activeIndex.value = (activeIndex.value + 1) % props.images.length
}

function goTo(index: number) {
  activeIndex.value = index
}
</script>

<template>
  <div
    class="image-carousel"
    :style="{ height: heightPx + 'px' }"
    tabindex="0"
    @keydown.left="prev"
    @keydown.right="next"
  >
    <img
      v-if="activeImage && !imageFailed"
      :src="activeImage"
      :alt="`Photo ${activeIndex + 1} of ${images.length}`"
      class="image-carousel__image"
      loading="lazy"
      @error="imageFailed = true"
    />
    <div v-else class="image-carousel__fallback">Photo unavailable</div>

    <template v-if="images.length > 1">
      <button
        type="button"
        class="image-carousel__nav image-carousel__nav--prev"
        aria-label="Previous photo"
        @click="prev"
      >
        &#8249;
      </button>
      <button
        type="button"
        class="image-carousel__nav image-carousel__nav--next"
        aria-label="Next photo"
        @click="next"
      >
        &#8250;
      </button>
      <div class="image-carousel__dots">
        <button
          v-for="(image, index) in images"
          :key="image + index"
          type="button"
          class="image-carousel__dot"
          :class="{ 'image-carousel__dot--active': index === activeIndex }"
          :aria-label="`Go to photo ${index + 1}`"
          @click="goTo(index)"
        />
      </div>
    </template>
  </div>
</template>

<style scoped>
.image-carousel {
  position: relative;
  border-radius: 14px;
  overflow: hidden;
  background: var(--color-navy);
  outline: none;
}

.image-carousel__image {
  width: 100%;
  height: 100%;
  display: block;
  object-fit: cover;
}

.image-carousel__fallback {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: var(--color-inactive);
  font-size: 13px;
}

.image-carousel__nav {
  position: absolute;
  top: 50%;
  transform: translateY(-50%);
  width: 38px;
  height: 38px;
  border-radius: 50%;
  border: none;
  background: var(--color-overlay-dark);
  color: var(--color-surface);
  font-size: 18px;
  line-height: 1;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
}

.image-carousel__nav--prev {
  left: 12px;
}

.image-carousel__nav--next {
  right: 12px;
}

.image-carousel__dots {
  position: absolute;
  bottom: 12px;
  left: 0;
  right: 0;
  display: flex;
  justify-content: center;
  gap: 6px;
}

.image-carousel__dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  border: none;
  padding: 0;
  background: var(--color-inactive);
  cursor: pointer;
}

.image-carousel__dot--active {
  background: var(--color-logo-accent);
}
</style>
