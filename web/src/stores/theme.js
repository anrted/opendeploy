import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export const useThemeStore = defineStore('theme', () => {
  // Check local storage or system preference
  const getInitialTheme = () => {
    const saved = localStorage.getItem('theme')
    if (saved) return saved
    return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
  }

  const isDark = ref(getInitialTheme() === 'dark')

  // Apply theme to document
  const applyTheme = () => {
    if (isDark.value) {
      document.documentElement.classList.add('dark')
      localStorage.setItem('theme', 'dark')
    } else {
      document.documentElement.classList.remove('dark')
      localStorage.setItem('theme', 'light')
    }
  }

  // Watch for changes and apply
  watch(isDark, applyTheme, { immediate: true })

  const toggle = () => {
    isDark.value = !isDark.value
  }

  return { isDark, toggle }
})
