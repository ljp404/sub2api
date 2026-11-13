import type { ParameterPricing } from '@/api/admin/channels'

export function emptyParameterPricing(): ParameterPricing {
  return { version: 1, unit: 'image', base_price: '', factors: [] }
}

export function cloneParameterPricing(rule?: ParameterPricing | null): ParameterPricing | null {
  return rule ? JSON.parse(JSON.stringify(rule)) as ParameterPricing : null
}

// Model versions use separate pricing cards. Defaults must match the
// upstream's actual behavior.
export function supplierImagePricing(): ParameterPricing {
  return {
    version: 1, unit: 'image', base_price: '0.052706',
    factors: [
      { key: 'quality', values: { high: '1', low: '0.111338', medium: '0.250028', xhigh: '1.777797', max: '3.999944' } },
      { key: 'resolution', values: { '1k': '1', '2k': '2.030577', '4k': '3.374175' } },
      { key: 'aspect_ratio', values: {
        '16:27': '0.505055', '16:9': '0.479421', '1:1': '1', '21:9': '0.324019',
        '27:16': '0.505055', '2:3': '0.590655', '3:2': '0.590655', '3:4': '0.682363',
        '4:3': '0.682363', '4:5': '0.740465', '5:4': '0.740465', '8:9': '0.850296',
        '9:16': '0.479421', '9:8': '0.850296', auto: '1',
      } },
    ],
  }
}
