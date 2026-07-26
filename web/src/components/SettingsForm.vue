<template>
  <div class="settings-form space-y-6">
    <div v-if="schema && schema.length">
      <!-- Basic / Advanced Toggle -->
      <div class="flex justify-end mb-4">
        <div class="bg-[#1e293b] p-1 rounded-lg inline-flex">
          <button 
            @click="showAdvanced = false" 
            :class="['px-4 py-1.5 text-sm font-medium rounded-md transition-colors', !showAdvanced ? 'bg-[#3b82f6] text-white' : 'text-[#94a3b8] hover:text-white']"
          >
            Basic
          </button>
          <button 
            @click="showAdvanced = true" 
            :class="['px-4 py-1.5 text-sm font-medium rounded-md transition-colors', showAdvanced ? 'bg-[#3b82f6] text-white' : 'text-[#94a3b8] hover:text-white']"
          >
            Advanced
          </button>
        </div>
      </div>

      <!-- Categories -->
      <div class="space-y-6">
        <div v-for="category in categories" :key="category.name" class="bg-[#1e293b] rounded-xl border border-[#334155] overflow-hidden">
          <div 
            class="px-6 py-4 border-b border-[#334155] bg-[#0f172a]/50 flex justify-between items-center cursor-pointer"
            @click="category.collapsed = !category.collapsed"
          >
            <h3 class="text-lg font-medium text-white">{{ category.name || 'General' }}</h3>
            <i :class="['feather', category.collapsed ? 'icon-chevron-down' : 'icon-chevron-up', 'text-[#64748b]']"></i>
          </div>
          
          <div v-show="!category.collapsed" class="p-6 space-y-5">
            <div v-for="setting in category.fields" :key="setting.id" class="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div class="md:col-span-1">
                <label class="block text-sm font-medium text-white mb-1">{{ setting.label }}</label>
                <p v-if="setting.description" class="text-xs text-[#94a3b8]">{{ setting.description }}</p>
                <div v-if="setting.requires_restart" class="mt-2 inline-flex items-center text-xs text-amber-400 bg-amber-400/10 px-2 py-1 rounded">
                  <i class="feather icon-alert-triangle mr-1"></i> Requires Restart
                </div>
              </div>
              
              <div class="md:col-span-2">
                <select v-if="setting.type === 'select'" v-model="formData[setting.id]" class="input w-full" :class="{'border-red-500': errors[setting.id]}">
                  <option v-for="opt in setting.options" :key="opt" :value="opt">{{ opt }}</option>
                </select>
                
                <label v-else-if="setting.type === 'boolean'" class="relative inline-flex items-center cursor-pointer mt-1">
                  <input type="checkbox" v-model="formData[setting.id]" class="sr-only peer">
                  <div class="w-11 h-6 bg-gray-700 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-blue-500"></div>
                </label>
                
                <input 
                  v-else 
                  :type="setting.type === 'number' ? 'number' : 'text'" 
                  v-model="formData[setting.id]" 
                  :placeholder="setting.placeholder"
                  class="input w-full" 
                  :class="{'border-red-500': errors[setting.id]}"
                  @input="validateField(setting)"
                />
                
                <p v-if="errors[setting.id]" class="text-xs text-red-400 mt-1">{{ errors[setting.id] }}</p>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Action Footer -->
      <div class="mt-6 flex justify-end">
        <button @click="saveSettings" class="btn btn-primary" :disabled="saving || hasErrors">
          <i v-if="saving" class="feather icon-loader animate-spin mr-2"></i>
          <i v-else class="feather icon-save mr-2"></i>
          Save Settings
        </button>
      </div>

      <!-- Save Confirmation Modal -->
      <div v-if="showConfirmModal" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm">
        <div class="bg-[#1e293b] rounded-xl border border-[#334155] p-6 max-w-md w-full shadow-2xl">
          <h3 class="text-xl font-bold text-white mb-2 flex items-center">
            <i class="feather icon-alert-circle text-amber-400 mr-2"></i>
            Service Restart Required
          </h3>
          <p class="text-[#94a3b8] mb-6">
            Some of the settings you modified require a full service restart to take effect. This may cause temporary downtime. Do you want to proceed?
          </p>
          <div class="flex justify-end space-x-3">
            <button @click="showConfirmModal = false" class="btn bg-transparent border border-[#334155] text-white hover:bg-[#334155]">Cancel</button>
            <button @click="executeSave" class="btn bg-amber-500 text-white hover:bg-amber-600">Restart & Save</button>
          </div>
        </div>
      </div>
    </div>
    
    <div v-else class="py-10 text-center text-[#64748b]">
      <i class="feather icon-settings text-4xl mb-4 text-[#475569]"></i>
      <h3 class="text-lg font-medium text-white mb-2">No Settings</h3>
      <p>This module does not have any configurable settings.</p>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted } from 'vue'
