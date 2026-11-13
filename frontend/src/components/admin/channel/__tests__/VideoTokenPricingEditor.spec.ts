import { shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import VideoTokenPricingEditor from '../VideoTokenPricingEditor.vue'
import { supplierVideoTokenPricing } from '../videoTokenPricing'

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

describe('VideoTokenPricingEditor', () => {
  it('allows removing an unsupported tier such as 4k', async () => {
    const wrapper = shallowMount(VideoTokenPricingEditor, {
      props: { modelValue: supplierVideoTokenPricing() },
    })

    await wrapper.get('[aria-label="admin.channels.videoToken.removeTier 4k"]').trigger('click')

    const updated = wrapper.emitted('update:modelValue')![0][0]
    expect(Object.keys(updated.tiers)).not.toContain('4k')
    expect(Object.keys(updated.tiers)).toContain('1080p')
  })

  it('keeps at least one price tier', () => {
    const rule = supplierVideoTokenPricing()
    rule.tiers = { '480p': rule.tiers['480p'] }
    const wrapper = shallowMount(VideoTokenPricingEditor, { props: { modelValue: rule } })

    expect(wrapper.get('[aria-label="admin.channels.videoToken.removeTier 480p"]').attributes('disabled')).toBeDefined()
  })
})
