<template>
  <div
    class="flex shrink-0 items-center justify-center rounded-2xl border border-indigo-400/25 bg-indigo-500/10 text-indigo-400"
    :class="sizeClass"
  >
    <svg :class="svgSize" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path
        v-for="path in paths"
        :key="path"
        :d="path"
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="1.8"
      />
    </svg>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  icon: { type: String, required: true },
  size: { type: String, default: 'md' },
})

const icons = {
  server: [
    'M4 5.5A1.5 1.5 0 015.5 4h13A1.5 1.5 0 0120 5.5v4a1.5 1.5 0 01-1.5 1.5h-13A1.5 1.5 0 014 9.5v-4z',
    'M4 14.5A1.5 1.5 0 015.5 13h13a1.5 1.5 0 011.5 1.5v4a1.5 1.5 0 01-1.5 1.5h-13A1.5 1.5 0 014 18.5v-4z',
    'M7 7.5h.01M7 16.5h.01M10 7.5h7M10 16.5h7',
  ],
  database: [
    'M19 6c0 1.657-3.134 3-7 3S5 7.657 5 6s3.134-3 7-3 7 1.343 7 3z',
    'M5 6v6c0 1.657 3.134 3 7 3s7-1.343 7-3V6',
    'M5 12v6c0 1.657 3.134 3 7 3s7-1.343 7-3v-6',
  ],
  code: ['M8 9l-4 3 4 3', 'M16 9l4 3-4 3', 'M14 5l-4 14'],
  shield: ['M12 3l8 3v5c0 5.2-3.4 8.7-8 10-4.6-1.3-8-4.8-8-10V6l8-3z', 'M9 12l2 2 4-4'],
  'git-branch': ['M6 3v12a3 3 0 003 3h6', 'M18 6V3m0 0l-3 3m3-3l3 3', 'M6 6a3 3 0 100-6 3 3 0 000 6z', 'M18 21a3 3 0 100-6 3 3 0 000 6z'],
  box: ['M4 7l8-4 8 4-8 4-8-4z', 'M4 7v10l8 4 8-4V7', 'M12 11v10'],
}

const paths = computed(() => icons[props.icon] || icons.box)
const sizeClass = computed(() => ({
  sm: 'h-9 w-9',
  md: 'h-12 w-12',
  lg: 'h-16 w-16 rounded-[1.25rem]',
}[props.size] || 'h-12 w-12'))
const svgSize = computed(() => props.size === 'lg' ? 'h-8 w-8' : props.size === 'sm' ? 'h-5 w-5' : 'h-6 w-6')
</script>
