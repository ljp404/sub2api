import { describe, expect, it, vi } from 'vitest'
import { mountPlaza } from '../plaza'
import markup from '../plaza.html?raw'

describe('custom model plaza provider icons', () => {
  it.each(['CNY', 'USD'])('displays per-second video charges in the configured currency with the video multiplier (%s source)', async (currency) => {
    const host = document.createElement('div')
    const root = host.attachShadow({ mode: 'open' })
    root.innerHTML = markup
    const dialog = root.getElementById('details')
    dialog.close = vi.fn()
    dialog.showModal = vi.fn()
    const dispose = mountPlaza(root, vi.fn().mockResolvedValue({
      groups: [{
        id: 1, name: 'Video', rate_multiplier: 9,
        video_rate_independent: true, video_rate_multiplier: 2,
        models: [{
          name: 'MiniMax-H3', platform: 'minimax',
          pricing: {
            billing_mode: 'video',
            video_task_pricing: {
              version: 1, currency, cny_per_usd: '5',
              price_480p_per_second: '0.04', price_768p_per_second: '0.1',
              upgrade_2k_per_second: '0.15', prompt_polish_price_per_call: '0.05',
              prompt_polish_default_enabled: false,
            },
          },
        }],
      }],
    }), false)
    try {
      await vi.waitFor(() => expect(root.querySelector('.model-card')).not.toBeNull())
      const prices = [...root.querySelectorAll('.model-card .actual')].map((cell) => cell.textContent)
      expect(prices).toEqual(currency === 'CNY'
        ? ['¥0.08', '¥0.2', '¥0.5', '¥0.1']
        : ['$0.08', '$0.2', '$0.5', '$0.1'])
      root.querySelector('.details-button').click()
      const body = root.querySelector('#detail-body')
      expect(body.textContent).toContain('2K / 秒（含基础费）')
      expect(body.textContent).toContain('提示词优化默认关闭')
      expect([...body.querySelectorAll('.actual')].map((cell) => cell.textContent)).toEqual(prices)
      expect(body.textContent).toContain(currency === 'CNY' ? '平台价 / 人民币' : '平台价 / USD')
      expect(root.querySelector('#detail-subtitle').textContent).toContain(currency === 'CNY' ? '人民币（CNY）' : 'USD')
      if (currency === 'CNY') {
        expect(body.textContent).toContain('结算按配置汇率 5 CNY/USD 换算为美元')
      }
      expect(body.textContent).not.toContain('单次请求')
    } finally {
      dispose()
    }
  })

  it('shows a manual per-request reference price for token-billed video models without discount comparison', async () => {
    const host = document.createElement('div')
    const root = host.attachShadow({ mode: 'open' })
    root.innerHTML = markup
    const dialog = root.getElementById('details')
    dialog.close = vi.fn()
    dialog.showModal = vi.fn(() => {
      dialog.open = true
    })
    const fetchPlaza = vi.fn().mockResolvedValue({
      groups: [
        {
          id: 1,
          name: 'Default',
          platform: 'minimax',
          models: [
            {
              name: 'MiniMax-M2.7',
              platform: 'minimax',
              pricing: {
                billing_mode: 'video_token',
                video_token_pricing: {
                  tiers: {
                    standard: { without_video: 7, with_video: 7 },
                  },
                },
              },
              reference_pricing: {
                pricing: { per_request_price: 0.08 },
                source_label: 'Supplier price list',
                source_url: '',
                notes: '',
                updated_at: '2026-11-11T00:00:00Z',
              },
            },
          ],
        },
      ],
    })

    const dispose = mountPlaza(root, fetchPlaza, false)
    try {
      await vi.waitFor(() => {
        expect(root.querySelector('.model-card')).not.toBeNull()
      })
      root.querySelector('.details-button')?.click()
      expect(root.querySelector('#detail-body')?.textContent).toContain(
        '人工维护参考价：$0.08/次',
      )
      expect(root.querySelector('#detail-body')?.textContent).toContain(
        '不同计费单位，不参与折扣比较',
      )
    } finally {
      dispose()
    }
  })

  it.each([
    ['kimi', 'Kimi'],
    ['deepseek', 'DeepSeek'],
    ['minimax', 'MiniMax'],
    ['zhipu', 'Zhipu GLM'],
  ])('renders the %s platform name and icon', async (platform, name) => {
    const host = document.createElement('div')
    const root = host.attachShadow({ mode: 'open' })
    root.innerHTML = markup
    root.getElementById('details').close = vi.fn()
    const fetchPlaza = vi.fn().mockResolvedValue({
      groups: [
        {
          id: 1,
          name: 'Default',
          platform,
          models: [
            {
              name: `${platform}-model`,
              platform,
              pricing: { billing_mode: 'token', input_price: 1e-6 },
            },
          ],
        },
      ],
    })

    const dispose = mountPlaza(root, fetchPlaza, false)

    try {
      await vi.waitFor(() => {
        expect(root.querySelector('.model-card')).not.toBeNull()
      })

      const providerChip = root.querySelector(
        `.chip[data-filter="provider"][data-value="${platform}"]`,
      )
      const cardIcon = root.querySelector('.model-card .brand img')
      const filterIcon = providerChip?.querySelector('img')

      expect(providerChip?.querySelector('span:not(.count)')?.textContent).toBe(
        name,
      )
      expect(cardIcon?.getAttribute('src')).toMatch(
        /^data:image\/svg\+xml,/,
      )
      expect(cardIcon?.getAttribute('alt')).toBe(name)
      expect(filterIcon?.getAttribute('src')).toBe(cardIcon?.getAttribute('src'))
      expect(root.querySelector('.model-card .brand-fallback')).toBeNull()
    } finally {
      dispose()
    }
  })

  it('groups Seedance models under ByteDance and shares the provider icon', async () => {
    const host = document.createElement('div')
    const root = host.attachShadow({ mode: 'open' })
    root.innerHTML = markup
    root.getElementById('details').close = vi.fn()
    const fetchPlaza = vi.fn().mockResolvedValue({
      groups: [
        {
          id: 1,
          name: 'Default',
          models: [
            {
              name: 'seedance-2.0',
              platform: 'openai',
              pricing: { billing_mode: 'video_token' },
            },
            {
              name: 'seedance-2.0-fast',
              platform: 'openai',
              pricing: { billing_mode: 'video_token' },
            },
            {
              name: 'gpt-image-2.5',
              platform: 'openai',
              pricing: { billing_mode: 'token' },
            },
          ],
        },
      ],
    })

    const dispose = mountPlaza(root, fetchPlaza, false)

    try {
      await vi.waitFor(() => {
        expect(root.querySelectorAll('.model-card')).toHaveLength(3)
      })

      const providerChip = root.querySelector(
        '.chip[data-filter="provider"][data-value="doubao"]',
      )
      const seedanceCard = root.querySelector(
        '.model-card[data-model="seedance-2.0"]',
      )
      const cardIcon = seedanceCard?.querySelector('.brand img')
      const openAiChip = root.querySelector(
        '.chip[data-filter="provider"][data-value="openai"]',
      )
      const openAiCardIcon = root
        .querySelector('.model-card[data-model="gpt-image-2.5"] .brand img')

      expect(providerChip?.querySelector('span:not(.count)')?.textContent).toBe(
        '字节跳动',
      )
      expect(providerChip?.querySelector('.count')?.textContent).toBe('2')
      expect(providerChip?.querySelector('img')?.getAttribute('src')).toBe(
        cardIcon?.getAttribute('src'),
      )
      expect(cardIcon?.getAttribute('alt')).toBe('字节跳动')
      const iconSvg = decodeURIComponent(cardIcon?.getAttribute('src')?.split(',')[1] ?? '')
      for (const color of ['#1E37FC', '#37E1BE', '#A569FF']) {
        expect(iconSvg).toContain(`fill="${color}"`)
      }

      expect(openAiChip?.querySelector('span:not(.count)')?.textContent).toBe(
        'OpenAI',
      )
      expect(openAiChip?.querySelector('.count')?.textContent).toBe('1')
      expect(openAiChip?.querySelector('img')?.getAttribute('src')).toBe(
        openAiCardIcon?.getAttribute('src'),
      )

      providerChip?.click()
      expect(root.querySelectorAll('.model-card')).toHaveLength(2)
      expect(root.querySelector('.model-card[data-model="gpt-image-2.5"]')).toBeNull()
    } finally {
      dispose()
    }
  })

  it('renders the Anthropic icon on model cards and provider filters', async () => {
    const host = document.createElement('div')
    const root = host.attachShadow({ mode: 'open' })
    root.innerHTML = markup
    root.getElementById('details').close = vi.fn()
    const fetchPlaza = vi.fn().mockResolvedValue({
      groups: [
        {
          id: 1,
          name: 'Default',
          models: [
            {
              name: 'claude-sonnet-4',
              platform: 'anthropic',
              pricing: {
                billing_mode: 'token',
                input_price: 3e-6,
                output_price: 15e-6,
                cache_read_price: 0.3e-6,
                intervals: [],
              },
              official_pricing: null,
            },
          ],
        },
      ],
    })

    const dispose = mountPlaza(root, fetchPlaza, false)

    try {
      expect(root.querySelector('.source-note')).toBeNull()
      expect(root.querySelector('.page-foot')).toBeNull()

      await vi.waitFor(() => {
        expect(fetchPlaza).toHaveBeenCalledOnce()
        expect(root.querySelector('.model-card')).not.toBeNull()
      })

      const cardIcon = root.querySelector('.model-card .brand img')
      const filterIcon = root.querySelector(
        '.chip[data-filter="provider"][data-value="anthropic"] img',
      )

      expect(cardIcon?.getAttribute('src')).toMatch(
        /^data:image\/svg\+xml(?:;base64)?,/,
      )
      expect(cardIcon?.getAttribute('alt')).toBe('Anthropic')
      expect(filterIcon?.getAttribute('src')).toBe(cardIcon?.getAttribute('src'))
      const iconSvg = decodeURIComponent(
        cardIcon?.getAttribute('src')?.split(',')[1] ?? '',
      )
      expect(iconSvg).toContain('fill="#D97757"')
      expect(iconSvg).toContain('<path d="m3.127 10.604')
      expect(root.querySelector('.model-card .brand-fallback')).toBeNull()
    } finally {
      dispose()
    }
  })
})
