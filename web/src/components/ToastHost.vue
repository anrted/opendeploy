<template>
  <div class="pointer-events-none fixed right-4 top-4 z-[100] flex w-[min(26rem,calc(100vw-2rem))] flex-col gap-3">
    <transition-group name="toast">
      <div v-for="toast in toasts" :key="toast.id" class="pointer-events-auto rounded-xl border border-red-500/30 bg-slate-950/95 p-4 text-sm text-slate-200 shadow-2xl backdrop-blur">
        <div class="flex items-start gap-3">
          <div class="mt-0.5 rounded-full bg-red-500/15 p-1 text-red-400">!</div>
          <div class="min-w-0 flex-1">
            <div class="font-semibold text-white">{{ toast.message }}</div>
            <div v-if="toast.recommendation" class="mt-1 text-xs text-slate-400">{{ toast.recommendation }}</div>
            <div v-if="toast.errorId" class="mt-2 font-mono text-[11px] text-slate-500">ID: {{ toast.errorId }}</div>
          </div>
          <button class="text-slate-500 hover:text-white" type="button" @click="remove(toast.id)">×</button>
        </div>
      </div>
    </transition-group>
  </div>
</template>

<script setup>
import { onMounted, onUnmounted, ref } from 'vue'

const toasts = ref([])
let sequence = 0

function remove(id) {
  toasts.value = toasts.value.filter(toast => toast.id !== id)
}

function receive(event) {
  const detail = event.detail || {}
  const id = ++sequence
  toasts.value.push({
    id,
    message: detail.message || 'Request failed',
    recommendation: detail.recommendation || '',
    errorId: detail.error_id || ''
  })
  window.setTimeout(() => remove(id), 8000)
}

onMounted(() => window.addEventListener('opendeploy:toast', receive))
onUnmounted(() => window.removeEventListener('opendeploy:toast', receive))
</script>

<style scoped>
.toast-enter-active, .toast-leave-active { transition: all 0.2s ease; }
.toast-enter-from, .toast-leave-to { opacity: 0; transform: translateY(-8px); }
</style>
