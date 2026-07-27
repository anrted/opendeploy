<template>
  <component :is="iconComponent" aria-hidden="true" />
</template>

<script setup>
import { computed } from 'vue'
import {
  ArrowDownIcon,
  ArrowDownTrayIcon,
  ArrowPathIcon,
  CheckCircleIcon,
  DocumentTextIcon,
  MagnifyingGlassIcon,
  PlayIcon,
  QuestionMarkCircleIcon,
  ShieldCheckIcon,
  ShieldExclamationIcon,
  StopIcon,
  TrashIcon
} from '@heroicons/vue/24/outline'

const props = defineProps({
  name: { type: String, required: true }
})

const icons = {
  'arrow-down': ArrowDownIcon,
  'check-circle': CheckCircleIcon,
  download: ArrowDownTrayIcon,
  'file-text': DocumentTextIcon,
  play: PlayIcon,
  refresh: ArrowPathIcon,
  'refresh-cw': ArrowPathIcon,
  'rotate-cw': ArrowPathIcon,
  search: MagnifyingGlassIcon,
  shield: ShieldCheckIcon,
  'shield-off': ShieldExclamationIcon,
  square: StopIcon,
  trash: TrashIcon,
  'trash-2': TrashIcon
}

const iconComponent = computed(() => icons[props.name] || QuestionMarkCircleIcon)
</script>
