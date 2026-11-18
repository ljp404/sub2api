export function convertCNYToUSD(price: number | string, cnyPerUsd: number | string): number | null {
  if (String(price).trim() === '' || String(cnyPerUsd).trim() === '') return null
  const amount = Number(price)
  const rate = Number(cnyPerUsd)
  if (!Number.isFinite(amount) || !Number.isFinite(rate) || amount < 0 || rate <= 0) return null
  const converted = Number((amount / rate).toFixed(10))
  if (!Number.isFinite(converted) || (amount > 0 && converted === 0)) return null
  return converted
}
