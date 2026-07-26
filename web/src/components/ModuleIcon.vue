<template>
  <div 
    class="flex items-center justify-center rounded-2xl border"
    :class="[sizeClass, bgColor, borderColor]"
  >
    <i 
      class="feather" 
      :class="[`icon-${icon}`, iconSize, iconColor]"
    ></i>
  </div>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  icon: {
    type: String,
    required: true
  },
  size: {
    type: String,
    default: 'md' // sm, md, lg
  }
})

const sizeClass = computed(() => {
  switch (props.size) {
    case 'sm': return 'w-8 h-8'
    case 'lg': return 'w-16 h-16 rounded-[1.25rem]'
    default: return 'w-12 h-12'
  }
})

const iconSize = computed(() => {
  switch (props.size) {
    case 'sm': return 'text-sm'
    case 'lg': return 'text-3xl'
    default: return 'text-xl'
  }
})

const bgColor = computed(() => {
  return 'bg-black/20'
})

const borderColor = computed(() => {
  return 'border-[#334155]'
})

const iconColor = computed(() => {
  return 'text-indigo-400'
})
</script>
