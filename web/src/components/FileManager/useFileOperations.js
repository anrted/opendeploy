import { ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useConfirmStore } from '@/stores/confirm'

// Owns mutating file operations; the component remains presentation-only.
export function useFileOperations(site, listing, t) {
  const confirm = useConfirmStore()
  const { currentPath, selectedFiles, error, refresh } = listing
  const contextMenu = ref({ show: false, x: 0, y: 0, file: null })

  const getFilePath = name => currentPath.value === '/' ? '/' + name : currentPath.value + '/' + name
  const request = async (payload, fallback) => {
    try {
      await api.post(`/sites/${site.id}/files/batch`, payload)
      refresh()
    } catch (exception) {
      error.value = apiErrorMessage(exception, fallback)
    }
  }

  async function processUploads(fileList) {
    error.value = ''
    for (const file of fileList) {
      const formData = new FormData()
      formData.append('file', file)
      try {
        await api.post(`/sites/${site.id}/file`, formData, { params: { path: getFilePath(file.name) } })
      } catch (exception) {
        error.value = apiErrorMessage(exception, t('fileManager.uploadFailed', { name: file.name }))
        break
      }
    }
    refresh()
  }
  async function uploadFiles(event) {
    if (event.target.files.length) await processUploads(event.target.files)
    event.target.value = ''
  }
  async function handleDrop(event) {
    if (event.dataTransfer.files.length) await processUploads(event.dataTransfer.files)
  }
  async function createFolder() {
    const name = prompt(t('fileManager.folderPrompt'))
    if (!name?.trim()) return
    try {
      await api.post(`/sites/${site.id}/directory`, { path: getFilePath(name.trim()) })
      refresh()
    } catch (exception) {
      error.value = apiErrorMessage(exception, t('fileManager.createFailed'))
    }
  }
  function openContextMenu(event, file) {
    contextMenu.value = { show: true, x: event.clientX, y: event.clientY, file }
  }
  function closeContextMenu() { contextMenu.value.show = false }
  async function deleteFile(file) {
    closeContextMenu()
    const accepted = await confirm.require({
      title: t('fileManager.deleteTitle'), message: t('fileManager.deleteMessage', { name: file.name }),
      confirmText: t('common.delete'), type: 'danger'
    })
    if (accepted) await request({ action: 'delete', paths: [getFilePath(file.name)] }, t('fileManager.deleteFailed'))
  }
  async function batchDelete() {
    if (!selectedFiles.value.length) return
    const accepted = await confirm.require({
      title: t('fileManager.batchDeleteTitle'),
      message: t('fileManager.batchDeleteMessage', { count: selectedFiles.value.length }),
      confirmText: t('common.delete'), type: 'danger'
    })
    if (accepted) {
      await request({ action: 'delete', paths: selectedFiles.value.map(getFilePath) }, t('fileManager.batchDeleteFailed'))
    }
  }
  async function renameFile(file) {
    closeContextMenu()
    const name = prompt(t('fileManager.renamePrompt'), file.name)
    if (name && name !== file.name) {
      await request({ action: 'move', paths: [getFilePath(file.name)], dst_path: getFilePath(name) }, t('fileManager.renameFailed'))
    }
  }
  async function copyFile(file) {
    closeContextMenu()
    const destination = prompt(t('fileManager.copyPrompt'), getFilePath(`${file.name}.copy`))
    if (destination) await request({ action: 'copy', paths: [getFilePath(file.name)], dst_path: destination }, t('fileManager.copyFailed'))
  }
  async function moveFile(file) {
    closeContextMenu()
    const source = getFilePath(file.name)
    const destination = prompt(t('fileManager.movePrompt'), source)
    if (destination && destination !== source) {
      await request({ action: 'move', paths: [source], dst_path: destination }, t('fileManager.moveFailed'))
    }
  }
  const isArchive = (name = '') => /\.(zip|tar|tar\.gz|tgz)$/i.test(name)
  async function batchArchive() {
    if (!selectedFiles.value.length) return
    const name = prompt(t('fileManager.archivePrompt'), 'archive.zip')
    if (name) {
      await request({
        action: 'archive', paths: selectedFiles.value.map(getFilePath), dst_path: getFilePath(name)
      }, t('fileManager.archiveFailed'))
    }
  }
  async function extractFile(file) {
    closeContextMenu()
    const accepted = await confirm.require({
      title: t('fileManager.extractTitle'), message: t('fileManager.extractMessage', { name: file.name }),
      confirmText: t('fileManager.extract'), type: 'warning'
    })
    if (accepted) {
      await request({
        action: 'extract', paths: [getFilePath(file.name)], dst_path: currentPath.value
      }, t('fileManager.extractFailed'))
    }
  }
  async function chmodFile(file) {
    closeContextMenu()
    const current = file.mode === undefined ? '' : (file.mode & 0o777).toString(8).padStart(3, '0')
    const permissions = prompt(t('fileManager.permissionsPrompt'), current)
    if (permissions) {
      await request({
        action: 'chmod', paths: [getFilePath(file.name)], mode: parseInt(permissions, 8)
      }, t('fileManager.chmodFailed'))
    }
  }
  function downloadFile(file) {
    closeContextMenu()
    window.open(`${api.defaults.baseURL}/sites/${site.id}/file?path=${encodeURIComponent(getFilePath(file.name))}`, '_blank')
  }

  return {
    contextMenu, getFilePath, uploadFiles, handleDrop, createFolder, openContextMenu,
    closeContextMenu, deleteFile, batchDelete, renameFile, copyFile, moveFile,
    isArchive, batchArchive, extractFile, chmodFile, downloadFile
  }
}
