import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import api from '@/api/client'

export const LOCAL_SERVER_ID = 'local'
const STORAGE_KEY = 'opendeploy.currentServerId'

export const useServerStore = defineStore('server', () => {
  const servers = ref([])
  const loading = ref(false)
  const loaded = ref(false)
  const capabilities = ref([])
  const currentServerId = ref(localStorage.getItem(STORAGE_KEY) || LOCAL_SERVER_ID)

  const currentServer = computed(() =>
    servers.value.find((server) => server.id === currentServerId.value) || {
      id: LOCAL_SERVER_ID,
      name: 'Localhost',
      hostname: window.location.hostname,
      status: 'online',
      local: true,
    },
  )

  async function loadServers(force = false) {
    if ((loaded.value && !force) || loading.value) return
    loading.value = true
    try {
      const { data } = await api.get('/servers', {
        params: { limit: 100, offset: 0, sort: 'name' },
        serverContext: false,
      })
      servers.value = data.items || []
      if (!servers.value.some((server) => server.id === currentServerId.value)) {
        selectServer(LOCAL_SERVER_ID)
      }
      loaded.value = true
      await loadCapabilities()
    } finally {
      loading.value = false
    }
  }

  function selectServer(serverId) {
    if (!serverId || serverId === currentServerId.value) return
    currentServerId.value = serverId
    localStorage.setItem(STORAGE_KEY, serverId)
    loadCapabilities()
  }

  async function loadCapabilities() {
    try {
      const { data } = await api.get(`/servers/${currentServerId.value}/capabilities`, { serverContext: false })
      capabilities.value = (data.items || []).filter((item) => item.available).map((item) => item.name)
    } catch {
      capabilities.value = []
    }
  }

  function supports(name) {
    return currentServerId.value === LOCAL_SERVER_ID || capabilities.value.includes(name)
  }

  return { servers, loading, capabilities, currentServerId, currentServer, loadServers, loadCapabilities, selectServer, supports }
})
