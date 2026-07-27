import { ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const editableExtensions = ['.json', '.php', '.txt', '.js', '.vue', '.html', '.css', '.md', '.env', '.sh', '.yml', '.yaml', '.xml']

export function useFileEditor(site, getFilePath, refresh, error) {
  const editingFile = ref(null)
  const editingContent = ref('')
  const loadingFile = ref(false)
  const savingFile = ref(false)

  function isEditable(filename) {
    if (!filename) return false
    const ext = filename.slice((Math.max(0, filename.lastIndexOf('.')) || Infinity)).toLowerCase()
    return editableExtensions.includes(ext) || filename.startsWith('.')
  }

  async function openEditor(file) {
    if (!isEditable(file.name)) return
    editingFile.value = file
    loadingFile.value = true
    editingContent.value = ''
    error.value = ''
    try {
      const { data } = await api.get(`/sites/${site.id}/file`, {
        params: { path: getFilePath(file.name) },
        transformResponse: [data => data]
      })
      editingContent.value = data
    } catch (e) {
      error.value = apiErrorMessage(e, `Failed to load ${file.name}`)
      editingFile.value = null
    } finally {
      loadingFile.value = false
    }
  }

  async function saveFile(newContent) {
    if (!editingFile.value) return
    savingFile.value = true
    error.value = ''
    try {
      const formData = new FormData()
      formData.append('file', new File([new Blob([newContent], { type: 'text/plain' })], editingFile.value.name, { type: 'text/plain' }))
      await api.post(`/sites/${site.id}/file`, formData, { params: { path: getFilePath(editingFile.value.name) } })
      editingContent.value = newContent
      refresh()
    } catch (e) {
      error.value = apiErrorMessage(e, `Failed to save ${editingFile.value.name}`)
    } finally {
      savingFile.value = false
    }
  }

  function closeEditor() {
    editingFile.value = null
    editingContent.value = ''
  }

  return { editingFile, editingContent, loadingFile, savingFile, isEditable, openEditor, saveFile, closeEditor }
}
