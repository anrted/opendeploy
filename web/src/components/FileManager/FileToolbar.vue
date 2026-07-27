<template>
  <div class="flex items-center justify-between border-b border-[#2d3748]/50 bg-[#1a202c]/60 px-6 py-4">
    <div class="flex items-center gap-2 font-mono text-sm text-slate-300">
      <button class="rounded p-1 transition-colors hover:bg-white/5 hover:text-indigo-400" @click="$emit('navigate', '/')">
        <svg class="mr-1 inline-block h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6"/></svg>
        {{ rootPath }}
      </button>
      <template v-for="(part, index) in pathParts" :key="index">
        <span class="text-slate-600">/</span>
        <button class="rounded px-2 py-1 transition-colors hover:bg-white/5 hover:text-indigo-400" @click="$emit('navigate', buildPath(index))">{{ part }}</button>
      </template>
    </div>
    <div class="flex items-center gap-3">
      <div v-if="selectedCount" class="mr-4 flex gap-2 border-r border-slate-700 pr-4">
        <button class="btn-secondary" @click="$emit('archive')">{{ $t('fileManager.archive', { count: selectedCount }) }}</button>
        <button class="btn-secondary border-red-500/30 text-red-400 hover:bg-red-500/10 hover:text-red-300" @click="$emit('delete')">{{ $t('fileManager.deleteSelected', { count: selectedCount }) }}</button>
      </div>
      <button class="btn-secondary" @click="$emit('create-folder')">{{ $t('fileManager.newFolder') }}</button>
      <input :value="search" class="input w-44" :placeholder="$t('fileManager.search')" @input="$emit('update:search', $event.target.value.trim())" />
      <label class="btn-primary cursor-pointer bg-indigo-600 hover:bg-indigo-500">
        <input type="file" class="hidden" multiple @change="$emit('upload', $event)" />{{ $t('fileManager.upload') }}
      </label>
      <button class="btn-secondary p-2" :title="$t('common.refresh')" @click="$emit('refresh')">
        <svg class="h-4 w-4" :class="{ 'animate-spin': loading }" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/></svg>
      </button>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  rootPath: { type: String, required: true },
  pathParts: { type: Array, required: true },
  selectedCount: { type: Number, required: true },
  loading: Boolean,
  search: { type: String, default: '' }
})
defineEmits(['navigate', 'archive', 'delete', 'create-folder', 'upload', 'refresh', 'update:search'])
function buildPath(index) {
  return '/' + props.pathParts.slice(0, index + 1).join('/')
}
</script>
