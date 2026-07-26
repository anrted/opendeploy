import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useConfirmStore } from './confirm'

describe('Confirm Store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('initializes with default state', () => {
    const store = useConfirmStore()
    expect(store.isOpen).toBe(false)
    expect(store.title).toBe('')
    expect(store.message).toBe('')
  })

  it('opens with correct options and resolves on confirm', async () => {
    const store = useConfirmStore()
    
    // Call require which returns a promise
    const promise = store.require({
      title: 'Test Title',
      message: 'Test Message',
      confirmText: 'Yes',
      cancelText: 'No',
      type: 'danger'
    })

    expect(store.isOpen).toBe(true)
    expect(store.title).toBe('Test Title')
    expect(store.message).toBe('Test Message')
    expect(store.confirmText).toBe('Yes')
    expect(store.cancelText).toBe('No')
    expect(store.type).toBe('danger')

    // Simulate user confirming
    store.accept()

    const result = await promise
    expect(result).toBe(true)
    expect(store.isOpen).toBe(false)
  })

  it('resolves false on cancel', async () => {
    const store = useConfirmStore()
    
    const promise = store.require({
      title: 'Test',
      message: 'Test'
    })

    store.reject()

    const result = await promise
    expect(result).toBe(false)
    expect(store.isOpen).toBe(false)
  })
})
