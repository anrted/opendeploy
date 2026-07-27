<template>
  <div class="flex-1 bg-[#1e293b]/50 border border-slate-700/50 rounded-2xl shadow-xl overflow-hidden backdrop-blur-sm flex flex-col relative"
       @drop.prevent="$emit('drop', $event)" @dragover.prevent>
    <div class="flex-1 overflow-y-auto min-h-0">
      <table class="w-full text-left border-collapse relative">
        <thead class="sticky top-0 z-10">
          <tr class="bg-slate-800/90 backdrop-blur-md text-slate-300 text-sm font-semibold border-b border-slate-700/50">
            <th class="p-4 w-12 text-center"><input type="checkbox" class="rounded border-slate-600 bg-slate-700 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-slate-800" :checked="allSelected" @change="$emit('toggle-all', $event)" /></th>
            <th class="p-4 cursor-pointer hover:text-white" @click="$emit('sort', 'name')">{{ $t('fileManager.name') }}</th>
            <th class="p-4 w-32 cursor-pointer hover:text-white" @click="$emit('sort', 'size')">{{ $t('fileManager.size') }}</th>
            <th class="p-4 w-48 cursor-pointer hover:text-white" @click="$emit('sort', 'mod_time')">{{ $t('fileManager.modified') }}</th>
            <th class="p-4 w-32 text-center">{{ $t('fileManager.owner') }}</th>
            <th class="p-4 w-24 text-center">{{ $t('fileManager.permissionsShort') }}</th>
          </tr>
        </thead>
        <tbody class="text-slate-300 text-sm">
          <tr v-if="loading"><td colspan="6" class="text-center py-12 text-slate-500">{{ $t('fileManager.loading') }}</td></tr>
          <tr v-else-if="currentPath !== '/'" class="cursor-pointer hover:bg-slate-700/30 border-b border-slate-700/30" @click="$emit('up')">
            <td class="p-4 text-center text-slate-500">
              <svg class="w-5 h-5 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M3 10h10a8 8 0 018 8v2M3 10l6 6m-6-6l6-6"/></svg>
            </td><td colspan="5" class="p-4 font-medium text-indigo-300">..</td>
          </tr>
          <tr v-if="!loading && !files.length && currentPath === '/'">
            <td colspan="6" class="text-center py-12 text-slate-500">
              <div class="flex flex-col items-center">
                <svg class="w-16 h-16 text-slate-700 mb-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z"/></svg>
                {{ $t('fileManager.empty') }}
              </div>
            </td>
          </tr>
          <tr v-for="file in files" :key="file.name"
              class="hover:bg-slate-700/30 border-b border-slate-700/30"
              :class="{ 'bg-indigo-900/20': selected.includes(file.name) }"
              @contextmenu.prevent="$emit('context', $event, file)">
            <td class="p-4 text-center">
              <input type="checkbox" class="rounded border-slate-600 bg-slate-700 text-indigo-500 focus:ring-indigo-500 focus:ring-offset-slate-800" :value="file.name" :checked="selected.includes(file.name)"
                     @change="toggle(file.name, $event.target.checked)" />
            </td>
            <td class="p-4">
              <div class="flex items-center gap-3">
                <svg v-if="file.is_dir" class="w-6 h-6 text-blue-400 drop-shadow-sm" fill="currentColor" viewBox="0 0 20 20"><path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z"/></svg>
                <svg v-else class="w-6 h-6 text-slate-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z"/></svg>
                <button v-if="file.is_dir" class="text-white hover:text-indigo-400 font-medium text-left truncate max-w-[200px] sm:max-w-xs transition-colors" @click="$emit('open-directory', file)">
                  {{ file.name }}
                </button>
                <button v-else-if="isEditable(file.name)" class="text-slate-200 hover:text-indigo-300 truncate max-w-[200px] sm:max-w-xs text-left transition-colors" @click="$emit('edit', file)">
                  {{ file.name }}
                </button>
                <span v-else class="text-slate-200 truncate max-w-[200px] sm:max-w-xs">{{ file.name }}</span>
              </div>
            </td>
            <td class="p-4 text-slate-400 font-mono text-xs">{{ file.is_dir ? '—' : formatBytes(file.size) }}</td>
            <td class="p-4 text-slate-400">{{ new Date(file.mod_time * 1000).toLocaleString() }}</td>
            <td class="p-4 text-center text-slate-400 font-mono text-xs">{{ file.owner }}:{{ file.group }}</td>
            <td class="p-4 text-center text-slate-400 font-mono text-xs">{{ formatPerms(file.mode) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup>
const props = defineProps({
  files: { type: Array, required: true },
  selected: { type: Array, required: true },
  loading: Boolean,
  currentPath: { type: String, required: true },
  allSelected: Boolean,
  isEditable: { type: Function, required: true }
})

const emit = defineEmits(['update:selected', 'toggle-all', 'sort', 'up', 'drop', 'context', 'open-directory', 'edit'])

function toggle(name, checked) {
  const next = checked ? [...props.selected, name] : props.selected.filter(item => item !== name)
  emit('update:selected', next)
}

function formatBytes(bytes) {
  if (bytes === 0) return '0 B'
  const k = 1024
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(k))
  return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i]
}

function formatPerms(mode) {
  if (mode === undefined) return ''
  return (mode & 0o777).toString(8).padStart(3, '0')
}
</script>
