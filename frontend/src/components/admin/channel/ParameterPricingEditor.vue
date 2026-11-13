<template>
  <div class="mt-3 space-y-4">
    <div class="flex flex-wrap items-end justify-between gap-3">
      <label class="text-xs text-gray-600 dark:text-gray-300">
        {{ t('admin.channels.parameter.basePrice') }}
        <input :value="rule.base_price" type="text" inputmode="decimal" class="input mt-1 w-48 text-sm"
          @input="update({ base_price: ($event.target as HTMLInputElement).value })" />
      </label>
      <button type="button" class="btn btn-secondary text-xs" @click="emit('update:modelValue', supplierImagePricing())">
        <Icon name="download" size="sm" class="mr-1" />{{ t('admin.channels.parameter.preset') }}
      </button>
    </div>
    <div v-for="(factor, index) in rule.factors" :key="index" class="border-t border-gray-200 pt-3 dark:border-dark-600">
      <div class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_2rem] items-end gap-2">
        <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('admin.channels.parameter.key') }}
          <input :value="factor.key" class="input mt-1 text-sm" type="text"
            @input="setFactor(index, { key: ($event.target as HTMLInputElement).value })" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('admin.channels.parameter.default') }}
          <select :value="factor.default || (factor.key === 'variant' && factor.values.flare ? 'flare' : '')" class="input mt-1 text-sm"
            @change="setFactor(index, { default: ($event.target as HTMLSelectElement).value })">
            <option value="">{{ t('admin.channels.parameter.required') }}</option>
            <option v-for="key in Object.keys(factor.values)" :key="key" :value="key">{{ key }}</option>
          </select>
        </label>
        <button type="button" class="mb-2 text-gray-400 hover:text-red-500" :title="t('admin.channels.parameter.removeFactor')"
          :aria-label="t('admin.channels.parameter.removeFactor')" @click="removeFactor(index)"><Icon name="trash" size="sm" /></button>
      </div>
      <div class="mt-2 grid grid-cols-1 gap-x-5 gap-y-1 sm:grid-cols-2">
        <div v-for="(value, key) in factor.values" :key="key" class="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_1.5rem] items-center gap-2">
          <input :value="key" :aria-label="t('admin.channels.parameter.value')" class="input text-sm" type="text"
            @change="renameValue(index, String(key), ($event.target as HTMLInputElement).value)" />
          <input :value="value" :aria-label="t('admin.channels.parameter.factor')" class="input text-sm" type="text" inputmode="decimal"
            @input="setValue(index, String(key), ($event.target as HTMLInputElement).value)" />
          <button type="button" :title="t('admin.channels.parameter.removeValue')" :aria-label="t('admin.channels.parameter.removeValue')"
            class="text-gray-400 hover:text-red-500" @click="removeValue(index, String(key))"><Icon name="x" size="sm" /></button>
        </div>
      </div>
      <button type="button" class="mt-2 inline-flex items-center gap-1 text-xs text-primary-600" @click="addValue(index)">
        <Icon name="plus" size="sm" />{{ t('admin.channels.parameter.addValue') }}
      </button>
    </div>
    <button type="button" class="inline-flex items-center gap-1 text-xs text-primary-600" @click="addFactor">
      <Icon name="plus" size="sm" />{{ t('admin.channels.parameter.addFactor') }}
    </button>
    <div class="border-t border-gray-200 pt-3 dark:border-dark-600">
      <h4 class="text-sm font-medium text-gray-700 dark:text-gray-200">{{ t('admin.channels.parameter.preview') }}</h4>
      <div class="mt-2 grid grid-cols-2 gap-2 sm:grid-cols-3">
        <label v-for="factor in rule.factors" :key="factor.key" class="text-xs text-gray-600 dark:text-gray-300">{{ factor.key }}
          <select v-model="dimensions[factor.key]" class="input mt-1 text-sm">
            <option value="">{{ t('admin.channels.parameter.useDefault') }}</option>
            <option v-for="key in Object.keys(factor.values)" :key="key" :value="key">{{ key }}</option>
          </select>
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('admin.channels.parameter.quantity') }}
          <input v-model.number="quantity" type="number" min="1" max="10000" class="input mt-1 text-sm" />
        </label>
        <label class="text-xs text-gray-600 dark:text-gray-300">{{ t('admin.channels.parameter.multiplier') }}
          <input v-model="multiplier" type="text" inputmode="decimal" class="input mt-1 text-sm" />
        </label>
      </div>
      <button type="button" :disabled="busy" class="btn btn-primary mt-3 text-sm" @click="preview">
        {{ t('admin.channels.parameter.calculate') }}
      </button>
      <p v-if="error" role="alert" class="mt-2 break-words text-sm text-red-600">{{ error }}</p>
      <div v-if="quote" class="mt-3 space-y-1 break-words text-sm text-gray-700 dark:text-gray-200" aria-live="polite">
        <p>{{ formula }}</p>
        <p class="font-semibold text-primary-700 dark:text-primary-300">{{ t('admin.channels.parameter.actual') }}: ${{ quote.actual_cost }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { previewParameterPricing, type ParameterPricing, type ParameterPricingSnapshot } from '@/api/admin/channels'
import { cloneParameterPricing, emptyParameterPricing, supplierImagePricing } from './parameterPricing'

const props = defineProps<{ modelValue?: ParameterPricing | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: ParameterPricing] }>()
const { t } = useI18n()
const rule = computed(() => props.modelValue || emptyParameterPricing())
const dimensions = ref<Record<string, string>>({})
const quantity = ref(1)
const multiplier = ref('1')
const quote = ref<ParameterPricingSnapshot | null>(null)
const busy = ref(false)
const error = ref('')
let revision = 0
watch([() => props.modelValue, dimensions, quantity, multiplier], () => {
  revision++
  quote.value = null
  error.value = ''
}, { deep: true })
const formula = computed(() => quote.value
  ? `$${quote.value.rule.base_price} × ${Object.values(quote.value.matched_factors).join(' × ') || '1'} × ${quote.value.input.quantity} × ${quote.value.input.multiplier}`
  : '')

