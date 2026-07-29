<template>
  <div>
    <div class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
      <div><h1 class="page-title">Servers</h1><p class="page-subtitle">Infrastructure · Remote OpenDeploy Agents</p></div>
      <button class="btn-primary" @click="openWizard">+ Add Server</button>
    </div>
    <div class="card mb-5 grid gap-3 md:grid-cols-4">
      <input v-model="query" class="input md:col-span-2" placeholder="Search servers…" @input="debouncedLoad">
      <select v-model="status" class="input" @change="load"><option value="">All statuses</option><option v-for="value in ['online','warning','offline','pending']" :key="value">{{ value }}</option></select>
      <select v-model="sort" class="input" @change="load"><option value="created_at">Newest</option><option value="name">Name</option><option value="status">Status</option><option value="last_heartbeat">Heartbeat</option></select>
    </div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-red-300">{{ errorMessage }}</div>
    <div class="table-wrapper"><table class="table">
      <thead><tr><th>Name</th><th>Status</th><th>OS</th><th>Agent</th><th>Last heartbeat</th><th></th></tr></thead>
      <tbody>
        <tr v-if="loading"><td colspan="6" class="py-10 text-center">Loading…</td></tr>
        <tr v-else-if="!items.length"><td colspan="6" class="py-12 text-center text-text-muted">No servers registered yet.</td></tr>
        <tr v-for="server in items" :key="server.id">
          <td><button class="text-left font-semibold text-indigo-300" @click="select(server)">{{ server.name }}</button><div class="text-xs text-text-muted">{{ server.hostname || server.id }}<span v-if="server.local"> · Local</span></div></td>
          <td><span :class="badge(server.status)">{{ server.status }}</span></td>
          <td>{{ [server.distribution || server.os, server.os_version, server.architecture].filter(Boolean).join(' · ') || '—' }}</td>
          <td>{{ server.agent_version || '—' }}</td><td>{{ formatDate(server.last_heartbeat) }}</td>
          <td><div class="flex justify-end gap-2"><button v-if="!server.local && server.status!=='pending'" class="btn-primary text-xs" @click="openAgentUpdate(server)">Update</button><button class="btn-secondary text-xs" @click="select(server)">Switch</button></div></td>
        </tr>
      </tbody>
    </table></div>
    <div class="mt-4 flex justify-between text-sm text-text-muted"><span>{{ total }} total</span><div class="flex gap-2"><button class="btn-secondary" :disabled="offset===0" @click="page(-1)">Previous</button><button class="btn-secondary" :disabled="offset+limit>=total" @click="page(1)">Next</button></div></div>

    <div v-if="showWizard" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div class="card w-full max-w-2xl">
        <div class="mb-5 flex justify-between"><h2 class="text-xl font-semibold">Add Server · Step {{ wizardStep }}/2</h2><button @click="showWizard=false">✕</button></div>
        <form v-if="wizardStep===1" class="grid gap-4 sm:grid-cols-2" @submit.prevent="createEnrollment">
          <div><label class="label">Server Name</label><input v-model="form.name" class="input" required maxlength="80"></div>
          <div><label class="label">Operating System</label><select v-model="form.operating_system" class="input"><option>Ubuntu</option><option>Debian</option></select></div>
          <div class="sm:col-span-2"><label class="label">Description</label><textarea v-model="form.description" class="input" rows="3"></textarea></div>
          <div><label class="label">Tags</label><input v-model="tagsText" class="input" placeholder="production, eu"></div>
          <div><label class="label">Update Channel</label><select v-model="form.update_channel" class="input"><option value="stable">Stable</option><option value="beta">Beta</option><option value="nightly">Nightly</option></select></div>
          <div class="sm:col-span-2 flex justify-end gap-2"><button type="button" class="btn-secondary" @click="showWizard=false">Cancel</button><button class="btn-primary" :disabled="creating">Generate enrollment</button></div>
        </form>
        <div v-else class="space-y-4">
          <div class="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">Token is shown once and expires {{ formatDate(enrollment.expires_at) }}.</div>
          <div><label class="label">Registration Code</label><div class="rounded-lg bg-black/30 p-3 font-mono text-lg tracking-wider">{{ enrollment.registration_code }}</div></div>
          <div><label class="label">Installation Command</label><pre class="overflow-x-auto whitespace-pre-wrap rounded-lg bg-[#090d16] p-4 text-xs text-green-300">{{ enrollment.installation_command }}</pre></div>
          <div class="flex justify-end gap-2"><button class="btn-secondary" @click="copyCommand">Copy</button><button class="btn-primary" @click="finishWizard">Done</button></div>
        </div>
      </div>
    </div>
    <div v-if="updateServer" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div class="card w-full max-w-2xl">
        <div class="mb-5 flex justify-between"><h2 class="text-xl font-semibold">Update agent · {{ updateServer.name }}</h2><button @click="updateServer=null">✕</button></div>
        <div class="space-y-4">
          <div class="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
            Agent {{ updateServer.agent_version || 'unknown' }} cannot update itself remotely. Run this command once on the server. Its ID, certificate and configuration will be preserved.
          </div>
          <div><label class="label">Update Command</label><pre class="overflow-x-auto whitespace-pre-wrap rounded-lg bg-[#090d16] p-4 text-xs text-green-300">{{ agentUpdateCommand }}</pre></div>
          <div class="flex justify-end gap-2"><button class="btn-secondary" @click="copyUpdateCommand">Copy</button><button class="btn-primary" @click="updateServer=null">Done</button></div>
        </div>
      </div>
    </div>
  </div>
