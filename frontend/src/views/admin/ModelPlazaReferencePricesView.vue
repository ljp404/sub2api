<template>
  <AppLayout>
    <div class="space-y-5">
      <section class="card overflow-hidden">
        <div class="flex flex-col gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="relative w-full sm:max-w-sm">
            <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
            <input v-model="query" class="input pl-9" :placeholder="t('admin.channels.referencePrices.search')" />
          </div>
          <div class="flex justify-end gap-2">
            <button class="btn btn-secondary" :disabled="catalogLoading || loading" @click="openCatalogImport">
              <Icon name="download" size="sm" class="mr-1" />
              {{ catalogLoading ? t('admin.channels.referencePrices.catalogLoading') : t('admin.channels.referencePrices.importCatalog') }}
            </button>
            <button
              v-if="selectedIds.length"
              class="btn btn-danger"
              :disabled="loading || deleting"
              @click="removeSelected"
            >
              <Icon name="trash" size="sm" class="mr-1" />
              {{ t('admin.channels.referencePrices.deleteSelected', { count: selectedIds.length }) }}
            </button>
            <button class="btn btn-secondary" :disabled="loading" :title="t('common.refresh')" @click="load">
              <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            </button>
            <button class="btn btn-primary" @click="openCreate">
              <Icon name="plus" size="sm" class="mr-1" />
              {{ t('admin.channels.referencePrices.add') }}
            </button>
          </div>
        </div>

        <div v-if="loading && !prices.length" class="p-8 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
        <div v-else-if="!prices.length" class="p-8 text-center text-sm text-gray-500">
          {{ t('admin.channels.referencePrices.empty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="w-10 px-4 py-3">
                  <input
                    type="checkbox"
                    class="checkbox"
                    :checked="currentPageFullySelected"
                    :disabled="!prices.length || deleting"
                    :aria-label="t('admin.channels.referencePrices.selectCurrentPage')"
                    @change="toggleCurrentPage(($event.target as HTMLInputElement).checked)"
                  />
                </th>
                <th class="px-4 py-3">{{ t('admin.channels.referencePrices.platform') }}</th>
                <th class="px-4 py-3">{{ t('admin.channels.referencePrices.model') }}</th>
                <th class="px-4 py-3">{{ t('admin.channels.referencePrices.prices') }}</th>
                <th class="px-4 py-3">{{ t('admin.channels.referencePrices.source') }}</th>
                <th class="px-4 py-3">{{ t('admin.channels.referencePrices.updated') }}</th>
                <th class="px-4 py-3">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="item in prices" :key="item.id">
                <td class="px-4 py-3">
                  <input
                    type="checkbox"
                    class="checkbox"
                    :value="item.id"
                    :checked="selectedIds.includes(item.id)"
                    :disabled="deleting"
                    :aria-label="t('admin.channels.referencePrices.selectModel', { model: item.model_name })"
                    @change="toggleRow(item.id, ($event.target as HTMLInputElement).checked)"
                  />
                </td>
                <td class="whitespace-nowrap px-4 py-3 font-medium">{{ item.platform }}</td>
                <td class="whitespace-nowrap px-4 py-3 font-mono text-xs">{{ item.model_name }}</td>
                <td class="min-w-64 px-4 py-3 text-xs text-gray-600 dark:text-gray-300">{{ priceSummary(item.pricing) }}</td>
                <td class="px-4 py-3">
                  <a v-if="item.source_url" :href="item.source_url" target="_blank" rel="noopener noreferrer" class="text-primary-600 hover:underline dark:text-primary-400">{{ item.source_label }}</a>
                  <span v-else>{{ item.source_label }}</span>
                </td>
                <td class="whitespace-nowrap px-4 py-3 text-xs text-gray-500">{{ formatDate(item.updated_at) }}</td>
                <td class="whitespace-nowrap px-4 py-3">
                  <button class="btn btn-secondary btn-sm mr-2" @click="openEdit(item)">{{ t('common.edit') }}</button>
                  <button class="btn btn-danger btn-sm" @click="remove(item)">{{ t('common.delete') }}</button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
        <div v-if="selectedIds.length" class="border-t border-gray-200 px-4 py-2 text-sm text-gray-600 dark:border-dark-700 dark:text-gray-300">
          {{ t('admin.channels.referencePrices.selectedCount', { count: selectedIds.length }) }}
        </div>
        <Pagination
          v-if="pagination.total > 0"
          :page="pagination.page"
          :total="pagination.total"
          :page-size="pagination.page_size"
          @update:page="handlePageChange"
          @update:pageSize="handlePageSizeChange"
        />
      </section>
    </div>

    <div v-if="catalogDialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeCatalogDialog">
      <section class="max-h-[90vh] w-full max-w-5xl overflow-hidden rounded-lg bg-white shadow-xl dark:bg-dark-800">
        <div class="flex items-start justify-between gap-4 border-b border-gray-200 p-5 dark:border-dark-700">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.channels.referencePrices.importCatalog') }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.channels.referencePrices.catalogDescription') }}</p>
          </div>
          <button class="btn btn-secondary btn-sm" :disabled="catalogSaving" @click="closeCatalogDialog">{{ t('common.close') }}</button>
        </div>

        <div class="flex flex-col gap-3 border-b border-gray-200 px-5 py-3 dark:border-dark-700 sm:flex-row sm:items-center sm:justify-between">
          <div class="flex w-full flex-col gap-2 sm:w-auto sm:flex-row">
            <select v-model="catalogPlatform" class="input sm:w-48">
              <option value="">{{ t('admin.channels.referencePrices.allPlatforms') }}</option>
              <option v-for="platform in catalogPlatforms" :key="platform" :value="platform">{{ platform }}</option>
            </select>
            <div class="relative w-full sm:max-w-sm">
              <Icon name="search" size="sm" class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400" />
              <input v-model="catalogQuery" class="input pl-9" :placeholder="t('admin.channels.referencePrices.catalogSearch')" />
            </div>
          </div>
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
            <input v-model="overwriteExisting" type="checkbox" class="checkbox" />
            {{ t('admin.channels.referencePrices.overwriteExisting') }}
          </label>
        </div>

        <div class="max-h-[55vh] overflow-auto">
          <div v-if="!catalogCandidates.length" class="p-8 text-center text-sm text-gray-500">
            {{ t('admin.channels.referencePrices.catalogEmpty') }}
          </div>
          <table v-else class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="sticky top-0 bg-gray-50 text-left text-xs font-medium text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="w-10 px-5 py-3"></th>
                <th class="px-3 py-3">{{ t('admin.channels.referencePrices.platform') }}</th>
                <th class="px-3 py-3">{{ t('admin.channels.referencePrices.model') }}</th>
                <th class="px-3 py-3">{{ t('admin.channels.referencePrices.prices') }}</th>
                <th class="px-3 py-3">{{ t('admin.channels.referencePrices.importState') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="candidate in catalogCandidates" :key="candidate.key">
                <td class="px-5 py-3">
                  <input
                    type="checkbox"
                    class="checkbox"
                    :checked="selectedCatalogKeys.includes(candidate.key)"
                    :disabled="candidate.exists && !overwriteExisting"
                    @change="toggleCatalogCandidate(candidate.key, ($event.target as HTMLInputElement).checked)"
                  />
                </td>
                <td class="whitespace-nowrap px-3 py-3">{{ candidate.platform }}</td>
                <td class="whitespace-nowrap px-3 py-3 font-mono text-xs">{{ candidate.modelName }}</td>
                <td class="min-w-64 px-3 py-3 text-xs text-gray-600 dark:text-gray-300">{{ priceSummary(candidate.pricing) }}</td>
                <td class="whitespace-nowrap px-3 py-3 text-xs" :class="candidate.exists ? 'text-amber-700 dark:text-amber-300' : 'text-gray-500'">
                  {{ candidate.exists ? t('admin.channels.referencePrices.existingReference') : t('admin.channels.referencePrices.newImport') }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <div class="flex items-center justify-between gap-3 border-t border-gray-200 px-5 py-4 dark:border-dark-700">
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.channels.referencePrices.selectedCount', { count: selectedCatalogKeys.length }) }}
          </span>
          <button class="btn btn-primary" :disabled="catalogSaving || !selectedCatalogKeys.length" @click="importCatalogPrices">
            <Icon name="download" size="sm" class="mr-1" />
            {{ catalogSaving ? t('common.saving') : t('admin.channels.referencePrices.confirmImport') }}
          </button>
        </div>
      </section>
    </div>

    <div v-if="dialogOpen" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="closeDialog">
      <form class="max-h-[90vh] w-full max-w-3xl overflow-y-auto rounded-lg bg-white p-5 shadow-xl dark:bg-dark-800" @submit.prevent="save">
        <div class="flex items-start justify-between gap-4">
          <div>
            <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ editingId ? t('admin.channels.referencePrices.edit') : t('admin.channels.referencePrices.add') }}</h2>
            <p class="mt-1 text-xs text-amber-700 dark:text-amber-300">{{ t('admin.channels.referencePrices.displayOnly') }}</p>
          </div>
          <button type="button" class="btn btn-secondary btn-sm" @click="closeDialog">{{ t('common.close') }}</button>
        </div>

        <div class="mt-5 grid gap-4 sm:grid-cols-2">
          <label class="text-sm font-medium">{{ t('admin.channels.referencePrices.platform') }} *
            <input v-model.trim="form.platform" required maxlength="50" class="input mt-1" placeholder="minimax" :readonly="editingId !== null" />
          </label>
          <label class="text-sm font-medium">{{ t('admin.channels.referencePrices.model') }} *
            <input v-model.trim="form.model_name" required maxlength="255" class="input mt-1" placeholder="MiniMax-M2.7" :readonly="editingId !== null" />
          </label>
          <label v-for="field in priceFields" :key="field.key" class="text-sm font-medium">
            {{ t(`admin.channels.referencePrices.fields.${field.key}`) }}
            <span class="font-normal text-gray-500">{{ field.unit === 'million' ? t('admin.channels.referencePrices.unitMillion') : t('admin.channels.referencePrices.unitRequest') }}</span>
            <input v-model="form.prices[field.key]" type="number" min="0" step="any" class="input mt-1" />
          </label>
          <label class="text-sm font-medium">{{ t('admin.channels.referencePrices.source') }} *
            <input v-model.trim="form.source_label" required maxlength="255" class="input mt-1" :placeholder="t('admin.channels.referencePrices.sourcePlaceholder')" />
          </label>
          <label class="text-sm font-medium">{{ t('admin.channels.referencePrices.sourceUrl') }}
            <input v-model.trim="form.source_url" type="url" maxlength="2048" class="input mt-1" placeholder="https://" />
          </label>
          <label class="text-sm font-medium sm:col-span-2">{{ t('admin.channels.referencePrices.notes') }}
            <textarea v-model.trim="form.notes" rows="3" class="input mt-1" maxlength="2000" />
          </label>
        </div>
        <div class="mt-5 flex justify-end gap-2">
          <button type="button" class="btn btn-secondary" :disabled="saving" @click="closeDialog">{{ t('common.cancel') }}</button>
          <button type="submit" class="btn btn-primary" :disabled="saving || !hasPrice">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
        </div>
      </form>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
  import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import modelPlazaReferencePricesAPI, {
  type ModelPlazaReferencePrice,
  type ModelPlazaReferencePricing
} from '@/api/admin/modelPlazaReferencePrices'

type PriceKey = keyof ModelPlazaReferencePricing
const { t, locale } = useI18n()
const appStore = useAppStore()
const prices = ref<ModelPlazaReferencePrice[]>([])
const query = ref('')
const loading = ref(false)
const deleting = ref(false)
const selectedIds = ref<number[]>([])
const pagination = reactive({ page: 1, page_size: 20, total: 0 })
const saving = ref(false)
const dialogOpen = ref(false)
const catalogLoading = ref(false)
const catalogSaving = ref(false)
const catalogDialogOpen = ref(false)
const catalogQuery = ref('')
const catalogPlatform = ref('')
const overwriteExisting = ref(false)
const catalogModels = ref<{ key: string; platform: string; modelName: string; pricing: ModelPlazaReferencePricing }[]>([])
const catalogPlatforms = computed(() => [...new Set(catalogModels.value.map((item) => item.platform))].sort())
const selectedCatalogKeys = ref<string[]>([])
const editingId = ref<number | null>(null)
const priceFields: { key: PriceKey; unit: 'million' | 'request' }[] = [
  { key: 'input_price', unit: 'million' },
  { key: 'output_price', unit: 'million' },
  { key: 'cache_write_price', unit: 'million' },
  { key: 'cache_write_1h_price', unit: 'million' },
  { key: 'cache_read_price', unit: 'million' },
  { key: 'image_input_price', unit: 'million' },
  { key: 'image_output_price', unit: 'million' },
  { key: 'per_request_price', unit: 'request' }
]

const emptyForm = () => ({
  platform: '',
  model_name: '',
  source_label: '',
  source_url: '',
  notes: '',
  prices: Object.fromEntries(priceFields.map(({ key }) => [key, ''])) as Record<PriceKey, string>
})
const form = reactive(emptyForm())
const hasPrice = computed(() => priceFields.some(({ key }) => form.prices[key] !== '' && Number.isFinite(Number(form.prices[key]))))
const currentPageFullySelected = computed(() =>
  prices.value.length > 0 && prices.value.every((item) => selectedIds.value.includes(item.id)))
const catalogCandidates = computed(() => {
  const needle = catalogQuery.value.trim().toLowerCase()
  return catalogModels.value
    .map((item) => ({
      ...item,
      exists: prices.value.some((price) => price.platform.toLowerCase() === item.platform.toLowerCase() &&
        price.model_name.toLowerCase() === item.modelName.toLowerCase())
    }))
    .filter((item) => (!catalogPlatform.value || item.platform === catalogPlatform.value) &&
      (!needle || `${item.platform} ${item.modelName}`.toLowerCase().includes(needle)))
})

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value))
}