function update(patch: Partial<ParameterPricing>) {
  emit('update:modelValue', { ...rule.value, ...patch })
}
function setFactor(index: number, patch: Partial<ParameterPricing['factors'][number]>) {
  const next = cloneParameterPricing(rule.value)!
  next.factors[index] = { ...next.factors[index], ...patch }
  emit('update:modelValue', next)
}
function addFactor() {
  update({ factors: [...rule.value.factors, { key: '', values: { value: '1' } }] })
}
function removeFactor(index: number) {
  update({ factors: rule.value.factors.filter((_, i) => i !== index) })
}
function setValue(index: number, key: string, value: string) {
  setFactor(index, { values: { ...rule.value.factors[index].values, [key]: value } })
}
function removeValue(index: number, key: string) {
  const values = { ...rule.value.factors[index].values }
  delete values[key]
  setFactor(index, { values })
}
function renameValue(index: number, old: string, key: string) {
  key = key.trim().toLowerCase()
  if (!key || (key !== old && key in rule.value.factors[index].values)) return
  const factor = rule.value.factors[index]
  const values = Object.fromEntries(Object.entries(factor.values).map(([k, v]) => [k === old ? key : k, v]))
  setFactor(index, { values, default: factor.default === old ? key : factor.default })
}
function addValue(index: number) {
  const values = rule.value.factors[index].values
  let key = 'value'
  for (let n = 2; key in values; n++) key = `value${n}`
  setValue(index, key, '1')
}
async function preview() {
  const at = revision
  busy.value = true
  error.value = ''
  try {
    const result = await previewParameterPricing(rule.value, {
      dimensions: dimensions.value, quantity: quantity.value, multiplier: multiplier.value,
    })
    if (at === revision) quote.value = result
  } catch (e) {
    if (at === revision) error.value = (e as { message?: string }).message || t('admin.channels.parameter.invalid')
  } finally {
    busy.value = false
  }
}
</script>
