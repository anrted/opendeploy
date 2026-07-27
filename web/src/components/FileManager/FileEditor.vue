<template>
  <div class="fixed inset-0 z-50 flex items-center justify-center p-4 sm:p-6 bg-black/50 backdrop-blur-sm">
    <div class="bg-gray-800 rounded-xl shadow-2xl w-full max-w-6xl h-[90vh] flex flex-col border border-gray-700 overflow-hidden">
      
      <!-- Header -->
      <div class="flex items-center justify-between px-6 py-4 border-b border-gray-700 bg-gray-800">
        <div class="flex items-center space-x-3">
          <DocumentTextIcon class="w-6 h-6 text-gray-400" />
          <h3 class="text-lg font-medium text-white truncate max-w-lg">
            {{ filename }}
            <span v-if="isDirty" class="text-indigo-400 ml-2 text-sm">* {{ t('fileManager.unsaved') }}</span>
          </h3>
        </div>
        <div class="flex items-center space-x-3">
          <button
            @click="saveFile"
            :disabled="!isDirty || isSaving"
            class="btn-primary flex items-center space-x-2"
            :class="{ 'opacity-50 cursor-not-allowed': !isDirty || isSaving }"
          >
            <ArrowDownTrayIcon class="w-5 h-5" />
            <span>{{ isSaving ? t('fileManager.saving') : t('fileManager.save') }}</span>
          </button>
          <button @click="handleClose" class="text-gray-400 hover:text-gray-300 p-1">
            <XMarkIcon class="w-6 h-6" />
          </button>
        </div>
      </div>

      <!-- Editor -->
      <div class="flex-1 min-h-0 relative bg-[#1e1e1e]">
        <vue-monaco-editor
          v-model:value="content"
          :language="language"
          theme="vs-dark"
          :options="editorOptions"
          @mount="handleMount"
          @change="handleChange"
          class="absolute inset-0"
        />
      </div>

      <!-- Footer / Status Bar -->
      <div class="bg-[#007acc] text-white text-xs px-4 py-1 flex justify-between items-center select-none">
        <div class="flex space-x-4">
          <span>{{ t('fileManager.lines', { count: lineCount }) }}</span>
          <span>{{ fileSizeStr }}</span>
        </div>
        <div class="flex space-x-4">
          <span>{{ language }}</span>
          <span>UTF-8</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import { DocumentTextIcon, XMarkIcon, ArrowDownTrayIcon } from '@heroicons/vue/24/outline'
import VueMonacoEditor from '@guolao/vue-monaco-editor'
import { useConfirmStore } from '@/stores/confirm'
import { formatBytes } from '@/utils/formatters'
import { useI18n } from 'vue-i18n'

const props = defineProps({
  filename: {
    type: String,
    required: true
  },
  initialContent: {
    type: String,
    default: ''
  },
  isSaving: {
    type: Boolean,
    default: false
  }
})

const emit = defineEmits(['close', 'save'])

const confirm = useConfirmStore()
const { t } = useI18n()
const content = ref(props.initialContent)
const originalContent = ref(props.initialContent)
const isDirty = computed(() => content.value !== originalContent.value)

// Monaco instance
let editorRef = null

const handleMount = (editor) => {
  editorRef = editor
  // Add save shortcut (Ctrl+S / Cmd+S)
  editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS, () => {
    if (isDirty.value && !props.isSaving) {
      saveFile()
    }
  })
}

const handleChange = () => {
  // Dirty state is computed based on original content comparison
}

const editorOptions = {
  automaticLayout: true,
  minimap: { enabled: true },
  wordWrap: 'on',
  formatOnPaste: true,
  formatOnType: true,
  folding: true,
  renderWhitespace: 'selection',
  bracketPairColorization: { enabled: true },
  fontSize: 14,
  fontFamily: "'JetBrains Mono', 'Fira Code', 'Roboto Mono', monospace",
  scrollBeyondLastLine: false,
  smoothScrolling: true,
  padding: { top: 16, bottom: 16 }
}

const languageMap = {
  'js': 'javascript', 'jsx': 'javascript',
  'ts': 'typescript', 'tsx': 'typescript',
  'vue': 'vue',
  'html': 'html',
  'css': 'css', 'scss': 'scss',
  'json': 'json',
  'yaml': 'yaml', 'yml': 'yaml',
  'xml': 'xml',
  'php': 'php',
  'py': 'python',
  'go': 'go',
  'sql': 'sql',
  'md': 'markdown',
  'txt': 'plaintext',
  'ini': 'ini', 'env': 'ini',
  'sh': 'shell', 'bash': 'shell',
  'conf': 'nginx', // nginx config is basically mapped to generic or ini if not supported, monaco has 'ini'
}

const language = computed(() => {
  const ext = props.filename.split('.').pop().toLowerCase()
  if (props.filename === 'Dockerfile') return 'dockerfile'
  return languageMap[ext] || 'plaintext'
})

const lineCount = computed(() => {
  return (content.value.match(/\n/g) || []).length + 1
})

const fileSizeStr = computed(() => {
  return formatBytes(new Blob([content.value]).size)
})

const saveFile = () => {
  emit('save', content.value)
}

watch(() => props.isSaving, (newVal, oldVal) => {
  if (oldVal && !newVal) {
    // Save finished, reset dirty state
    originalContent.value = content.value
  }
})

const handleClose = async () => {
  if (isDirty.value) {
    const isConfirmed = await confirm.require({
      title: t('fileManager.unsavedTitle'),
      message: t('fileManager.unsavedMessage'),
      confirmText: t('fileManager.discard'),
      type: 'danger'
    })
    
    if (!isConfirmed) return
  }
  emit('close')
}
</script>
