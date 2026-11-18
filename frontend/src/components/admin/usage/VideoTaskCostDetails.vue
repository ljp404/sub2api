<template>
  <div class="space-y-1 break-words border-b border-gray-600 pb-2 text-xs text-gray-200">
    <div class="font-semibold">{{ t('usage.videoTaskRule') }}</div>
    <div>{{ snapshot.input.resolution.toUpperCase() }} · {{ snapshot.input.duration_seconds }}s</div>
    <div>{{ t('usage.videoTaskBase') }}: {{ baseRate }} {{ snapshot.source_currency }}/s × {{ snapshot.input.duration_seconds }}s → ${{ snapshot.base_price }}</div>
    <div v-if="snapshot.input.resolution === '2k'">
      {{ t('usage.videoTaskUpgrade') }}: {{ snapshot.rule.upgrade_2k_per_second }} {{ snapshot.source_currency }}/s × {{ snapshot.input.duration_seconds }}s → ${{ snapshot.upgrade_price }}
    </div>
    <div>{{ t('usage.videoTaskPrompt') }}: {{ t(snapshot.input.prompt_polish ? 'common.enabled' : 'common.disabled') }} · {{ snapshot.rule.prompt_polish_price_per_call }} {{ snapshot.source_currency }}/{{ t('usage.videoTaskCall') }} → ${{ snapshot.prompt_polish_price }}</div>
    <div v-if="snapshot.source_currency === 'CNY'">
      {{ t('usage.videoTaskFX') }}: {{ snapshot.fx_rate }} CNY/USD
    </div>
    <div>{{ t('usage.videoTaskTotal') }}: ${{ snapshot.total_cost }} × {{ snapshot.input.multiplier }} = ${{ snapshot.actual_cost }}</div>
    <div class="text-gray-400">{{ snapshot.rule_hash.slice(0, 16) }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { VideoTaskPricingSnapshot } from '@/api/admin/channels'

const props = defineProps<{ snapshot: VideoTaskPricingSnapshot }>()
const { t } = useI18n()
const baseRate = computed(() => props.snapshot.input.resolution === '480p'
  ? props.snapshot.rule.price_480p_per_second
  : props.snapshot.rule.price_768p_per_second)
</script>
