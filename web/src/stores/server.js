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
  const capabilitiesLoading = ref(false)
  const capabilitiesError = ref('')
  const currentServerId = ref(localStorage.getItem(STORAGE_KEY) || LOCAL_SERVER_ID)
  let capabilitiesRequest = 0

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

  async function selectServer(serverId) {
    if (!serverId || serverId === currentServerId.value) return
    currentServerId.value = serverId
    localStorage.setItem(STORAGE_KEY, serverId)
    capabilities.value = []
    capabilitiesError.value = ''
    await loadCapabilities()
  }

  async function loadCapabilities() {
    const serverId = currentServerId.value
    const requestId = ++capabilitiesRequest
    capabilitiesLoading.value = true
    capabilitiesError.value = ''
    try {
      const { data } = await api.get(`/servers/${serverId}/capabilities`, { serverContext: false })
      if (requestId !== capabilitiesRequest || serverId !== currentServerId.value) return
      capabilities.value = (data.items || []).filter((item) => item.available).map((item) => item.name)
      if (serverId !== LOCAL_SERVER_ID && capabilities.value.length === 0) {
        capabilitiesError.value = 'The selected Agent is offline or has not completed its capability handshake.'
      }
    } catch (error) {
      if (requestId !== capabilitiesRequest || serverId !== currentServerId.value) return
      capabilities.value = []
      capabilitiesError.value = error?.response?.data?.error?.message || 'Failed to load capabilities from the selected Agent.'
    } finally {
      if (requestId === capabilitiesRequest) capabilitiesLoading.value = false
    }
  }

  function supports(name) {
    return currentServerId.value === LOCAL_SERVER_ID || capabilities.value.includes(name)
  }

  return {
    servers, loading, capabilities, capabilitiesLoading, capabilitiesError,
    currentServerId, currentServer, loadServers, loadCapabilities, selectServer, supports,
  }
})
