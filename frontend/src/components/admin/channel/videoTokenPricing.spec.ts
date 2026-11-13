import { describe, expect, it } from 'vitest'
import { supplierVideoTokenPricing, videoTokenPricingVersion } from './videoTokenPricing'

describe('video token pricing version', () => {
  it('defaults to standard when no pricing is provided', () => {
    expect(videoTokenPricingVersion()).toBe('standard')
    expect(videoTokenPricingVersion(null)).toBe('standard')
    expect(videoTokenPricingVersion({ ...supplierVideoTokenPricing(), tiers: {} })).toBe('standard')
  })

  it.each([false, true])('restores the version from saved tiers, fast=%s', fast => {
    const saved = JSON.parse(JSON.stringify(supplierVideoTokenPricing(fast)))
    expect(videoTokenPricingVersion(saved)).toBe(fast ? 'fast' : 'standard')
  })

  it('preserves the version after customizing prices and pixel dimensions', () => {
    const rule = supplierVideoTokenPricing(true)
    rule.tiers['fast:720p'].without_video = '6.123456'
    rule.tiers['fast:720p'].sizes['16:9'].width = 1232
    const before = JSON.stringify(rule)
    expect(videoTokenPricingVersion(rule)).toBe('fast')
    expect(JSON.stringify(rule)).toBe(before)
  })

  it('does not depend on tier order or a complete preset', () => {
    const rule = supplierVideoTokenPricing()
    rule.tiers = { '1080p': rule.tiers['1080p'], '480p': rule.tiers['480p'] }
    expect(videoTokenPricingVersion(rule)).toBe('standard')
    delete rule.tiers['480p']
    expect(videoTokenPricingVersion(rule)).toBe('standard')
  })

  it('does not incorrectly mark mixed tariffs as one version', () => {
    const rule = supplierVideoTokenPricing()
    Object.assign(rule.tiers, supplierVideoTokenPricing(true).tiers)
    expect(videoTokenPricingVersion(rule)).toBeNull()
  })
})
