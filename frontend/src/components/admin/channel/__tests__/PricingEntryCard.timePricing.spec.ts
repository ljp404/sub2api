import { flushPromises, shallowMount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import PricingEntryCard from '../PricingEntryCard.vue'
import type { PricingFormEntry } from '../types'
import channelsAPI from '@/api/admin/channels'

vi.mock('@/api/admin/channels', () => ({
  default: { getModelDefaultPricing: vi.fn() },
}))

vi.mock('vue-i18n', async importOriginal => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))

function createEntry(billingMode: PricingFormEntry['billing_mode'] = 'token'): PricingFormEntry {
  return {
    models: [],
    billing_mode: billingMode,
    input_price: null,
    output_price: null,
    cache_write_price: null,
    cache_read_price: null,
    fast_multiplier: null,
    flex_multiplier: null,
    reasoning_effort_multipliers: null,
    image_input_price: null,
    image_output_price: null,
    per_request_price: null,
    intervals: [],
    time_pricing: {
      timezone: 'Asia/Shanghai',
      weekdays_only: false,
      periods: [{ start_time: '09:00', end_time: '12:00', multiplier: '2.00' }],
    },
  }
}

describe('PricingEntryCard time pricing visibility', () => {
  it('is hidden by default', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry() },
    })

    expect(wrapper.findComponent({ name: 'TimePricingSection' }).exists()).toBe(false)
  })

  it('is shown for token pricing when explicitly enabled', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), enableTimePricing: true },
    })

    expect(wrapper.findComponent({ name: 'TimePricingSection' }).exists()).toBe(true)
  })

  it('is hidden for non-token pricing even when explicitly enabled', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry('per_request'), enableTimePricing: true },
    })

    expect(wrapper.findComponent({ name: 'TimePricingSection' }).exists()).toBe(false)
  })

  it('clears time periods when changing billing mode', () => {
    const entry = createEntry()
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry, enableTimePricing: true },
    })

    wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', 'image')

    expect(wrapper.emitted('update')?.[0]?.[0]).toEqual({
      ...entry,
      billing_mode: 'image',
      parameter_pricing: null,
      video_token_pricing: null,
      intervals: [],
      video_task_pricing: null,
      time_pricing: { timezone: 'Asia/Shanghai', weekdays_only: false, periods: [] },
    })
    expect(entry.time_pricing.periods).toHaveLength(1)
  })
})

describe('PricingEntryCard video task pricing', () => {
  it('offers the optional task-price editor without replacing legacy video pricing', async () => {
    const entry = createEntry('video')
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry },
    })

    expect(wrapper.findComponent({ name: 'VideoTaskPricingEditor' }).exists()).toBe(false)
    await wrapper.findAll('button')[1].trigger('click')

    const enabled = wrapper.emitted('update')![0][0] as PricingFormEntry
    expect(enabled.billing_mode).toBe('video')
    expect(enabled.video_task_pricing).toMatchObject({
      version: 1,
      currency: 'CNY',
      prompt_polish_default_enabled: false,
    })
    expect(enabled.per_request_price).toBe(entry.per_request_price)
  })

  it('renders the task-price editor when a rule is configured', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: {
        entry: {
          ...createEntry('video'),
          video_task_pricing: {
            version: 1,
            currency: 'CNY',
            cny_per_usd: '7.2',
            price_480p_per_second: '0.04',
            price_768p_per_second: '0.10',
            upgrade_2k_per_second: '0.15',
            prompt_polish_price_per_call: '0.05',
            prompt_polish_default_enabled: true,
          },
        },
      },
    })

    expect(wrapper.findComponent({ name: 'VideoTaskPricingEditor' }).exists()).toBe(true)
  })
})

describe('PricingEntryCard media pricing platforms', () => {
  it.each(['openai', 'composite'])('offers both media rules for %s when enabled', platform => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), platform, enableParameterPricing: true },
    })
    expect(wrapper.findComponent({ name: 'Select' }).props('options').map((option: { value: string }) => option.value))
      .toEqual(['token', 'per_request', 'image', 'video', 'parameter', 'video_token'])
    wrapper.unmount()
  })

  it('offers image parameter pricing, but not video token pricing, for Gemini', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), platform: 'gemini', enableParameterPricing: true },
    })
    expect(wrapper.findComponent({ name: 'Select' }).props('options').map((option: { value: string }) => option.value))
      .toEqual(['token', 'per_request', 'image', 'video', 'parameter'])
    wrapper.unmount()
  })

  it.each([
    { platform: 'composite', enabled: false },
    { platform: 'anthropic', enabled: true },
  ])('keeps media rules unavailable for $platform with enabled=$enabled', ({ platform, enabled }) => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), platform, enableParameterPricing: enabled },
    })
    expect(wrapper.findComponent({ name: 'Select' }).props('options').map((option: { value: string }) => option.value))
      .toEqual(['token', 'per_request', 'image', 'video'])
    wrapper.unmount()
  })

  it.each(['parameter', 'video_token'] as const)('initializes and reopens the %s editor for composite groups', async mode => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), platform: 'composite', enableParameterPricing: true },
    })
    wrapper.findComponent({ name: 'Select' }).vm.$emit('update:modelValue', mode)
    const updated = wrapper.emitted('update')![0][0] as PricingFormEntry
    expect(updated.billing_mode).toBe(mode)
    expect(updated[mode === 'parameter' ? 'parameter_pricing' : 'video_token_pricing']).toBeTruthy()
    await wrapper.setProps({ entry: updated })
    expect(wrapper.findComponent({ name: mode === 'parameter' ? 'ParameterPricingEditor' : 'VideoTokenPricingEditor' }).exists()).toBe(true)
    expect(wrapper.findComponent({ name: 'Select' }).props('modelValue')).toBe(mode)
    wrapper.unmount()
  })
})

