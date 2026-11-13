<template>
  <div class="mt-3 space-y-3 border-t border-gray-200 pt-3 dark:border-dark-600">
    <div class="grid gap-2 sm:grid-cols-2">
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.currency') }}
        <select :value="rule.currency" class="input mt-1 text-sm" @change="update({ currency: ($event.target as HTMLSelectElement).value as VideoTaskPricing['currency'] })">
          <option value="CNY">CNY</option>
          <option value="USD">USD</option>
        </select>
      </label>
      <label v-if="rule.currency === 'CNY'" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.cnyPerUsd') }}
        <input :value="rule.cny_per_usd" type="number" min="0.00000001" step="any" class="input mt-1 text-sm" @input="update({ cny_per_usd: ($event.target as HTMLInputElement).value })" />
      </label>
    </div>
    <div class="grid gap-2 sm:grid-cols-2">
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.price480') }} / {{ t('admin.channels.videoTaskPricing.perSecond') }}
        <input :value="rule.price_480p_per_second" type="number" min="0" step="any" class="input mt-1 text-sm" @input="update({ price_480p_per_second: ($event.target as HTMLInputElement).value })" />
      </label>
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.price768') }} / {{ t('admin.channels.videoTaskPricing.perSecond') }}
        <input :value="rule.price_768p_per_second" type="number" min="0" step="any" class="input mt-1 text-sm" @input="update({ price_768p_per_second: ($event.target as HTMLInputElement).value })" />
      </label>
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.upgrade2k') }} / {{ t('admin.channels.videoTaskPricing.perSecond') }}
        <input :value="rule.upgrade_2k_per_second" type="number" min="0" step="any" class="input mt-1 text-sm" @input="update({ upgrade_2k_per_second: ($event.target as HTMLInputElement).value })" />
      </label>
      <label class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('admin.channels.videoTaskPricing.promptPolish') }} / {{ t('admin.channels.videoTaskPricing.perCall') }}
        <input :value="rule.prompt_polish_price_per_call" type="number" min="0" step="any" class="input mt-1 text-sm" @input="update({ prompt_polish_price_per_call: ($event.target as HTMLInputElement).value })" />
      </label>
    </div>
    <label class="flex items-center gap-2 text-xs text-gray-600 dark:text-gray-300">
      <input :checked="rule.prompt_polish_default_enabled" type="checkbox" class="checkbox" @change="update({ prompt_polish_default_enabled: ($event.target as HTMLInputElement).checked })" />
      {{ t('admin.channels.videoTaskPricing.promptPolishDefault') }}
    </label>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { VideoTaskPricing } from '@/api/admin/channels'
import { emptyVideoTaskPricing } from './videoTaskPricing'

const props = defineProps<{ modelValue?: VideoTaskPricing | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: VideoTaskPricing] }>()
const { t } = useI18n()
const rule = computed(() => props.modelValue || emptyVideoTaskPricing())

function update(patch: Partial<VideoTaskPricing>) {
  emit('update:modelValue', { ...rule.value, ...patch })
}
</script>
