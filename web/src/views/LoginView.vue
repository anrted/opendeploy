<template>
  <div class="min-h-screen flex flex-col items-center justify-center p-4" style="background: radial-gradient(ellipse at top, #1a1040 0%, #0f1117 70%);">
    
    <!-- Top Right Language Switcher -->
    <div class="absolute top-4 right-4">
      <LanguageSwitcher />
    </div>

    <div class="w-full max-w-md">
      <!-- Logo -->
      <div class="text-center mb-8">
        <div class="inline-flex items-center justify-center w-16 h-16 rounded-2xl bg-gradient-to-br from-indigo-500 to-violet-600 shadow-2xl shadow-indigo-500/40 mb-4">
          <svg class="w-8 h-8 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                  d="M5 12h14M5 12a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v4a2 2 0 01-2 2M5 12a2 2 0 00-2 2v4a2 2 0 002 2h14a2 2 0 002-2v-4a2 2 0 00-2-2" />
          </svg>
        </div>
        <h1 class="text-3xl font-bold bg-gradient-to-r from-indigo-400 to-violet-400 bg-clip-text text-transparent">
          OpenDeploy
        </h1>
        <p class="text-[#64748b] text-sm mt-2">Server Management Panel</p>
      </div>

      <!-- Login form -->
      <div class="card">
        <h2 class="text-lg font-semibold text-white mb-6">{{ $t('login.title') }}</h2>

        <form @submit.prevent="handleLogin" class="space-y-4">
          <div>
            <label class="label">{{ $t('login.username') }}</label>
            <input
              id="username"
              v-model="form.username"
              type="text"
              class="input"
              placeholder="admin"
              autocomplete="username"
              required
            />
          </div>

          <div>
            <label class="label">{{ $t('login.password') }}</label>
            <input
              id="password"
              v-model="form.password"
              type="password"
              class="input"
              placeholder="••••••••"
              autocomplete="current-password"
              required
            />
          </div>

          <div v-if="error" class="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-400">
            {{ error }}
          </div>

          <button id="login-btn" type="submit" class="btn-primary w-full justify-center" :disabled="loading">
            <svg v-if="loading" class="w-4 h-4 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/>
              <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/>
            </svg>
            {{ loading ? $t('login.signingIn') : $t('login.signIn') }}
          </button>
        </form>
      </div>

      <p class="text-center text-xs text-[#4a5568] mt-6">
        OpenDeploy — Open Source Server Management
      </p>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '@/stores/auth'
import LanguageSwitcher from '@/components/LanguageSwitcher.vue'

const auth = useAuthStore()
const router = useRouter()

const form = reactive({ username: '', password: '' })
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  error.value = ''
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    router.push('/')
  } catch (e) {
    error.value = e.response?.data?.error?.message || 'Invalid credentials'
  } finally {
    loading.value = false
  }
}
</script>
