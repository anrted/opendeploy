<template>
  <div v-if="server">
    <div class="mb-6 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
      <div><router-link to="/servers" class="text-sm text-indigo-300">← Servers</router-link><h1 class="page-title mt-2">{{ server.name }}</h1><p class="page-subtitle">{{ server.hostname || server.id }}</p></div>
      <div class="flex flex-wrap gap-2"><button v-if="server.status==='pending'" class="btn-primary" @click="reissueEnrollment">Generate new token</button><template v-else><button class="btn-secondary" @click="run('refresh_information')">Refresh</button><button class="btn-secondary" @click="run('health_check')">Health Check</button><button class="btn-secondary" @click="run(server.maintenance?'maintenance_off':'maintenance_on')">Maintenance</button><button class="btn-primary" @click="showAgentUpdate=true">Update Agent</button></template><button class="btn-danger" @click="remove">Delete</button></div>
    </div>
    <div v-if="errorMessage" class="mb-4 rounded-lg border border-red-500/30 bg-red-500/10 p-3 text-red-300">{{ errorMessage }}</div>
    <div class="mb-6 grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
      <div class="card"><div class="text-xs uppercase text-text-muted">Status</div><div class="mt-2"><span :class="badge(server.status)">{{ server.status }}</span></div></div>
      <div class="card"><div class="text-xs uppercase text-text-muted">Health</div><div class="mt-2 text-xl font-semibold">{{ server.health_status }}</div></div>
      <div class="card"><div class="text-xs uppercase text-text-muted">Latency</div><div class="mt-2 text-xl font-semibold">{{ server.latency_ms }} ms</div></div>
      <div class="card"><div class="text-xs uppercase text-text-muted">Uptime</div><div class="mt-2 text-xl font-semibold">{{ duration(server.uptime) }}</div></div>
    </div>
    <div v-if="diagnostics" class="mb-6 card">
      <div class="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div><h2 class="text-lg font-semibold">Remote Management Diagnostics</h2><p class="mt-1 text-sm text-text-muted">Heartbeat and command transport are independent health signals.</p></div>
        <span :class="diagnostics.control_plane_connected?'badge-success':'badge-warning'">{{ diagnostics.control_plane_connected?'Control Plane ready':'Control Plane disconnected' }}</span>
      </div>
      <div class="grid gap-4 sm:grid-cols-2 xl:grid-cols-4">
        <Diagnostic label="Legacy heartbeat" :value="server.status==='online'?'Online':'Offline'"/>
        <Diagnostic label="Connection mode" :value="server.connection_mode||'unknown'"/>
        <Diagnostic label="Agent / API" :value="`${diagnostics.agent_version||server.agent_version||'unknown'} / ${diagnostics.api_version||'unknown'}`"/>
        <Diagnostic label="Last Control Plane message" :value="dateTime(diagnostics.last_seen)"/>
        <Diagnostic label="mTLS session" :value="diagnostics.connection_id||'Not established'"/>
        <Diagnostic label="Capabilities" :value="`${diagnostics.items?.length||0} advertised`"/>
        <Diagnostic label="Connected at" :value="dateTime(diagnostics.connected_at)"/>
        <Diagnostic label="TLS identity" :value="diagnostics.control_plane_connected?'Verified Agent certificate':'Not verified'"/>
      </div>
      <div v-if="diagnostics.reason" class="mt-4 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
        <strong>{{ diagnostics.reason }}</strong><div class="mt-1">{{ diagnostics.recommendation }}</div>
        <pre class="mt-3 overflow-x-auto whitespace-pre-wrap rounded bg-black/30 p-3 text-xs text-green-300">{{ agentUpdateCommand }}</pre>
      </div>
    </div>
    <div class="grid gap-6 xl:grid-cols-3">
      <div class="card xl:col-span-2"><h2 class="mb-4 text-lg font-semibold">System Information</h2><dl class="grid gap-4 sm:grid-cols-2"><div v-for="[label,value] in facts" :key="label"><dt class="text-xs uppercase text-text-muted">{{ label }}</dt><dd class="mt-1 break-all">{{ value || '—' }}</dd></div></dl></div>
      <div class="card"><h2 class="mb-4 text-lg font-semibold">Latest Load</h2><template v-if="heartbeats[0]"><Metric label="CPU" :value="heartbeats[0].cpu_usage"/><Metric label="Memory" :value="heartbeats[0].memory_usage"/><Metric label="Disk" :value="heartbeats[0].disk_usage"/></template><p v-else class="text-text-muted">No heartbeat data.</p></div>
    </div>
    <div class="mt-6 card"><div class="mb-4 flex gap-2"><button v-for="name in ['tasks','events','heartbeats']" :key="name" :class="tab===name?'btn-primary':'btn-secondary'" @click="tab=name">{{ name }}</button></div><div class="space-y-2"><div v-for="item in activeItems" :key="item.id" class="rounded-lg border border-border-subtle p-3 text-sm"><div class="flex justify-between gap-4"><strong>{{ item.action || item.type || item.state }}</strong><span class="text-text-muted">{{ new Date(item.created_at).toLocaleString() }}</span></div><div class="mt-1 text-text-muted">{{ item.message || item.output || `${item.cpu_usage?.toFixed?.(1)||0}% CPU · ${item.memory_usage?.toFixed?.(1)||0}% RAM` }}</div></div></div></div>
  </div><div v-else class="py-20 text-center">{{ loading?'Loading…':'Server not found' }}</div>
    <div v-if="enrollment" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div class="card w-full max-w-2xl">
        <div class="mb-5 flex justify-between"><h2 class="text-xl font-semibold">New enrollment token</h2><button @click="enrollment=null">✕</button></div>
        <div class="space-y-4">
          <div class="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">Previous tokens are no longer valid. This token is shown once and expires {{ new Date(enrollment.expires_at).toLocaleString() }}.</div>
          <div><label class="label">Registration Code</label><div class="rounded-lg bg-black/30 p-3 font-mono text-lg tracking-wider">{{ enrollment.registration_code }}</div></div>
          <div><label class="label">Installation Command</label><pre class="overflow-x-auto whitespace-pre-wrap rounded-lg bg-[#090d16] p-4 text-xs text-green-300">{{ enrollment.installation_command }}</pre></div>
          <div class="flex justify-end gap-2"><button class="btn-secondary" @click="copyCommand">Copy</button><button class="btn-primary" @click="enrollment=null">Done</button></div>
        </div>
      </div>
    </div>
    <div v-if="showAgentUpdate" class="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4">
      <div class="card w-full max-w-2xl">
        <div class="mb-5 flex justify-between"><h2 class="text-xl font-semibold">Update agent</h2><button @click="showAgentUpdate=false">✕</button></div>
        <div class="space-y-4">
          <div class="rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-sm text-amber-200">
            Agent {{ server.agent_version || 'unknown' }} cannot update itself remotely. Run this command once on {{ server.hostname || server.name }}. The existing server ID, certificate and configuration will be preserved.
          </div>
          <div><label class="label">Update Command</label><pre class="overflow-x-auto whitespace-pre-wrap rounded-lg bg-[#090d16] p-4 text-xs text-green-300">{{ agentUpdateCommand }}</pre></div>
          <div class="flex justify-end gap-2"><button class="btn-secondary" @click="copyUpdateCommand">Copy</button><button class="btn-primary" @click="showAgentUpdate=false">Done</button></div>
        </div>
      </div>
    </div>
