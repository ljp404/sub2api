import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'

const { listImageHolds, releaseUncertainImageHold } = vi.hoisted(() => ({
  listImageHolds: vi.fn(),
  releaseUncertainImageHold: vi.fn()
}))

vi.mock('@/api/admin/usage', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/api/admin/usage')>()
  return {
    ...actual,
    adminUsageAPI: { ...actual.adminUsageAPI, listImageHolds, releaseUncertainImageHold }
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({
    showError: vi.fn(),
    showSuccess: vi.fn()
  })
}))

vi.mock('vue-i18n', async (importOriginal) => {
  const actual = await importOriginal<typeof import('vue-i18n')>()
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
      locale: { value: 'en' }
    })
  }
})

import ImageHoldReviewView from '../ImageHoldReviewView.vue'

describe('ImageHoldReviewView', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    listImageHolds.mockResolvedValue({
      items: [
        {
          id: 'submitted-hold',
          user_id: 1,
          api_key_id: 2,
          account_id: 3,
          amount: '0.05000000',
          state: 'submitted',
          dimensions: {},
          created_at: '2026-11-16T16:23:51+08:00'
        },
        {
          id: 'uncertain-hold',
          user_id: 1,
          api_key_id: 2,
          account_id: 3,
          amount: '0.10000000',
          state: 'uncertain',
          dimensions: {},
          created_at: '2026-11-16T16:20:00+08:00'
        }
      ],
      total: 2,
      page: 1,
      page_size: 20
    })
  })

  it('shows submitted holds for investigation without offering release', async () => {
    const wrapper = mount(ImageHoldReviewView, {
      global: {
        stubs: {
          AppLayout: { template: '<div><slot /></div>' },
          Icon: true,
          Pagination: true
        }
      }
    })
    await flushPromises()

    const rows = wrapper.findAll('tbody tr')
    expect(rows).toHaveLength(2)
    expect(rows[0].text()).toContain('admin.usage.imageHolds.states.submitted')
    expect(rows[0].text()).toContain('admin.usage.imageHolds.investigate')
    expect(rows[0].find('button').exists()).toBe(false)
    expect(rows[1].text()).toContain('admin.usage.imageHolds.states.uncertain')
    expect(rows[1].find('button').text()).toBe('admin.usage.imageHolds.release')
  })
})