describe('PricingEntryCard request multipliers', () => {
  it('shows Fast and Flex controls only when explicitly enabled', () => {
    const hidden = shallowMount(PricingEntryCard, { props: { entry: createEntry() } })
    expect(hidden.text()).not.toContain('admin.channels.form.fastMultiplier')

    const shown = shallowMount(PricingEntryCard, {
      props: { entry: createEntry(), enableTierMultipliers: true },
    })
    expect(shown.text()).toContain('admin.channels.form.fastMultiplier')
    expect(shown.text()).toContain('admin.channels.form.flexMultiplier')
    expect(shown.text()).toContain('admin.channels.form.reasoningEffortMultipliers')
  })

  it.each(['token', 'per_request', 'image', 'video'] as const)('supports every reasoning level for %s pricing and account statistics', billingMode => {
    const wrapper = shallowMount(PricingEntryCard, { props: { entry: createEntry(billingMode) } })
    expect(wrapper.findAll('[data-reasoning-effort]').map(input => input.attributes('data-reasoning-effort')))
      .toEqual(['none', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'])
  })

  it('edits levels independently, then clears individual and all overrides', async () => {
    const entry = { ...createEntry(), reasoning_effort_multipliers: { high: 1.5, max: 3 } }
    const wrapper = shallowMount(PricingEntryCard, { props: { entry } })
    const update = () => wrapper.emitted('update')!.at(-1)![0] as PricingFormEntry

    await wrapper.get('[data-reasoning-effort="high"]').setValue('0.75')
    expect(update().reasoning_effort_multipliers).toEqual({ high: '0.75', max: 3 })
    expect(entry.reasoning_effort_multipliers.high).toBe(1.5)
    await wrapper.setProps({ entry: update() })

    await wrapper.get('[data-reasoning-effort="max"]').setValue('')
    expect(update().reasoning_effort_multipliers).toEqual({ high: '0.75' })
    await wrapper.setProps({ entry: update() })

    await wrapper.get('[data-testid="reasoning-effort-multipliers"] button').trigger('click')
    expect(update().reasoning_effort_multipliers).toBeNull()
    await wrapper.setProps({ entry: update() })
    expect(wrapper.get<HTMLInputElement>('[data-reasoning-effort="high"]').element.value).toBe('')
  })

  it('clearing the last configured level restores default billing', async () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: { ...createEntry(), reasoning_effort_multipliers: { max: 3 } } },
    })
    await wrapper.get('[data-reasoning-effort="max"]').setValue('')
    expect((wrapper.emitted('update')![0][0] as PricingFormEntry).reasoning_effort_multipliers).toBeNull()
  })

  it('marks invalid multipliers and shows a validation error', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: { ...createEntry(), reasoning_effort_multipliers: { high: 0, max: 2 } } },
    })
    expect(wrapper.get('[data-reasoning-effort="high"]').attributes('aria-invalid')).toBe('true')
    expect(wrapper.get('[data-reasoning-effort="max"]').attributes('aria-invalid')).toBe('false')
    expect(wrapper.get('[role="alert"]').text()).toContain('reasoningEffortMultiplierPositive')
  })

  it('does not assign a model-specific multiplier to Fable', () => {
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: { ...createEntry(), models: ['claude-fable-5-1'] } },
    })
    expect(wrapper.get<HTMLInputElement>('[data-reasoning-effort="max"]').element.value).toBe('')
    expect(wrapper.get('[data-reasoning-effort="max"]').attributes('placeholder'))
      .toBe('admin.channels.form.reasoningEffortMultiplierDefault')
  })

  it('keeps custom effort multipliers when auto-filling model token prices', async () => {
    vi.mocked(channelsAPI.getModelDefaultPricing).mockResolvedValue({
      found: true, input_price: 3e-6, output_price: 15e-6,
    })
    const wrapper = shallowMount(PricingEntryCard, {
      props: { entry: { ...createEntry(), reasoning_effort_multipliers: { high: 0.5 } } },
    })
    wrapper.findComponent({ name: 'ModelTagInput' }).vm.$emit('update:models', ['example-model'])
    await flushPromises()
    expect(wrapper.emitted('update')!.at(-1)![0]).toMatchObject({
      models: ['example-model'], input_price: 3, output_price: 15,
      reasoning_effort_multipliers: { high: 0.5 },
    })
  })
})
