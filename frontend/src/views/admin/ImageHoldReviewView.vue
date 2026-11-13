<template>
  <AppLayout>
    <div class="space-y-5">
      <div>
        <h1 class="text-xl font-semibold text-gray-900 dark:text-white">
          {{ t('admin.usage.imageHolds.title') }}
        </h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.usage.imageHolds.description') }}
        </p>
      </div>

      <div class="card overflow-hidden">
        <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <span class="text-sm text-gray-600 dark:text-gray-300">
            {{ t('admin.usage.imageHolds.pendingCount', { count: total }) }}
          </span>
          <button class="btn btn-secondary" :disabled="loading" @click="load">
            <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
            {{ t('common.refresh') }}
          </button>
        </div>

        <div v-if="loading && !holds.length" class="p-8 text-center text-sm text-gray-500">
          {{ t('common.loading') }}
        </div>
        <div v-else-if="!holds.length" class="p-8 text-center text-sm text-gray-500">
          {{ t('admin.usage.imageHolds.empty') }}
        </div>
        <div v-else class="overflow-x-auto">
          <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
            <thead class="bg-gray-50 text-left text-xs font-medium uppercase text-gray-500 dark:bg-dark-800 dark:text-gray-400">
              <tr>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.holdId') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.userId') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.apiKeyId') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.accountId') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.amount') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.dimensions') }}</th>
                <th class="px-4 py-3">{{ t('admin.usage.imageHolds.createdAt') }}</th>
                <th class="px-4 py-3">{{ t('common.actions') }}</th>
              </tr>
            </thead>
            <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
              <tr v-for="hold in holds" :key="hold.id">
                <td class="whitespace-nowrap px-4 py-3 font-mono text-xs">{{ hold.id }}</td>
                <td class="px-4 py-3">{{ hold.user_id }}</td>
                <td class="px-4 py-3">{{ hold.api_key_id }}</td>
                <td class="px-4 py-3">{{ hold.account_id }}</td>
                <td class="whitespace-nowrap px-4 py-3 font-medium">{{ hold.amount }} USD</td>
                <td class="px-4 py-3">
                  <span v-if="Object.keys(hold.dimensions || {}).length">
                    {{ Object.entries(hold.dimensions).map(([key, value]) => `${key}: ${value}`).join(', ') }}
                  </span>
                  <span v-else class="text-gray-400">-</span>
                </td>
                <td class="whitespace-nowrap px-4 py-3">{{ formatDate(hold.created_at) }}</td>
                <td class="whitespace-nowrap px-4 py-3">
                  <button class="btn btn-danger btn-sm" @click="openRelease(hold)">
                    {{ t('admin.usage.imageHolds.release') }}
                  </button>
                </td>
              </tr>
            </tbody>
          </table>
        </div>

        <Pagination
          v-if="total > pageSize"
          :page="page"
          :total="total"
          :page-size="pageSize"
          @update:page="changePage"
        />
      </div>
    </div>

    <div v-if="selected" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4">
      <div class="w-full max-w-lg rounded-lg bg-white p-5 shadow-xl dark:bg-dark-800">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">
          {{ t('admin.usage.imageHolds.confirmTitle') }}
        </h2>
        <p class="mt-2 text-sm text-amber-700 dark:text-amber-300">
          {{ t('admin.usage.imageHolds.confirmWarning', { amount: selected.amount, id: selected.id }) }}
        </p>
        <label class="mt-4 block text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.usage.imageHolds.reason') }}
          <textarea
            v-model="reason"
            rows="3"
            maxlength="1000"
            class="input mt-1 w-full"
            :placeholder="t('admin.usage.imageHolds.reasonPlaceholder')"
          />
        </label>
        <div class="mt-4 flex justify-end gap-2">
          <button class="btn btn-secondary" :disabled="releasing" @click="selected = null">
            {{ t('common.cancel') }}
          </button>
          <button class="btn btn-danger" :disabled="releasing || !reason.trim()" @click="release">
            {{ releasing ? t('common.submitting') : t('admin.usage.imageHolds.confirmRelease') }}
          </button>
        </div>
      </div>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import { useAppStore } from '@/stores/app'
import { adminUsageAPI, type UncertainImageHold } from '@/api/admin/usage'
import type { PaginatedResponse } from '@/types'

const { t, locale } = useI18n()
const appStore = useAppStore()
const holds = ref<UncertainImageHold[]>([])
const selected = ref<UncertainImageHold | null>(null)
const reason = ref('')
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const loading = ref(false)
const releasing = ref(false)

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'medium', timeStyle: 'medium' }).format(new Date(value))
}

async function load() {
  loading.value = true
  try {
    const result: PaginatedResponse<UncertainImageHold> = await adminUsageAPI.listUncertainImageHolds({
      page: page.value,
      page_size: pageSize.value
    })
    holds.value = result.items
    total.value = result.total
  } catch {
    appStore.showError(t('admin.usage.imageHolds.loadFailed'))
  } finally {
    loading.value = false
  }
}

function changePage(value: number) {
  page.value = value
  void load()
}

function openRelease(hold: UncertainImageHold) {
  selected.value = hold
  reason.value = ''
}

async function release() {
  if (!selected.value || !reason.value.trim()) return
  releasing.value = true
  try {
    await adminUsageAPI.releaseUncertainImageHold(selected.value.id, reason.value.trim())
    appStore.showSuccess(t('admin.usage.imageHolds.releaseSuccess'))
    selected.value = null
    await load()
  } catch {
    appStore.showError(t('admin.usage.imageHolds.releaseFailed'))
  } finally {
    releasing.value = false
  }
}

onMounted(load)
</script>