</template>
<script setup>
import { computed, defineComponent, h, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import api, { apiErrorMessage } from '@/api/client'
const route=useRoute(),router=useRouter(),server=ref(null),diagnostics=ref(null),loading=ref(true),errorMessage=ref(''),tasks=ref([]),events=ref([]),heartbeats=ref([]),tab=ref('tasks'),enrollment=ref(null),showAgentUpdate=ref(false)
const agentUpdateCommand='curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install-agent.sh | sudo bash -s -- --update'
const Metric=defineComponent({props:{label:String,value:Number},setup(p){return()=>h('div',{class:'mb-4'},[h('div',{class:'mb-1 flex justify-between text-sm'},[h('span',p.label),h('span',`${(p.value||0).toFixed(1)}%`)]),h('div',{class:'h-2 rounded bg-slate-700'},[h('div',{class:'h-2 rounded bg-indigo-500',style:{width:`${Math.min(100,p.value||0)}%`}})])])}})
const Diagnostic=defineComponent({props:{label:String,value:String},setup(p){return()=>h('div',{},[h('div',{class:'text-xs uppercase text-text-muted'},p.label),h('div',{class:'mt-1 break-all text-sm'},p.value||'—')])}})
const facts=computed(()=>[['UUID',server.value.id],['Agent',server.value.agent_version],['OS',[server.value.distribution,server.value.os_version].filter(Boolean).join(' ')],['Kernel',server.value.kernel],['Architecture',server.value.architecture],['CPU',`${server.value.cpu_model||''} (${server.value.cpu_cores||0} cores)`],['RAM',bytes(server.value.ram_total)],['Disk',bytes(server.value.disk_total)],['Public IP',server.value.public_ip],['Private IP',server.value.private_ip],['Channel',server.value.update_channel],['Heartbeat',server.value.last_heartbeat?new Date(server.value.last_heartbeat).toLocaleString():'Never']])
const activeItems=computed(()=>({tasks:tasks.value,events:events.value,heartbeats:heartbeats.value}[tab.value]||[]))
async function load(){loading.value=true;try{const id=route.params.id,[s,t,e,hb,d]=await Promise.all([api.get(`/servers/${id}`),api.get(`/servers/${id}/tasks`),api.get(`/servers/${id}/events`),api.get(`/servers/${id}/heartbeats`),api.get(`/servers/${id}/capabilities`)]);server.value=s.data;tasks.value=t.data||[];events.value=e.data||[];heartbeats.value=hb.data||[];diagnostics.value=d.data}catch(e){errorMessage.value=apiErrorMessage(e)}finally{loading.value=false}}
async function run(action){try{await api.post(`/servers/${route.params.id}/actions/${action}`);await load()}catch(e){errorMessage.value=apiErrorMessage(e)}} async function remove(){if(!window.confirm('Delete this server and its history?'))return;await api.delete(`/servers/${route.params.id}`);router.push('/servers')}
async function reissueEnrollment(){try{const {data}=await api.post(`/servers/${route.params.id}/enrollment`);enrollment.value=data;errorMessage.value=''}catch(e){errorMessage.value=apiErrorMessage(e)}} async function copyCommand(){await navigator.clipboard.writeText(enrollment.value.installation_command)}
async function copyUpdateCommand(){await navigator.clipboard.writeText(agentUpdateCommand)}
const dateTime=v=>v&&!String(v).startsWith('0001-')?new Date(v).toLocaleString():'Never'
const badge=v=>v==='online'?'badge-success':v==='warning'?'badge-warning':v==='offline'?'badge-danger':'badge-muted',bytes=v=>v?`${(v/1073741824).toFixed(1)} GB`:'—',duration=v=>v?`${Math.floor(v/86400)}d ${Math.floor((v%86400)/3600)}h`:'—'
onMounted(load)
</script>
