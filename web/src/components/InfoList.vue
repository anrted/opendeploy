<template>
  <section>
    <h4 class="mb-2 text-xs font-semibold uppercase tracking-wide text-[#64748b]">{{ label }}</h4>
    <ul class="space-y-1.5">
      <li v-for="(item, index) in visibleItems" :key="index"
        class="rounded-lg border border-[#334155] bg-[#1e293b] px-3 py-2 text-[#cbd5e1]"
        :class="{ 'font-mono text-xs': mono }">{{ item }}</li>
    </ul>
  </section>
</template>

<script setup>
import { computed } from 'vue'

const props = defineProps({
  label: { type: String, required: true },
  items: { type: Array, default: () => [] },
  mono: Boolean,
})

const visibleItems = computed(() => props.items.filter(Boolean))
</script>
