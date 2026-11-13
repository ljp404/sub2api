import { describe, expect, it } from 'vitest'
import { cloneParameterPricing, supplierImagePricing } from './parameterPricing'

describe('supplier image pricing defaults', () => {
  it('prices separate models without a variant factor', () => {
    const rule = supplierImagePricing()
    expect(rule.factors.map(factor => factor.key)).toEqual(['quality', 'resolution', 'aspect_ratio'])
    expect(rule.base_price).toBe('0.052706')
    const other = cloneParameterPricing(rule)!
    other.base_price = '0.1'
    other.factors[0].values.high = '2'
    expect(rule.base_price).toBe('0.052706')
    expect(rule.factors[0].values.high).toBe('1')
    expect(supplierImagePricing()).toEqual(rule)
  })

  it('preserves legacy variant rules when editing a saved rule', () => {
    const rule = supplierImagePricing()
    rule.factors.push({ key: 'variant', default: 'sunburst', values: { flare: '1', sunburst: '2' } })
    expect(cloneParameterPricing(rule)?.factors.at(-1)).toEqual(rule.factors.at(-1))
  })
})
