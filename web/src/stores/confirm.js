import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useConfirmStore = defineStore('confirm', () => {
  const isOpen = ref(false)
  const title = ref('')
  const message = ref('')
  const confirmText = ref('Confirm')
  const cancelText = ref('Cancel')
  const type = ref('warning') // 'warning', 'danger', 'info'
  
  let resolvePromise = null

  const require = (options) => {
    title.value = options.title || 'Are you sure?'
    message.value = options.message || ''
    confirmText.value = options.confirmText || 'Confirm'
    cancelText.value = options.cancelText || 'Cancel'
    type.value = options.type || 'warning'
    
    isOpen.value = true

    return new Promise((resolve) => {
      resolvePromise = resolve
    })
  }

  const accept = () => {
    isOpen.value = false
    if (resolvePromise) resolvePromise(true)
  }

  const reject = () => {
    isOpen.value = false
    if (resolvePromise) resolvePromise(false)
  }

  return { isOpen, title, message, confirmText, cancelText, type, require, accept, reject }
})
