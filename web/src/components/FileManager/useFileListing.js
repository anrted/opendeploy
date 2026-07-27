import { computed, ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useI18n } from 'vue-i18n'

export function useFileListing(site) {
  const { t } = useI18n()
  const currentPath = ref('/')
  const files = ref([])
  const loading = ref(true)
  const error = ref('')
  const selectedFiles = ref([])
  const sortConfig = ref({ key: 'name', dir: 'asc' })
  const searchQuery = ref('')

  const pathParts = computed(() => currentPath.value.split('/').filter(Boolean))
  const visibleFiles = computed(() => {
    const { key, dir } = sortConfig.value
    const modifier = dir === 'asc' ? 1 : -1
    const sorted = [...files.value].sort((a, b) => {
      if (a.is_dir && !b.is_dir) return -1
      if (!a.is_dir && b.is_dir) return 1
      if (a[key] < b[key]) return -1 * modifier
      if (a[key] > b[key]) return modifier
      return 0
    })
    const query = searchQuery.value.toLowerCase()
    return query ? sorted.filter(file => file.name.toLowerCase().includes(query)) : sorted
  })
  const isAllSelected = computed(() => files.value.length > 0 && selectedFiles.value.length === files.value.length)

  async function refresh() {
    loading.value = true
    error.value = ''
    selectedFiles.value = []
    try {
      const { data } = await api.get(`/sites/${site.id}/files`, { params: { path: currentPath.value } })
      files.value = data || []
    } catch (e) {
      error.value = apiErrorMessage(e, t('fileManager.loadFailed'))
      files.value = []
    } finally {
      loading.value = false
    }
  }

  function navigateTo(path) {
    currentPath.value = path
    refresh()
  }

  function navigateUp() {
    const parts = [...pathParts.value]
    parts.pop()
    navigateTo('/' + parts.join('/'))
  }

  function sortBy(key) {
    if (sortConfig.value.key === key) {
      sortConfig.value.dir = sortConfig.value.dir === 'asc' ? 'desc' : 'asc'
    } else {
      sortConfig.value = { key, dir: 'asc' }
    }
  }

  function toggleSelectAll(event) {
    selectedFiles.value = event.target.checked ? files.value.map(file => file.name) : []
  }

  return {
    currentPath, files, loading, error, selectedFiles, searchQuery,
    pathParts, visibleFiles, isAllSelected,
    refresh, navigateTo, navigateUp, sortBy, toggleSelectAll
  }
}
