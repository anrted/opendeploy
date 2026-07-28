<template>
  <div class="fixed inset-0 z-50 flex flex-col bg-[#0f1117]/95 backdrop-blur-md font-sans">
    <div class="flex items-center justify-between px-6 py-4 border-b border-[#2d3748]/50 bg-[#161b27]/80">
      <div class="flex items-center gap-4">
        <h2 class="text-xl font-bold text-white tracking-wide">{{ t('fileManager.title') }}</h2>
        <span class="text-sm font-mono text-indigo-300 bg-indigo-900/30 border border-indigo-500/30 px-3 py-1 rounded-full shadow-inner">{{ site.domain }}</span>
      </div>
      <button @click="$emit('close')" class="p-2 text-slate-400 hover:text-white hover:bg-white/10 rounded-full transition-all">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/></svg>
      </button>
    </div>

    <FileToolbar
      :root-path="site.root_path" :path-parts="pathParts"
      :selected-count="selectedFiles.length" :loading="loading"
      v-model:search="searchQuery"
      @navigate="navigateTo" @archive="batchArchive" @delete="batchDelete"
      @create-folder="createFolder" @upload="uploadFiles" @refresh="refresh"
    />

    <div v-if="error" class="mx-6 mt-6 rounded-xl border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-200 flex items-center shadow-lg shadow-red-500/5">
      <svg class="w-5 h-5 mr-3 text-red-400" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
      {{ error }}
    </div>

    <div class="flex-1 overflow-hidden p-6 flex gap-6">
      <FileTable
        :files="visibleFiles" :loading="loading" :current-path="currentPath"
        :all-selected="isAllSelected" :is-editable="isEditable"
        v-model:selected="selectedFiles"
        @toggle-all="toggleSelectAll" @sort="sortBy" @up="navigateUp"
        @drop="handleDrop" @context="openContextMenu" @edit="openEditor"
        @open-directory="file => navigateTo(getFilePath(file.name))"
      />
    </div>

    <FileContextMenu
      :menu="contextMenu" :editable="isEditable(contextMenu.file?.name)"
      :archive="isArchive(contextMenu.file?.name)"
      @download="downloadFile" @edit="file => { openEditor(file); closeContextMenu() }"
      @rename="renameFile" @copy="copyFile" @move="moveFile"
      @extract="extractFile" @chmod="chmodFile" @delete="deleteFile"
    />

    <FileEditor
      v-if="editingFile && !loadingFile"
      :filename="editingFile.name" :initial-content="editingContent" :is-saving="savingFile"
      @close="closeEditor" @save="saveFile"
    />
  </div>
</template>

<script setup>
import { onMounted, onBeforeUnmount, defineAsyncComponent } from 'vue'
import { useI18n } from 'vue-i18n'
import FileToolbar from './FileToolbar.vue'
import FileTable from './FileTable.vue'
import FileContextMenu from './FileContextMenu.vue'
import { useFileListing } from './useFileListing'
import { useFileEditor } from './useFileEditor'
import { useFileOperations } from './useFileOperations'

const FileEditor = defineAsyncComponent(() => import('./FileEditor.vue'))
const { t } = useI18n()
const props = defineProps({ site: { type: Object, required: true } })
defineEmits(['close'])

const listing = useFileListing(props.site)
const {
  currentPath, loading, error, selectedFiles, searchQuery, pathParts, visibleFiles,
  isAllSelected, refresh, navigateTo, navigateUp, sortBy, toggleSelectAll
} = listing
const operations = useFileOperations(props.site, listing, t)
const {
  contextMenu, getFilePath, uploadFiles, handleDrop, createFolder, openContextMenu,
  closeContextMenu, deleteFile, batchDelete, renameFile, copyFile, moveFile,
  isArchive, batchArchive, extractFile, chmodFile, downloadFile
} = operations
const {
  editingFile, editingContent, loadingFile, savingFile,
  isEditable, openEditor, saveFile, closeEditor
} = useFileEditor(props.site, getFilePath, refresh, error)

onMounted(() => {
  refresh()
  document.addEventListener('click', closeContextMenu)
})
onBeforeUnmount(() => document.removeEventListener('click', closeContextMenu))
</script>

<style scoped>
.fade-enter-active, .fade-leave-active { transition: opacity 0.2s ease, transform 0.2s ease; }
.fade-enter-from, .fade-leave-to { opacity: 0; transform: translateY(-5px); }
</style>
