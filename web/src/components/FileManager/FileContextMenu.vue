<template>
  <div v-if="menu.show"
       class="fixed bg-[#1e293b] border border-slate-600 rounded-lg shadow-2xl py-2 z-[60] text-sm w-48 backdrop-blur-md"
       :style="{ top: menu.y + 'px', left: menu.x + 'px' }"
       @click.stop>
    <div class="px-3 py-1 text-xs text-slate-500 uppercase tracking-wider font-semibold border-b border-slate-700/50 mb-1 truncate">
      {{ menu.file?.name }}
    </div>
    <button v-if="!menu.file?.is_dir" @click="$emit('download', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
      Download
    </button>
    <button v-if="!menu.file?.is_dir && editable" @click="$emit('edit', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15.232 5.232l3.536 3.536m-2.036-5.036a2.5 2.5 0 113.536 3.536L6.5 21.036H3v-3.572L16.732 3.732z"/></svg>
      Edit
    </button>
    <button @click="$emit('rename', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z"/></svg>
      Rename
    </button>
    <button v-if="!menu.file?.is_dir" @click="$emit('copy', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">Copy to…</button>
    <button @click="$emit('move', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">Move to…</button>
    <button v-if="!menu.file?.is_dir && archive" @click="$emit('extract', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">Extract here</button>
    <button @click="$emit('chmod', menu.file)" class="w-full text-left px-4 py-2 hover:bg-slate-700 hover:text-white transition-colors flex items-center">
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
      Permissions
    </button>
    <div class="border-t border-slate-700/50 my-1"></div>
    <button @click="$emit('delete', menu.file)" class="w-full text-left px-4 py-2 transition-colors flex items-center text-red-400 hover:bg-red-500/10 hover:text-red-300">
      <svg class="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
      Delete
    </button>
  </div>
</template>

<script setup>
defineProps({
  menu: { type: Object, required: true },
  editable: { type: Boolean, default: false },
  archive: { type: Boolean, default: false }
})

defineEmits(['download', 'edit', 'rename', 'copy', 'move', 'extract', 'chmod', 'delete'])
</script>
