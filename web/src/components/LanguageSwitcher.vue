<template>
  <div class="relative min-w-0 max-w-[7rem] text-left sm:max-w-none">
    <select
      v-model="$i18n.locale"
      @change="changeLocale"
      class="block max-w-full bg-transparent border border-white/10 text-sm rounded px-2 py-1 text-[#e2e8f0] focus:outline-none focus:ring-1 focus:ring-indigo-500"
    >
      <option value="en" class="bg-[#1a1b26]">English</option>
      <option value="ru" class="bg-[#1a1b26]">Русский</option>
    </select>
  </div>
</template>

<script setup>
import { useI18n } from 'vue-i18n'

const { locale } = useI18n()

function changeLocale(event) {
  localStorage.setItem('opendeploy_locale', event.target.value)
}
</script>
