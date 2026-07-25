import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import axios from 'axios'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(localStorage.getItem('od_access_token') || '')
  const refreshToken = ref(localStorage.getItem('od_refresh_token') || '')
  const user = ref(JSON.parse(localStorage.getItem('od_user') || 'null'))

  const isAuthenticated = computed(() => !!accessToken.value)

  async function login(username, password) {
    const { data } = await axios.post('/api/v1/auth/login', { username, password })
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    localStorage.setItem('od_access_token', data.access_token)
    localStorage.setItem('od_refresh_token', data.refresh_token)
    // Fetch user profile
    const { data: me } = await axios.get('/api/v1/auth/me', {
      headers: { Authorization: `Bearer ${data.access_token}` },
    })
    user.value = me
    localStorage.setItem('od_user', JSON.stringify(me))
  }

  async function refresh() {
    const { data } = await axios.post('/api/v1/auth/refresh', {
      refresh_token: refreshToken.value,
    })
    accessToken.value = data.access_token
    refreshToken.value = data.refresh_token
    localStorage.setItem('od_access_token', data.access_token)
    localStorage.setItem('od_refresh_token', data.refresh_token)
  }

  async function logout() {
    try {
      await axios.post('/api/v1/auth/logout', null, {
        headers: { Authorization: `Bearer ${accessToken.value}` },
      })
    } catch { /* ignore */ }
    accessToken.value = ''
    refreshToken.value = ''
    user.value = null
    localStorage.removeItem('od_access_token')
    localStorage.removeItem('od_refresh_token')
    localStorage.removeItem('od_user')
  }

  return { accessToken, refreshToken, user, isAuthenticated, login, refresh, logout }
})