function priceSummary(pricing: ModelPlazaReferencePricing) {
  return priceFields
    .filter(({ key }) => pricing[key] != null)
    .map(({ key, unit }) => `${t(`admin.channels.referencePrices.fields.${key}`)} $${(Number(pricing[key]) * (unit === 'million' ? 1_000_000 : 1)).toLocaleString(locale.value, { maximumSignificantDigits: 7 })}`)
    .join(' · ')
}

let loadSequence = 0
async function load() {
  const sequence = ++loadSequence
  loading.value = true
  try {
    const result = await modelPlazaReferencePricesAPI.list({
      page: pagination.page,
      page_size: pagination.page_size,
      search: query.value.trim() || undefined
    })
    if (sequence !== loadSequence) return
    prices.value = result.items
    pagination.total = result.total
    const lastPage = Math.max(1, Math.ceil(result.total / pagination.page_size))
    if (pagination.page > lastPage) {
      pagination.page = lastPage
      await load()
      return
    }
  } catch {
    if (sequence === loadSequence) appStore.showError(t('admin.channels.referencePrices.loadFailed'))
  } finally {
    if (sequence === loadSequence) loading.value = false
  }
}

function handlePageChange(page: number) {
  pagination.page = page
  load()
}

function handlePageSizeChange(pageSize: number) {
  pagination.page_size = pageSize
  pagination.page = 1
  load()
}

