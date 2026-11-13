import type { VideoTokenPricing } from '@/api/admin/channels'

export function videoTokenPricingVersion(rule?: VideoTokenPricing | null): 'standard' | 'fast' | null {
  const tiers = Object.keys(rule?.tiers || {})
  if (tiers.length === 0 || tiers.every(name => !name.startsWith('fast:'))) return 'standard'
  if (tiers.every(name => name.startsWith('fast:'))) return 'fast'
  return null
}

export function supplierVideoTokenPricing(fast = false): VideoTokenPricing {
  const rates = fast
    ? [['fast:480p', '5.6', '3.3', 854, 480], ['fast:720p', '5.6', '3.3', 1280, 720]] as const
    : [['480p', '7', '4.3', 854, 480], ['720p', '7', '4.3', 1280, 720],
      ['1080p', '7.7', '4.7', 1920, 1080], ['4k', '4', '2.4', 3840, 2160]] as const
  return {
    version: 1, fps: 24, divisor: 1024, adaptive_seconds: 15, max_input_seconds: 15,
    tiers: Object.fromEntries(rates.map(([name, without_video, with_video, width, height]) => [
      name, { without_video, with_video, sizes: {
        '16:9': { width, height }, '9:16': { width: height, height: width },
        '1:1': { width: height, height },
      } },
    ])),
  }
}
