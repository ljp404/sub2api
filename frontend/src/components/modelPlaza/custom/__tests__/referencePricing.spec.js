import { describe, expect, it } from 'vitest'
import { referencePricingSegments } from '../referencePricing'

const reference = {
  input_price: 2.5e-6,
  output_price: 15e-6,
  cache_read_price: 0.25e-6,
  intervals: [
    { min_tokens: 0, max_tokens: 272000 },
    { min_tokens: 272000, max_tokens: null, input_multiplier: 2, output_multiplier: 1.5 },
  ],
}

describe('referencePricingSegments', () => {
  it('resolves a platform tier within a larger reference tier', () => {
    expect(referencePricingSegments({ min_tokens: 0, max_tokens: 272 }, reference))
      .toMatchObject([{ min_tokens: 0, max_tokens: 272, reference: { input_price: 2.5e-6 } }])
  })

  it('splits a platform tier that crosses reference thresholds', () => {
    const segments = referencePricingSegments({ min_tokens: 272, max_tokens: null }, reference)
    expect(segments).toHaveLength(2)
    expect(segments).toMatchObject([
      { min_tokens: 272, max_tokens: 272000, reference: { input_price: 2.5e-6, output_price: 15e-6 } },
      { min_tokens: 272000, max_tokens: null, reference: { input_price: 5e-6, output_price: 22.5e-6 } },
    ])
  })

  it('uses the higher reference tier at the shared boundary', () => {
    expect(referencePricingSegments({ min_tokens: 272000, max_tokens: null }, reference))
      .toMatchObject([{ min_tokens: 272000, max_tokens: null, reference: { input_price: 5e-6 } }])
  })

  it('uses flat reference prices for all platform tiers', () => {
    const flat = { input_price: 2.5e-6, output_price: 15e-6 }
    expect(referencePricingSegments({ min_tokens: 272, max_tokens: null }, flat))
      .toEqual([{ min_tokens: 272, max_tokens: null, reference: flat }])
  })

  it('keeps missing references missing', () => {
    expect(referencePricingSegments({ min_tokens: 0, max_tokens: null }, null))
      .toEqual([{ min_tokens: 0, max_tokens: null, reference: null }])
  })

  it('does not invent reference prices in uncovered ranges', () => {
    const partial = { ...reference, intervals: [{ min_tokens: 100, max_tokens: 200, input_price: 0 }] }
    expect(referencePricingSegments({ min_tokens: 0, max_tokens: 300 }, partial))
      .toMatchObject([
        { min_tokens: 0, max_tokens: 100, reference: null },
        { min_tokens: 100, max_tokens: 200, reference: { input_price: 0 } },
        { min_tokens: 200, max_tokens: 300, reference: null },
      ])
  })
})
