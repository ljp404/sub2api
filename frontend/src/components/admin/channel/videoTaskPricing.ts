import type { VideoTaskPricing } from '@/api/admin/channels'

export function emptyVideoTaskPricing(): VideoTaskPricing {
  return {
    version: 1,
    currency: 'CNY',
    cny_per_usd: '',
    price_480p_per_second: '',
    price_768p_per_second: '',
    upgrade_2k_per_second: '',
    prompt_polish_price_per_call: '',
    prompt_polish_default_enabled: false,
  }
}

export function cloneVideoTaskPricing(rule?: VideoTaskPricing | null): VideoTaskPricing | null {
  return rule ? JSON.parse(JSON.stringify(rule)) as VideoTaskPricing : null
}
