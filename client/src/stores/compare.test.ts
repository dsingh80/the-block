import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useCompareStore } from './compare'

describe('useCompareStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('adds up to 2 ids', () => {
    const compare = useCompareStore()
    compare.toggle('a')
    compare.toggle('b')
    expect(compare.ids).toEqual(['a', 'b'])
  })

  it('is a no-op adding a 3rd id', () => {
    const compare = useCompareStore()
    compare.toggle('a')
    compare.toggle('b')
    compare.toggle('c')
    expect(compare.ids).toEqual(['a', 'b'])
  })

  it('removes an id that is already selected', () => {
    const compare = useCompareStore()
    compare.toggle('a')
    compare.toggle('b')
    compare.toggle('a')
    expect(compare.ids).toEqual(['b'])
  })

  it('only opens the modal with exactly 2 selected', () => {
    const compare = useCompareStore()
    compare.toggle('a')
    compare.openModal()
    expect(compare.open).toBe(false)

    compare.toggle('b')
    compare.openModal()
    expect(compare.open).toBe(true)
  })

  it('clear empties the selection and closes the modal', () => {
    const compare = useCompareStore()
    compare.toggle('a')
    compare.toggle('b')
    compare.openModal()
    compare.clear()
    expect(compare.ids).toEqual([])
    expect(compare.open).toBe(false)
  })
})
