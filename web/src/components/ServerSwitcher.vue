<template>
  <div ref="root" class="relative">
    <button
      class="w-full rounded-xl border border-border-subtle bg-bg-base/70 p-3 text-left transition hover:border-indigo-500/50"
      aria-haspopup="listbox"
      :aria-expanded="open"
      @click="open = !open"
    >
      <span class="mb-1 block text-[10px] font-semibold uppercase tracking-[0.14em] text-text-muted">Current server</span>
      <span class="flex items-center gap-2">
        <span :class="statusDot(server.currentServer.status)"></span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm font-semibold">{{ server.currentServer.name }}</span>
          <span class="block truncate text-xs text-text-muted">
            {{ server.currentServer.local ? 'Local' : (server.currentServer.hostname || 'Remote agent') }}
          </span>
        </span>
        <span class="text-xs text-text-muted">⌄</span>
      </span>
    </button>

    <div v-if="open" class="absolute left-0 right-0 z-50 mt-2 max-h-80 overflow-y-auto rounded-xl border border-border-subtle bg-bg-card p-1.5 shadow-2xl" role="listbox">
      <button
        v-for="item in server.servers"
        :key="item.id"
        class="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left hover:bg-indigo-500/10"
        :class="{ 'bg-indigo-500/10 text-indigo-400': item.id === server.currentServerId }"
        role="option"
        :aria-selected="item.id === server.currentServerId"
        @click="choose(item.id)"
      >
        <span :class="statusDot(item.status)"></span>
        <span class="min-w-0 flex-1">
          <span class="block truncate text-sm font-medium">{{ item.name }}</span>
          <span class="block truncate text-xs text-text-muted">{{ item.local ? 'Local' : (item.hostname || item.status) }}</span>
        </span>
      </button>
      <router-link class="mt-1 block rounded-lg border-t border-border-subtle px-3 py-2 text-xs font-medium text-indigo-400 hover:bg-indigo-500/10" to="/servers" @click="open=false">
        Manage servers
      </router-link>
    </div>
  </div>
</template>

<script setup>
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useServerStore } from '@/stores/server'

const server = useServerStore()
const open = ref(false)
const root = ref(null)

function choose(id) {
  server.selectServer(id)
  open.value = false
}
function statusDot(status) {
  const color = status === 'online' ? 'bg-emerald-500' : status === 'warning' ? 'bg-amber-500' : 'bg-slate-500'
  return `h-2 w-2 flex-none rounded-full ${color}`
}
function closeOutside(event) {
  if (!root.value?.contains(event.target)) open.value = false
}
onMounted(() => {
  document.addEventListener('click', closeOutside)
  server.loadServers()
})
onBeforeUnmount(() => document.removeEventListener('click', closeOutside))
</script>
