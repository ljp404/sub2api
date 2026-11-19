import { resolveIntervalPrices } from '@/utils/pricing'

// Split display ranges at reference boundaries without changing platform prices.
export function referencePricingSegments(tier, reference) {
  const end = tier.max_tokens ?? Infinity
  const referenceTiers = reference?.intervals || []
  const boundaries = [...new Set([
    tier.min_tokens,
    ...referenceTiers.flatMap((entry) => [entry.min_tokens, entry.max_tokens])
      .filter((value) => value != null && value > tier.min_tokens && value < end),
    end,
  ])].sort((a, b) => a - b)
  return boundaries.slice(0, -1).map((start, index) => {
    const match = referenceTiers.find((entry) =>
      entry.min_tokens <= start && (entry.max_tokens ?? Infinity) > start,
    )
    return {
      min_tokens: start,
      max_tokens: boundaries[index + 1] === Infinity ? null : boundaries[index + 1],
      reference: reference == null ? null : match
        ? { ...reference, ...resolveIntervalPrices(match, reference) }
        : referenceTiers.length ? null : reference,
    }
  })
}
