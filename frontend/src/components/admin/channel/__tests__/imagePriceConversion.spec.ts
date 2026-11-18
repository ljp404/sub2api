import { describe, expect, it } from 'vitest'
import { convertCNYToUSD } from '../imagePriceConversion'

describe('image price currency conversion', () => {
  it('divides CNY by the CNY-per-USD rate without rounding to cents', () => {
    expect(convertCNYToUSD('0.35', '7')).toBe(0.05)
    expect(convertCNYToUSD('1', '7')).toBe(0.1428571429)
    expect(convertCNYToUSD(0, 7)).toBe(0)
  })

  it.each([
    ['', 7], [1, ''], [' ', 7], [1, 0], [1, -7], [-1, 7],
    ['bad', 7], [1, 'bad'], [Infinity, 7], [1, Infinity], [1e-12, 7],
  ])('rejects missing or invalid conversion inputs (%s, %s)', (price, rate) => {
    expect(convertCNYToUSD(price, rate)).toBeNull()
  })
})
