<template>
  <div class="mt-3 space-y-3">
    <div class="grid gap-2 border-t border-gray-200 pt-3 dark:border-dark-600 sm:grid-cols-2">
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.currency') }}
        <select :value="currency" class="input mt-1 text-sm" data-testid="image-price-currency"
          @change="emit('update', { ...entry, image_price_currency: ($event.target as HTMLSelectElement).value as 'USD' | 'CNY' })">
          <option value="USD">USD</option>
          <option value="CNY">CNY</option>
        </select>
      </label>
      <label v-if="currency === 'CNY'" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.cnyPerUsd') }}
        <input :value="entry.image_cny_per_usd" type="number" min="0.00000001" step="any"
          class="input mt-1 text-sm" data-testid="image-exchange-rate"
          @input="emit('update', { ...entry, image_cny_per_usd: ($event.target as HTMLInputElement).value })" />
      </label>
    </div>
    <div class="grid gap-2 sm:grid-cols-2" data-testid="image-price-fields">
      <label class="min-w-0 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.defaultImagePrice') }}
        <span class="ml-1 font-normal text-gray-400">{{ currency }}/{{ t('admin.channels.form.perImage') }}</span>
        <input :value="entry.per_request_price" type="number" min="0" step="any"
          class="input mt-1 text-sm" data-testid="image-generation-price"
          :placeholder="t('admin.channels.form.pricePlaceholder')"
          @input="updatePrice('per_request_price', ($event.target as HTMLInputElement).value)" />
      </label>
      <label class="min-w-0 text-xs font-medium text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.form.imageEditPrice') }}
        <span class="ml-1 font-normal text-gray-400">{{ currency }}/{{ t('admin.channels.form.perImage') }}</span>
        <input :value="entry.image_edit_price" type="number" min="0" step="any"
          class="input mt-1 text-sm" data-testid="image-edit-price"
          :placeholder="t('admin.channels.form.imageEditFallback')"
          @input="updatePrice('image_edit_price', ($event.target as HTMLInputElement).value)" />
      </label>
    </div>
    <p v-if="currency === 'CNY'" class="text-xs text-gray-500 dark:text-gray-400">
      {{ t('admin.channels.form.imageCurrencyHint') }}
    </p>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { PricingFormEntry } from './types'

const props = defineProps<{ entry: PricingFormEntry }>()
const emit = defineEmits<{ update: [entry: PricingFormEntry] }>()
const { t } = useI18n()
const currency = computed(() => props.entry.image_price_currency || 'USD')

function updatePrice(field: 'per_request_price' | 'image_edit_price', value: string) {
  emit('update', { ...props.entry, [field]: value === '' ? null : value })
}
</script>
