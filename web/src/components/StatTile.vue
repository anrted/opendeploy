<template>
  <div class="card flex flex-col gap-3">
    <div class="flex items-start justify-between">
      <div>
        <p class="label">{{ label }}</p>
        <p class="text-2xl font-bold tracking-tight" :class="valueColor">{{ value }}</p>
        <p class="text-xs mt-0.5" :class="subtitleColor">{{ subtitle }}</p>
      </div>
      <div :class="['w-10 h-10 rounded-xl flex items-center justify-center flex-shrink-0', iconBg]">
        <svg class="w-5 h-5" :class="iconColor" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path v-if="icon === 'cpu'" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M9 3H7a2 2 0 00-2 2v2M9 3h6M9 3v2m6-2h2a2 2 0 012 2v2m0 0V7m0 0h-2M3 9v6m0 0v2a2 2 0 002 2h2m0 0h6m0 0h2a2 2 0 002-2v-2m0 0V9M9 21v-2m6 2v-2M9 9h6v6H9V9z"/>
          <path v-else-if="icon === 'memory'" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"/>
          <path v-else-if="icon === 'disk'" stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z"/>
          <path v-else stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                d="M13 10V3L4 14h7v7l9-11h-7z"/>
        </svg>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: String,
  value: String,
  subtitle: String,
  icon: { type: String, default: 'activity' },
  color: { type: String, default: 'emerald' },
})

const colorMap = {
  emerald: {
    value: 'text-emerald-400',
    subtitle: 'text-[#64748b]',
    iconBg: 'bg-emerald-500/10',
    iconColor: 'text-emerald-400',
  },
  amber: {
    value: 'text-amber-400',
    subtitle: 'text-[#64748b]',
    iconBg: 'bg-amber-500/10',
    iconColor: 'text-amber-400',
  },
  red: {
    value: 'text-red-400',
    subtitle: 'text-[#64748b]',
    iconBg: 'bg-red-500/10',
    iconColor: 'text-red-400',
  },
  indigo: {
    value: 'text-indigo-400',
    subtitle: 'text-[#64748b]',
    iconBg: 'bg-indigo-500/10',
    iconColor: 'text-indigo-400',
  },
  muted: {
    value: 'text-[#94a3b8]',
    subtitle: 'text-[#64748b]',
    iconBg: 'bg-slate-500/10',
    iconColor: 'text-slate-400',
  },
}

const colors = computed(() => colorMap[props.color] || colorMap.emerald)
const valueColor = computed(() => colors.value.value)
const subtitleColor = computed(() => colors.value.subtitle)
const iconBg = computed(() => colors.value.iconBg)
const iconColor = computed(() => colors.value.iconColor)
</script>