function toggleCurrentPage(checked: boolean) {
  const currentPageIds = new Set(prices.value.map((item) => item.id))
  if (!checked) {
    selectedIds.value = selectedIds.value.filter((id) => !currentPageIds.has(id))
    return
  }
  if (selectedIds.value.some((id) => !currentPageIds.has(id)) &&
      selectedIds.value.length + [...currentPageIds].filter((id) => !selectedIds.value.includes(id)).length > 1000) {
    appStore.showError(t('admin.channels.referencePrices.selectionLimit'))
    return
  }
  selectedIds.value = [...new Set([...selectedIds.value, ...currentPageIds])]
}

function toggleRow(id: number, checked: boolean) {
  if (!checked) {
    selectedIds.value = selectedIds.value.filter((selected) => selected !== id)
    return
  }
  if (selectedIds.value.length >= 1000) {
    appStore.showError(t('admin.channels.referencePrices.selectionLimit'))
    return
  }
  selectedIds.value = [...selectedIds.value, id]
}

watch(query, () => {
  pagination.page = 1
  selectedIds.value = []
  load()
})

async function openCatalogImport() {
  catalogLoading.value = true
  try {
    const catalog = await modelPlazaReferencePricesAPI.catalog()
    catalogModels.value = catalog.map((item) => ({
      key: `${item.platform.toLowerCase()}::${item.model_name.toLowerCase()}`,
      platform: item.platform,
      modelName: item.model_name,
      pricing: item.pricing
    })).sort((a, b) =>
      a.platform.localeCompare(b.platform) || a.modelName.localeCompare(b.modelName))
    selectedCatalogKeys.value = catalogModels.value
      .filter((item) => !prices.value.some((price) => price.platform.toLowerCase() === item.platform.toLowerCase() &&
        price.model_name.toLowerCase() === item.modelName.toLowerCase()))
      .map((item) => item.key)
    overwriteExisting.value = false
    catalogQuery.value = ''
    catalogPlatform.value = ''
    catalogDialogOpen.value = true
    if (!catalogModels.value.length) appStore.showError(t('admin.channels.referencePrices.catalogEmpty'))
  } catch {
    appStore.showError(t('admin.channels.referencePrices.catalogLoadFailed'))
  } finally {
    catalogLoading.value = false
  }
}

