<template>
  <Teleport to="body">
    <Transition name="fade">
      <div v-if="confirm.isOpen" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/50 backdrop-blur-sm">
        <div class="bg-bg-card border border-border-subtle rounded-xl shadow-xl w-full max-w-md overflow-hidden" @click.stop>
          <div class="p-6">
            <div class="flex items-start gap-4">
              <div :class="[
                'flex-shrink-0 w-10 h-10 rounded-full flex items-center justify-center',
                confirm.type === 'danger' ? 'bg-red-100 text-red-600 dark:bg-red-900/30 dark:text-red-400' :
                confirm.type === 'warning' ? 'bg-amber-100 text-amber-600 dark:bg-amber-900/30 dark:text-amber-400' :
                'bg-indigo-100 text-indigo-600 dark:bg-indigo-900/30 dark:text-indigo-400'
              ]">
                <svg v-if="confirm.type === 'danger'" class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"/></svg>
                <svg v-else-if="confirm.type === 'warning'" class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
                <svg v-else class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
              </div>
              <div class="flex-1 min-w-0">
                <h3 class="text-lg font-bold text-text-main">{{ confirm.title }}</h3>
                <p class="mt-2 text-sm text-text-muted whitespace-pre-wrap">{{ confirm.message }}</p>
              </div>
            </div>
          </div>
          <div class="px-6 py-4 bg-slate-50 dark:bg-[#161b27]/50 border-t border-border-subtle flex items-center justify-end gap-3">
            <button @click="confirm.reject" class="btn btn-secondary">
              {{ confirm.cancelText }}
            </button>
            <button @click="confirm.accept" :class="[
              'btn',
              confirm.type === 'danger' ? 'btn-danger' : 'btn-primary'
            ]">
              {{ confirm.confirmText }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup>
import { useConfirmStore } from '@/stores/confirm'
import { onMounted, onUnmounted } from 'vue'

const confirm = useConfirmStore()

const handleKeydown = (e) => {
  if (confirm.isOpen) {
    if (e.key === 'Escape') confirm.reject()
    if (e.key === 'Enter') confirm.accept()
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', handleKeydown)
})
</script>

<style scoped>
.fade-enter-active,
.fade-leave-active {
  transition: opacity 0.2s ease;
}

.fade-enter-from,
.fade-leave-to {
  opacity: 0;
}
</style>