</template>
<script setup>
import { onMounted, reactive, ref } from 'vue'
import api, { apiErrorMessage } from '@/api/client'
import { useRouter } from 'vue-router'
import { useServerStore } from '@/stores/server'
const router=useRouter(),serverStore=useServerStore()
const items=ref([]),total=ref(0),loading=ref(false),errorMessage=ref(''),query=ref(''),status=ref(''),sort=ref('created_at'),offset=ref(0),limit=25
const showWizard=ref(false),wizardStep=ref(1),creating=ref(false),enrollment=ref({}),tagsText=ref('')
const updateServer=ref(null)
const agentUpdateCommand='curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install-agent.sh | sudo bash -s -- --update'
const form=reactive({name:'',description:'',operating_system:'Ubuntu',update_channel:'stable'})
let debounce
function debouncedLoad(){clearTimeout(debounce);debounce=setTimeout(()=>{offset.value=0;load()},250)}
async function load(){loading.value=true;try{const {data}=await api.get('/servers',{params:{query:query.value,status:status.value,sort:sort.value,limit,offset:offset.value},serverContext:false});items.value=data.items||[];total.value=data.total||0;errorMessage.value=''}catch(e){errorMessage.value=apiErrorMessage(e)}finally{loading.value=false}}
function select(server){serverStore.selectServer(server.id);router.push('/')}
function page(d){offset.value=Math.max(0,offset.value+d*limit);load()} function badge(v){return v==='online'?'badge-success':v==='warning'?'badge-warning':v==='offline'?'badge-danger':'badge-muted'} function formatDate(v){return v?new Date(v).toLocaleString():'Never'}
function openWizard(){wizardStep.value=1;showWizard.value=true}
async function createEnrollment(){creating.value=true;try{const {data}=await api.post('/servers',{...form,tags:tagsText.value.split(',').map(v=>v.trim()).filter(Boolean)});enrollment.value=data;wizardStep.value=2;await load()}catch(e){errorMessage.value=apiErrorMessage(e)}finally{creating.value=false}}
async function copyCommand(){await navigator.clipboard.writeText(enrollment.value.installation_command)} function finishWizard(){showWizard.value=false;load()}
function openAgentUpdate(server){updateServer.value=server}
async function copyUpdateCommand(){await navigator.clipboard.writeText(agentUpdateCommand)}
onMounted(load)
</script>
