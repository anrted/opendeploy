import axios from 'axios'
import { useAuthStore } from '@/stores/auth'

const api = axios.create({
  baseURL: '/api/v1',
  timeout: 30000,
  withCredentials: true,
})

let csrfToken = null
let csrfPromise = null

// Fetch CSRF token once
async function fetchCsrfToken() {
  if (csrfToken) return csrfToken
  if (csrfPromise) return csrfPromise

  csrfPromise = axios
    .get('/api/v1/auth/csrf', { withCredentials: true })
    .then((res) => {
      const token = res.headers['x-csrf-token']
      if (!token) {
        throw new Error('CSRF token was not returned by the server')
      }
      csrfToken = token
      return token
    })
    .finally(() => {
      // A settled promise must not pin a failed or stale token forever.
      csrfPromise = null
    })

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
    if (error.response?.status === 403 && error.response?.data?.error?.code?.toLowerCase() === 'forbidden' && !originalRequest._csrfRetry) {
      originalRequest._csrfRetry = true
      csrfToken = null // refresh both the token and its matching cookie
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
    const payload = error.response?.data?.error
    if (error.response?.status !== 401 && typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('opendeploy:toast', {
        detail: {
          message: payload?.message || error.message || 'Request failed',
          recommendation: payload?.recommendation || '',
          error_id: payload?.error_id || ''
        }
      }))
    }
    return Promise.reject(error)
  }
)

export default api

export function apiErrorMessage(error, fallback = 'Request failed') {
  return error?.response?.data?.error?.message || error?.message || fallback
}

