import { describe, expect, it } from 'vitest'
import { mount, type VueWrapper } from '@vue/test-utils'
import ImageCarousel from './ImageCarousel.vue'

function byLabel(wrapper: VueWrapper, label: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.attributes('aria-label') === label)
  if (!button) throw new Error(`No button with aria-label "${label}"`)
  return button
}

describe('ImageCarousel', () => {
  it('wraps forward from the last image back to the first', async () => {
    const wrapper = mount(ImageCarousel, { props: { images: ['a.jpg', 'b.jpg', 'c.jpg'] } })
    const next = byLabel(wrapper, 'Next photo')

    await next.trigger('click')
    await next.trigger('click')
    await next.trigger('click') // index 0 -> 1 -> 2 -> 0

    expect(wrapper.find('img').attributes('src')).toBe('a.jpg')
  })

  it('wraps backward from the first image to the last', async () => {
    const wrapper = mount(ImageCarousel, { props: { images: ['a.jpg', 'b.jpg', 'c.jpg'] } })

    await byLabel(wrapper, 'Previous photo').trigger('click')

    expect(wrapper.find('img').attributes('src')).toBe('c.jpg')
  })

  it('hides nav controls entirely for a single image', () => {
    const wrapper = mount(ImageCarousel, { props: { images: ['solo.jpg'] } })
    expect(wrapper.findAll('button')).toHaveLength(0)
  })

  it('renders one dot per real image, not a fixed count', () => {
    const wrapper = mount(ImageCarousel, {
      props: { images: ['a.jpg', 'b.jpg', 'c.jpg', 'd.jpg', 'e.jpg'] },
    })
    expect(wrapper.findAll('.image-carousel__dot')).toHaveLength(5)
  })

  it('a dot click jumps directly to that photo', async () => {
    const wrapper = mount(ImageCarousel, { props: { images: ['a.jpg', 'b.jpg', 'c.jpg'] } })

    await byLabel(wrapper, 'Go to photo 3').trigger('click')

    expect(wrapper.find('img').attributes('src')).toBe('c.jpg')
  })

  it('falls back to a placeholder when the image fails to load', async () => {
    const wrapper = mount(ImageCarousel, { props: { images: ['broken.jpg'] } })

    await wrapper.find('img').trigger('error')

    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.text()).toContain('Photo unavailable')
  })
})