function toggleCatalogCandidate(key: string, checked: boolean) {
  selectedCatalogKeys.value = checked
    ? [...new Set([...selectedCatalogKeys.value, key])]
    : selectedCatalogKeys.value.filter((selected) => selected !== key)
}

function closeCatalogDialog() {
  if (!catalogSaving.value) catalogDialogOpen.value = false
}

async function importCatalogPrices() {
  catalogSaving.value = true
  try {
    const selected = new Set(selectedCatalogKeys.value)
    const entries = catalogModels.value.filter((item) => selected.has(item.key))
    await modelPlazaReferencePricesAPI.import(entries.map((item) => ({
        platform: item.platform,
        model_name: item.modelName,
        pricing: item.pricing,
        source_label: t('admin.channels.referencePrices.importSourceLabel'),
        source_url: '',
        notes: t('admin.channels.referencePrices.importSourceNotes')
      })))
    catalogDialogOpen.value = false
    appStore.showSuccess(t('admin.channels.referencePrices.importSuccess', { count: entries.length }))
    await load()
  } catch {
    appStore.showError(t('admin.channels.referencePrices.importFailed'))
    await load()
  } finally {
    catalogSaving.value = false
  }
}

function openCreate() {
  editingId.value = null
  Object.assign(form, emptyForm())
  dialogOpen.value = true
}

