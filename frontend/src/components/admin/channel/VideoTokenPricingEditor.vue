<template>
  <div class="mt-3 space-y-4" data-testid="video-token-pricing-editor">
    <div class="flex flex-wrap items-center gap-2">
      <button type="button" class="btn text-xs" :class="version === 'standard' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="version === 'standard'" @click="preset(false)">{{ t('admin.channels.videoToken.standard') }}</button>
      <button type="button" class="btn text-xs" :class="version === 'fast' ? 'btn-primary' : 'btn-secondary'" :aria-pressed="version === 'fast'" @click="preset(true)">{{ t('admin.channels.videoToken.fast') }}</button>
      <span class="text-xs text-gray-500">24 fps / 1024</span>
    </div>
    <div class="overflow-x-auto">
      <table class="w-full text-left text-xs">
        <thead class="text-gray-500"><tr>
          <th class="pb-2">{{ t('admin.channels.videoToken.tier') }}</th>
          <th class="pb-2">{{ t('admin.channels.videoToken.withoutVideo') }}</th>
          <th class="pb-2">{{ t('admin.channels.videoToken.withVideo') }}</th>
        </tr></thead>
        <tbody>
          <tr v-for="(tier, name) in rule.tiers" :key="name">
            <td class="py-1 pr-2 font-medium">{{ displayTierName(String(name)) }}</td>
            <td class="py-1 pr-2"><input :value="tier.without_video" :aria-label="`${displayTierName(String(name))} ${t('admin.channels.videoToken.withoutVideo')}`" type="text" inputmode="decimal" class="input min-w-24 text-sm" @input="setRate(String(name), 'without_video', ($event.target as HTMLInputElement).value)" /></td>
            <td class="py-1"><input :value="tier.with_video" :aria-label="`${displayTierName(String(name))} ${t('admin.channels.videoToken.withVideo')}`" type="text" inputmode="decimal" class="input min-w-24 text-sm" @input="setRate(String(name), 'with_video', ($event.target as HTMLInputElement).value)" /></td>
          </tr>
        </tbody>
      </table>
    </div>
    <details class="border-t border-gray-200 pt-3 dark:border-dark-600">
      <summary class="cursor-pointer text-xs font-medium">{{ t('admin.channels.videoToken.sizes') }}</summary>
      <div v-for="(tier, name) in rule.tiers" :key="name" class="mt-3">
        <div class="mb-1 text-xs font-medium">{{ displayTierName(String(name)) }}</div>
        <div v-for="(size, ratio) in tier.sizes" :key="ratio" class="mb-2 grid grid-cols-[4rem_minmax(4rem,1fr)_minmax(4rem,1fr)_2rem] items-center gap-2">
          <span class="text-xs">{{ ratio }}</span>
          <input :value="size.width" :aria-label="`${displayTierName(String(name))} ${ratio} ${t('admin.channels.videoToken.width')}`" type="number" min="16" max="8192" class="input text-sm" @input="setSize(String(name), String(ratio), 'width', ($event.target as HTMLInputElement).value)" />
          <input :value="size.height" :aria-label="`${displayTierName(String(name))} ${ratio} ${t('admin.channels.videoToken.height')}`" type="number" min="16" max="8192" class="input text-sm" @input="setSize(String(name), String(ratio), 'height', ($event.target as HTMLInputElement).value)" />
          <button type="button" :title="t('admin.channels.videoToken.removeRatio')" :aria-label="t('admin.channels.videoToken.removeRatio')" @click="removeRatio(String(name), String(ratio))"><Icon name="trash" size="sm" /></button>
        </div>
        <div class="flex items-center gap-2">
          <input v-model="newRatios[String(name)]" type="text" placeholder="4:3" :aria-label="t('admin.channels.videoToken.ratio')" class="input w-24 text-xs" />
          <button type="button" :title="t('admin.channels.videoToken.addRatio')" :aria-label="t('admin.channels.videoToken.addRatio')" @click="addRatio(String(name))"><Icon name="plus" size="sm" /></button>
        </div>
      </div>
    </details>
    <label class="block max-w-xs text-xs text-gray-600 dark:text-gray-300">
      {{ t('admin.channels.videoToken.maxInput') }}
      <input :value="rule.max_input_seconds" type="number" min="1" max="3600" class="input mt-1 text-sm" @input="update({ max_input_seconds: Number(($event.target as HTMLInputElement).value) })" />
    </label>
    <div class="border-t border-gray-200 pt-3 dark:border-dark-600">
      <div class="mb-3 text-xs font-medium">{{ t('admin.channels.parameter.preview') }}</div>
      <div class="grid grid-cols-2 gap-3">
        <label class="text-xs">{{ t('admin.channels.videoToken.tier') }}<select v-model="tierName" class="input mt-1 text-sm"><option v-for="(_, name) in rule.tiers" :key="name" :value="name">{{ displayTierName(String(name)) }}</option></select></label>
        <label class="text-xs">{{ t('admin.channels.videoToken.ratio') }}<select v-model="ratio" class="input mt-1 text-sm"><option v-for="(_, name) in rule.tiers[tierName]?.sizes" :key="name" :value="name">{{ name }}</option></select></label>
        <label class="text-xs">{{ t('admin.channels.videoToken.outputSeconds') }}<input v-model="outputSeconds" type="text" inputmode="decimal" class="input mt-1 text-sm" /></label>
        <label class="text-xs">{{ t('admin.channels.videoToken.multiplier') }}<input v-model="multiplier" type="text" inputmode="decimal" class="input mt-1 text-sm" /></label>
        <label class="col-span-2 flex items-center gap-2 text-xs"><input v-model="hasVideo" type="checkbox" />{{ t('admin.channels.videoToken.hasVideo') }}</label>
        <label v-if="hasVideo" class="col-span-2 text-xs">{{ t('admin.channels.videoToken.inputSeconds') }}<input v-model="inputSeconds" type="text" inputmode="decimal" class="input mt-1 text-sm" /></label>
      </div>
      <button type="button" :disabled="busy" class="btn btn-primary mt-3 text-sm" @click="preview">{{ t('admin.channels.parameter.calculate') }}</button>
      <p v-if="error" role="alert" class="mt-2 break-words text-sm text-red-600">{{ error }}</p>
      <div v-if="quote" class="mt-3 space-y-1 break-words text-sm" aria-live="polite">
        <p>{{ t('admin.channels.videoToken.formula') }}</p>
        <p>{{ quote.tokens.toLocaleString() }} Token × ${{ quote.price_per_million }} / 1,000,000 × {{ quote.input.multiplier }}</p>
        <p class="font-semibold text-primary-700 dark:text-primary-300">{{ t('admin.channels.parameter.actual') }}: ${{ quote.actual_cost }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import { previewVideoTokenPricing, type VideoTokenPricing, type VideoTokenPricingSnapshot } from '@/api/admin/channels'
import { supplierVideoTokenPricing, videoTokenPricingVersion } from './videoTokenPricing'

const props = defineProps<{ modelValue?: VideoTokenPricing | null }>()
const emit = defineEmits<{ 'update:modelValue': [value: VideoTokenPricing] }>()
const { t } = useI18n()
const rule = computed(() => props.modelValue || supplierVideoTokenPricing())
const version = computed(() => videoTokenPricingVersion(rule.value))
const tierName = ref(Object.keys(rule.value.tiers)[0] || '')
const ratio = ref('16:9')
const outputSeconds = ref('5')
const inputSeconds = ref('0')
const hasVideo = ref(false)
const multiplier = ref('1')
const newRatios = ref<Record<string, string>>({})
const quote = ref<VideoTokenPricingSnapshot | null>(null)
const busy = ref(false)
const error = ref('')
let revision = 0
watch([rule, tierName, ratio, outputSeconds, inputSeconds, hasVideo, multiplier], () => {
  revision++
  quote.value = null
  error.value = ''
  if (!rule.value.tiers[tierName.value]) tierName.value = Object.keys(rule.value.tiers)[0] || ''
  if (!rule.value.tiers[tierName.value]?.sizes[ratio.value]) ratio.value = Object.keys(rule.value.tiers[tierName.value]?.sizes || {})[0] || ''
}, { deep: true })
function copy(): VideoTokenPricing { return JSON.parse(JSON.stringify(rule.value)) }
function displayTierName(name: string): string {
  return name.startsWith('fast:') ? name.slice('fast:'.length) : name
}
function update(patch: Partial<VideoTokenPricing>) { emit('update:modelValue', { ...rule.value, ...patch }) }
function preset(fast: boolean) {
  if (version.value === (fast ? 'fast' : 'standard')) return
  emit('update:modelValue', supplierVideoTokenPricing(fast))
}
function setRate(name: string, key: 'with_video' | 'without_video', value: string) {
  const next = copy()
  next.tiers[name][key] = value
  emit('update:modelValue', next)
}
function setSize(name: string, ratio: string, key: 'width' | 'height', value: string) {
  const next = copy()
  next.tiers[name].sizes[ratio][key] = Number(value)
  emit('update:modelValue', next)
}
function removeRatio(name: string, ratio: string) {
  const next = copy()
  delete next.tiers[name].sizes[ratio]
  emit('update:modelValue', next)
}
function addRatio(name: string) {
  const ratio = newRatios.value[name]?.trim()
  if (!ratio || !/^[1-9]\d*:[1-9]\d*$/.test(ratio) || rule.value.tiers[name].sizes[ratio]) return
  const next = copy()
  next.tiers[name].sizes[ratio] = { width: 0, height: 0 }
  emit('update:modelValue', next)
  newRatios.value[name] = ''
}
async function preview() {
  const at = revision
  busy.value = true
  error.value = ''
  try {
    const size = rule.value.tiers[tierName.value]?.sizes[ratio.value]
    const result = await previewVideoTokenPricing(rule.value, {
      tier: tierName.value, width: size?.width || 0, height: size?.height || 0,
      output_seconds: outputSeconds.value, input_seconds: hasVideo.value ? inputSeconds.value : '0',
      has_video: hasVideo.value, multiplier: multiplier.value,
    })
    if (at === revision) quote.value = result
  } catch (e) {
    if (at === revision) error.value = (e as { message?: string }).message || t('admin.channels.parameter.invalid')
  } finally { busy.value = false }
}
</script>