import api, { apiErrorMessage } from '@/api/client'

const props = defineProps({
  moduleId: {
    type: String,
    required: true
  },
  schema: {
    type: Array,
    default: () => []
  }
})

const showAdvanced = ref(false)
const formData = ref({})
const errors = ref({})
const saving = ref(false)
const showConfirmModal = ref(false)

// Organize fields into categories, filtering out advanced if needed
const categories = computed(() => {
  const cats = {}
  
  props.schema.forEach(field => {
    // Skip advanced fields if we are in basic mode
    if (!showAdvanced.value && field.advanced) return
    
    const catName = field.category || 'General'
    if (!cats[catName]) {
      cats[catName] = {
        name: catName,
        collapsed: false,
        fields: []
      }
    }
    cats[catName].fields.push(field)
  })
  
  return Object.values(cats)
})

const hasErrors = computed(() => {
  return Object.keys(errors.value).length > 0
})

// Initialize form data
onMounted(() => {
  initForm()
})

watch(() => props.schema, () => {
  initForm()
}, { deep: true })

function initForm() {
  const initial = {}
  props.schema.forEach(field => {
    initial[field.id] = field.value !== undefined ? field.value : ''
  })
  formData.value = initial
  errors.value = {}
}

function validateField(setting) {
  if (!setting.validation_regex) {
    delete errors.value[setting.id]
    return
  }
  
  const val = formData.value[setting.id]
  if (val === undefined || val === '') {
    delete errors.value[setting.id]
    return
  }
  
  try {
    const regex = new RegExp(setting.validation_regex)
    if (!regex.test(String(val))) {
      errors.value[setting.id] = 'Invalid format'
    } else {
      delete errors.value[setting.id]
    }
  } catch (e) {
    console.error("Invalid regex in schema", e)
  }
}

async function saveSettings() {
  // Validate all visible fields
  let isValid = true
  props.schema.forEach(field => {
    if ((showAdvanced.value || !field.advanced)) {
      validateField(field)
      if (errors.value[field.id]) isValid = false
    }
  })
  
  if (!isValid) return
  
  // Check if any modified field requires restart
  let needsRestart = false
  for (const field of props.schema) {
    if (field.requires_restart && formData.value[field.id] !== field.value) {
      needsRestart = true
      break
    }
  }
  
  if (needsRestart) {
    showConfirmModal.value = true
  } else {
    await executeSave()
  }
}

async function executeSave() {
  showConfirmModal.value = false
  saving.value = true
  
  try {
    // Send only the current form data
    await api.post(`/modules/${props.moduleId}/settings`, formData.value)
    
    // Update local schema values so next save comparison works
    props.schema.forEach(field => {
      if (formData.value[field.id] !== undefined) {
        field.value = formData.value[field.id]
      }
    })
    
    alert('Settings saved successfully')
  } catch (err) {
    alert('Failed to save settings: ' + apiErrorMessage(err))
  } finally {
    saving.value = false
  }
}
</script>