function openEdit(item: ModelPlazaReferencePrice) {
  editingId.value = item.id
  Object.assign(form, {
    platform: item.platform,
    model_name: item.model_name,
    source_label: item.source_label,
    source_url: item.source_url,
    notes: item.notes,
    prices: Object.fromEntries(priceFields.map(({ key, unit }) => {
      const value = item.pricing[key]
      return [key, value == null ? '' : String(Number(value) * (unit === 'million' ? 1_000_000 : 1))]
    })) as Record<PriceKey, string>
  })
  dialogOpen.value = true
}

function closeDialog() {
  if (!saving.value) dialogOpen.value = false
}

async function save() {
  if (!hasPrice.value) return
  saving.value = true
  try {
    const pricing: ModelPlazaReferencePricing = {}
    for (const { key, unit } of priceFields) {
      const raw = form.prices[key]
      if (raw === '') continue
      const value = Number(raw) / (unit === 'million' ? 1_000_000 : 1)
      if (!Number.isFinite(value) || value < 0) throw new Error('invalid price')
      pricing[key] = value
    }
    await modelPlazaReferencePricesAPI.upsert({
      platform: form.platform,
      model_name: form.model_name,
      source_label: form.source_label,
      source_url: form.source_url,
      notes: form.notes,
      pricing
    })
    appStore.showSuccess(t('admin.channels.referencePrices.saveSuccess'))
    dialogOpen.value = false
    await load()
  } catch {
    appStore.showError(t('admin.channels.referencePrices.saveFailed'))
  } finally {
    saving.value = false
  }
}

async function remove(item: ModelPlazaReferencePrice) {
  if (!window.confirm(t('admin.channels.referencePrices.deleteConfirm', { model: item.model_name }))) return
  try {
    await modelPlazaReferencePricesAPI.remove(item.id)
    selectedIds.value = selectedIds.value.filter((id) => id !== item.id)
    appStore.showSuccess(t('admin.channels.referencePrices.deleteSuccess'))
    await load()
  } catch {
    appStore.showError(t('admin.channels.referencePrices.deleteFailed'))
  }
}

async function removeSelected() {
  if (!selectedIds.value.length || deleting.value) return
  if (!window.confirm(t('admin.channels.referencePrices.deleteSelectedConfirm', { count: selectedIds.value.length }))) return
  deleting.value = true
  const ids = [...selectedIds.value]
  try {
    const result = await modelPlazaReferencePricesAPI.removeMany(ids)
    selectedIds.value = []
    appStore.showSuccess(t('admin.channels.referencePrices.deleteSelectedSuccess', { count: result.deleted_count }))
    await load()
  } catch {
    appStore.showError(t('admin.channels.referencePrices.deleteFailed'))
  } finally {
    deleting.value = false
  }
}

onMounted(load)
</script>
