import axios from 'axios'
import { useAuthStore } from '@/stores/auth'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
})

let csrfToken = null
let csrfPromise = null

// Fetch CSRF token once
async function fetchCsrfToken() {
  if (csrfToken) return csrfToken
  if (csrfPromise) return csrfPromise

  csrfPromise = axios.get('/api/v1/auth/csrf').then((res) => {
    csrfToken = res.headers['x-csrf-token']
    return csrfToken
  }).catch(() => null)

  return csrfPromise
}

// Attach JWT token and CSRF token to every request
api.interceptors.request.use(async (config) => {
  const auth = useAuthStore()
  if (auth.accessToken) {
    config.headers.Authorization = `Bearer ${auth.accessToken}`
  }

  // Attach CSRF for mutating requests
  if (['post', 'put', 'patch', 'delete'].includes(config.method?.toLowerCase())) {
    const token = await fetchCsrfToken()
    if (token) {
      config.headers['X-CSRF-Token'] = token
    }
  }

  return config
})

// Auto-refresh token on 401
api.interceptors.response.use(
  (response) => {
    // Update CSRF token if backend sends a new one
    if (response.headers['x-csrf-token']) {
      csrfToken = response.headers['x-csrf-token']
    }
    return response
  },
  async (error) => {
    const originalRequest = error.config
    
    // Refresh CSRF if it was forbidden
    if (error.response?.status === 403 && error.response?.data?.error?.code === 'forbidden' && !originalRequest._csrfRetry) {
      originalRequest._csrfRetry = true
      csrfToken = null // force refresh
      const token = await fetchCsrfToken()
      if (token) {
        originalRequest.headers['X-CSRF-Token'] = token
        return api(originalRequest)
      }
    }

    if (error.response?.status === 401 && !originalRequest._retry) {
      originalRequest._retry = true
      try {
        const auth = useAuthStore()
        await auth.refresh()
        originalRequest.headers.Authorization = `Bearer ${auth.accessToken}`
        return api(originalRequest)
      } catch {
        const auth = useAuthStore()
        auth.logout()
        window.location.href = '/login'
      }
    }
    return Promise.reject(error)
  }
)

export default api

export function apiErrorMessage(error, fallback = 'Request failed') {
  return error?.response?.data?.error?.message || error?.message || fallback
}

