<template>
  <div class="mt-3 space-y-3">
    <div class="grid gap-2 border-t border-gray-200 pt-3 dark:border-dark-600 sm:grid-cols-2">
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.currency') }}
        <select v-model="currency" class="input mt-1 text-sm" data-testid="image-price-currency">
          <option value="USD">USD</option>
          <option value="CNY">CNY</option>
        </select>
      </label>
      <label v-if="currency === 'CNY'" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.cnyPerUsd') }}
        <input :value="exchangeRate" type="number" min="0.00000001" step="any"
          class="input mt-1 text-sm" data-testid="image-exchange-rate"
          @input="updateExchangeRate(($event.target as HTMLInputElement).value)" />
      </label>
    </div>
    <div class="grid gap-2 sm:grid-cols-2" data-testid="image-price-fields">
      <label class="min-w-0 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.defaultImagePrice') }}
        <span class="ml-1 font-normal text-gray-400">{{ currency }}/{{ t('admin.channels.form.perImage') }}</span>
        <input :value="displayPrice(entry.per_request_price)" type="number" min="0" step="any"
          class="input mt-1 text-sm" :disabled="!canEdit" data-testid="image-generation-price"
          :placeholder="t('admin.channels.form.pricePlaceholder')"
          @input="updatePrice('per_request_price', ($event.target as HTMLInputElement).value)" />
      </label>
      <label class="min-w-0 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.imageEditPrice') }}
        <span class="ml-1 font-normal text-gray-400">{{ currency }}/{{ t('admin.channels.form.perImage') }}</span>
        <input :value="displayPrice(entry.image_edit_price)" type="number" min="0" step="any"
          class="input mt-1 text-sm" :disabled="!canEdit" data-testid="image-edit-price"
          :placeholder="t('admin.channels.form.imageEditFallback')"
          @input="updatePrice('image_edit_price', ($event.target as HTMLInputElement).value)" />
      </label>
    </div>
    <p v-if="currency === 'CNY'" class="text-xs text-gray-500 dark:text-gray-400">
      {{ t(canEdit ? 'admin.channels.form.imageCurrencyHint' : 'admin.channels.form.imageExchangeRateRequired') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { convertCNYToUSD } from './imagePriceConversion'
import type { PricingFormEntry } from './types'

const props = defineProps<{ entry: PricingFormEntry }>()
const emit = defineEmits<{ update: [entry: PricingFormEntry] }>()
const { t } = useI18n()
const currency = ref('USD')
const exchangeRate = ref('')
const lastValidRate = ref<number | null>(null)
const canEdit = computed(() => currency.value === 'USD' || convertCNYToUSD(1, exchangeRate.value) !== null)

function displayPrice(value: number | string | null | undefined) {
  if (value == null || value === '') return ''
  if (currency.value === 'USD') return value
  return canEdit.value ? Number((Number(value) * Number(exchangeRate.value)).toFixed(10)) : ''
}

function updatePrice(field: 'per_request_price' | 'image_edit_price', value: string) {
  if (!canEdit.value) return
  const price = value === '' ? null : currency.value === 'CNY' ? convertCNYToUSD(value, exchangeRate.value) : value
  if (value !== '' && price === null) return
  emit('update', { ...props.entry, [field]: price })
}

function updateExchangeRate(value: string) {
  exchangeRate.value = value
  if (convertCNYToUSD(1, value) === null) return
  const rate = Number(value)
  // Preserve the entered CNY prices when changing an already valid exchange rate.
  const previousRate = lastValidRate.value
  if (previousRate !== null) {
    const convert = (price: number | string | null | undefined) =>
      price == null || price === '' ? price : convertCNYToUSD(Number(price) * previousRate, rate)
    emit('update', {
      ...props.entry,
      per_request_price: convert(props.entry.per_request_price) ?? null,
      image_edit_price: convert(props.entry.image_edit_price) ?? null,
    })
  }
  lastValidRate.value = rate
}
</script>
