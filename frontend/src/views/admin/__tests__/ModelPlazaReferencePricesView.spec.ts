import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'
import ModelPlazaReferencePricesView from '../ModelPlazaReferencePricesView.vue'

const api = vi.hoisted(() => ({
  list: vi.fn(),
  upsert: vi.fn(),
  import: vi.fn(),
  remove: vi.fn(),
  removeMany: vi.fn(),
  catalog: vi.fn()
}))
const appStore = vi.hoisted(() => ({ showError: vi.fn(), showSuccess: vi.fn() }))

vi.mock('@/api/admin/modelPlazaReferencePrices', () => ({ default: api }))
vi.mock('@/stores/app', () => ({ useAppStore: () => appStore }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en' } })
}))

const rows = [
  {
    id: 7,
    platform: 'openai',
    model_name: 'gpt-image-2',
    pricing: { per_request_price: 0.04 },
    source_label: 'Official',
    source_url: '',
    notes: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  },
  {
    id: 8,
    platform: 'minimax',
    model_name: 'MiniMax-H3',
    pricing: { input_price: 0.000001 },
    source_label: 'Official',
    source_url: '',
    notes: '',
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z'
  }
]

const PaginationStub = {
  props: ['page', 'total', 'pageSize'],
  emits: ['update:page', 'update:pageSize'],
  template: '<button data-testid="next-page" @click="$emit(\'update:page\', page + 1)">next</button>'
}

enableAutoUnmount(afterEach)

beforeEach(() => {
  vi.clearAllMocks()
  api.list.mockResolvedValue({ items: rows, total: 42, page: 1, page_size: 20, pages: 3 })
  api.removeMany.mockResolvedValue({ deleted_count: 2 })
})

function mountView() {
  return mount(ModelPlazaReferencePricesView, {
    global: {
      stubs: {
        AppLayout: { template: '<div><slot /></div>' },
        Icon: true,
        Pagination: PaginationStub
      }
    }
  })
}

describe('model plaza reference price management', () => {
  it('requests the selected page and page size', async () => {
    const wrapper = mountView()
    await flushPromises()
    await wrapper.get('[data-testid="next-page"]').trigger('click')
    await flushPromises()

    expect(api.list).toHaveBeenLastCalledWith({ page: 2, page_size: 20, search: undefined })
  })

  it('submits selected rows to bulk delete and reloads the list', async () => {
    vi.stubGlobal('confirm', vi.fn(() => true))
    const wrapper = mountView()
    await flushPromises()
    const rowCheckboxes = wrapper.findAll('tbody input[type="checkbox"]')
    await rowCheckboxes[0].setValue(true)
    await rowCheckboxes[1].setValue(true)
    await wrapper.get('button.btn-danger').trigger('click')
    await flushPromises()

    expect(api.removeMany).toHaveBeenCalledWith([7, 8])
    expect(api.list).toHaveBeenCalledTimes(2)
    expect(appStore.showSuccess).toHaveBeenCalledWith('admin.channels.referencePrices.deleteSelectedSuccess')
  })
})
