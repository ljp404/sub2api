import { marked } from 'marked'
import DOMPurify from 'dompurify'
import { formatScaled, resolveIntervalPrices } from '@/utils/pricing'

export function mountPlaza(root, fetchPlaza, authenticated) {
  const lifecycle = new AbortController()
  let disposed = false
  const CONFIG = {
    // Display-only labels; billing rules always come from the API.
    modelTags: {
      gpt_image_2: '图片',
      'gpt-5.5': '文本',
      'gpt-5.6': '文本',
      'gpt-5.6-luna': '文本',
      'gpt-5.6-terra': '文本',
      'gpt-6-astra': '文本',
      'gpt-6.1-sol': '文本',
    },
  }
  const ASSETS = {
    search:
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtc2VhcmNoIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8Y2lyY2xlIGN4PSIxMSIgY3k9IjExIiByPSI4IiAvPgogIDxwYXRoIGQ9Im0yMSAyMS00LjMtNC4zIiAvPgo8L3N2Zz4K',
    x: 'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUteCIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTE4IDYgNiAxOCIgLz4KICA8cGF0aCBkPSJtNiA2IDEyIDEyIiAvPgo8L3N2Zz4K',
    'sliders-horizontal':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtc2xpZGVycy1ob3Jpem9udGFsIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8bGluZSB4MT0iMjEiIHgyPSIxNCIgeTE9IjQiIHkyPSI0IiAvPgogIDxsaW5lIHgxPSIxMCIgeDI9IjMiIHkxPSI0IiB5Mj0iNCIgLz4KICA8bGluZSB4MT0iMjEiIHgyPSIxMiIgeTE9IjEyIiB5Mj0iMTIiIC8+CiAgPGxpbmUgeDE9IjgiIHgyPSIzIiB5MT0iMTIiIHkyPSIxMiIgLz4KICA8bGluZSB4MT0iMjEiIHgyPSIxNiIgeTE9IjIwIiB5Mj0iMjAiIC8+CiAgPGxpbmUgeDE9IjEyIiB4Mj0iMyIgeTE9IjIwIiB5Mj0iMjAiIC8+CiAgPGxpbmUgeDE9IjE0IiB4Mj0iMTQiIHkxPSIyIiB5Mj0iNiIgLz4KICA8bGluZSB4MT0iOCIgeDI9IjgiIHkxPSIxMCIgeTI9IjE0IiAvPgogIDxsaW5lIHgxPSIxNiIgeDI9IjE2IiB5MT0iMTgiIHkyPSIyMiIgLz4KPC9zdmc+Cg==',
    copy: 'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtY29weSIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHJlY3Qgd2lkdGg9IjE0IiBoZWlnaHQ9IjE0IiB4PSI4IiB5PSI4IiByeD0iMiIgcnk9IjIiIC8+CiAgPHBhdGggZD0iTTQgMTZjLTEuMSAwLTItLjktMi0yVjRjMC0xLjEuOS0yIDItMmgxMGMxLjEgMCAyIC45IDIgMiIgLz4KPC9zdmc+Cg==',
    'layout-grid':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtbGF5b3V0LWdyaWQiCiAgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIgogIHdpZHRoPSIyNCIKICBoZWlnaHQ9IjI0IgogIHZpZXdCb3g9IjAgMCAyNCAyNCIKICBmaWxsPSJub25lIgogIHN0cm9rZT0iY3VycmVudENvbG9yIgogIHN0cm9rZS13aWR0aD0iMiIKICBzdHJva2UtbGluZWNhcD0icm91bmQiCiAgc3Ryb2tlLWxpbmVqb2luPSJyb3VuZCIKPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjMiIHk9IjMiIHJ4PSIxIiAvPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjE0IiB5PSIzIiByeD0iMSIgLz4KICA8cmVjdCB3aWR0aD0iNyIgaGVpZ2h0PSI3IiB4PSIxNCIgeT0iMTQiIHJ4PSIxIiAvPgogIDxyZWN0IHdpZHRoPSI3IiBoZWlnaHQ9IjciIHg9IjMiIHk9IjE0IiByeD0iMSIgLz4KPC9zdmc+Cg==',
    'table-2':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtdGFibGUtMiIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTkgM0g1YTIgMiAwIDAgMC0yIDJ2NG02LTZoMTBhMiAyIDAgMCAxIDIgMnY0TTkgM3YxOG0wIDBoMTBhMiAyIDAgMCAwIDItMlY5TTkgMjFINWEyIDIgMCAwIDEtMi0yVjltMCAwaDE4IiAvPgo8L3N2Zz4K',
    'refresh-cw':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtcmVmcmVzaC1jdyIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0iTTMgMTJhOSA5IDAgMCAxIDktOSA5Ljc1IDkuNzUgMCAwIDEgNi43NCAyLjc0TDIxIDgiIC8+CiAgPHBhdGggZD0iTTIxIDN2NWgtNSIgLz4KICA8cGF0aCBkPSJNMjEgMTJhOSA5IDAgMCAxLTkgOSA5Ljc1IDkuNzUgMCAwIDEtNi43NC0yLjc0TDMgMTYiIC8+CiAgPHBhdGggZD0iTTggMTZIM3Y1IiAvPgo8L3N2Zz4K',
    'chevron-right':
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtY2hldnJvbi1yaWdodCIKICB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciCiAgd2lkdGg9IjI0IgogIGhlaWdodD0iMjQiCiAgdmlld0JveD0iMCAwIDI0IDI0IgogIGZpbGw9Im5vbmUiCiAgc3Ryb2tlPSJjdXJyZW50Q29sb3IiCiAgc3Ryb2tlLXdpZHRoPSIyIgogIHN0cm9rZS1saW5lY2FwPSJyb3VuZCIKICBzdHJva2UtbGluZWpvaW49InJvdW5kIgo+CiAgPHBhdGggZD0ibTkgMTggNi02LTYtNiIgLz4KPC9zdmc+Cg==',
    unplug:
      'data:image/svg+xml;base64,PCEtLSBAbGljZW5zZSBsdWNpZGUtc3RhdGljIHYwLjQ2OC4wIC0gSVNDIC0tPgo8c3ZnCiAgY2xhc3M9Imx1Y2lkZSBsdWNpZGUtdW5wbHVnIgogIHhtbG5zPSJodHRwOi8vd3d3LnczLm9yZy8yMDAwL3N2ZyIKICB3aWR0aD0iMjQiCiAgaGVpZ2h0PSIyNCIKICB2aWV3Qm94PSIwIDAgMjQgMjQiCiAgZmlsbD0ibm9uZSIKICBzdHJva2U9ImN1cnJlbnRDb2xvciIKICBzdHJva2Utd2lkdGg9IjIiCiAgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIgogIHN0cm9rZS1saW5lam9pbj0icm91bmQiCj4KICA8cGF0aCBkPSJtMTkgNSAzLTMiIC8+CiAgPHBhdGggZD0ibTIgMjIgMy0zIiAvPgogIDxwYXRoIGQ9Ik02LjMgMjAuM2EyLjQgMi40IDAgMCAwIDMuNCAwTDEyIDE4bC02LTYtMi4zIDIuM2EyLjQgMi40IDAgMCAwIDAgMy40WiIgLz4KICA8cGF0aCBkPSJNNy41IDEzLjUgMTAgMTEiIC8+CiAgPHBhdGggZD0iTTEwLjUgMTYuNSAxMyAxNCIgLz4KICA8cGF0aCBkPSJtMTIgNiA2IDYgMi4zLTIuM2EyLjQgMi40IDAgMCAwIDAtMy40bC0yLjYtMi42YTIuNCAyLjQgMCAwIDAtMy40IDBaIiAvPgo8L3N2Zz4K',
    openai:
      'data:image/svg+xml;base64,PHN2ZyBmaWxsPSJjdXJyZW50Q29sb3IiIGZpbGwtcnVsZT0iZXZlbm9kZCIgaGVpZ2h0PSIxZW0iIHN0eWxlPSJmbGV4Om5vbmU7bGluZS1oZWlnaHQ6MSIgdmlld0JveD0iMCAwIDI0IDI0IiB3aWR0aD0iMWVtIiB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjx0aXRsZT5PcGVuQUk8L3RpdGxlPjxwYXRoIGQ9Ik05LjIwNSA4LjY1OHYtMi4yNmMwLS4xOS4wNzItLjMzMy4yMzgtLjQyOGw0LjU0My0yLjYxNmMuNjE5LS4zNTcgMS4zNTYtLjUyMyAyLjExNy0uNTIzIDIuODU0IDAgNC42NjIgMi4yMTIgNC42NjIgNC41NjYgMCAuMTY3IDAgLjM1Ny0uMDI0LjU0N2wtNC43MS0yLjc1OWEuNzk3Ljc5NyAwIDAwLS44NTYgMGwtNS45NyAzLjQ3M3ptMTAuNjA5IDguOFYxMi4wNmMwLS4zMzMtLjE0My0uNTctLjQyOS0uNzM3bC01Ljk3LTMuNDczIDEuOTUtMS4xMThhLjQzMy40MzMgMCAwMS40NzYgMGw0LjU0MyAyLjYxN2MxLjMwOS43NiAyLjE4OSAyLjM3OCAyLjE4OSAzLjk0OCAwIDEuODA4LTEuMDcgMy40NzMtMi43NiA0LjE2M3pNNy44MDIgMTIuNzAzbC0xLjk1LTEuMTQyYy0uMTY3LS4wOTUtLjIzOS0uMjM4LS4yMzktLjQyOFY1Ljg5OWMwLTIuNTQ1IDEuOTUtNC40NzIgNC41OTEtNC40NzIgMSAwIDEuOTI3LjMzMyAyLjcxMi45MjhMOC4yMyA1LjA2N2MtLjI4NS4xNjYtLjQyOC40MDQtLjQyOC43Mzd2Ni44OTh6TTEyIDE1LjEyOGwtMi43OTUtMS41N3YtMy4zM0wxMiA4LjY1OGwyLjc5NSAxLjU3djMuMzNMMTIgMTUuMTI4em0xLjc5NiA3LjIzYy0xIDAtMS45MjctLjMzMi0yLjcxMi0uOTI3bDQuNjg2LTIuNzEyYy4yODUtLjE2Ni40MjgtLjQwNC40MjgtLjczN3YtNi44OThsMS45NzQgMS4xNDJjLjE2Ny4wOTUuMjM4LjIzOC4yMzguNDI4djUuMjMzYzAgMi41NDUtMS45NzQgNC40NzItNC42MTQgNC40NzJ6bS01LjYzNy01LjMwM2wtNC41NDQtMi42MTdjLTEuMzA4LS43NjEtMi4xODgtMi4zNzgtMi4xODgtMy45NDhBNC40ODIgNC40ODIgMCAwMTQuMjEgNi4zMjd2NS40MjNjMCAuMzMzLjE0My41NzEuNDI4LjczOGw1Ljk0NyAzLjQ0OS0xLjk1IDEuMTE4YS40MzIuNDMyIDAgMDEtLjQ3NiAwem0tLjI2MiAzLjljLTIuNjg4IDAtNC42NjItMi4wMjEtNC42NjItNC41MTkgMC0uMTkuMDI0LS4zOC4wNDctLjU3bDQuNjg2IDIuNzFjLjI4Ni4xNjcuNTcxLjE2Ny44NTYgMGw1Ljk3LTMuNDQ4djIuMjZjMCAuMTktLjA3LjMzMy0uMjM3LjQyOGwtNC41NDMgMi42MTZjLS42MTkuMzU3LTEuMzU2LjUyMy0yLjExNy41MjN6bTUuODk5IDIuODNhNS45NDcgNS45NDcgMCAwMDUuODI3LTQuNzU2QzIyLjI4NyAxOC4zMzkgMjQgMTUuODQgMjQgMTMuMjk2YzAtMS42NjUtLjcxMy0zLjI4Mi0xLjk5OC00LjQ0OC4xMTktLjUuMTktLjk5OS4xOS0xLjQ5OCAwLTMuNDAxLTIuNzU5LTUuOTQ3LTUuOTQ2LTUuOTQ3LS42NDIgMC0xLjI2LjA5NS0xLjg4LjMxQTUuOTYyIDUuOTYyIDAgMDAxMC4yMDUgMGE1Ljk0NyA1Ljk0NyAwIDAwLTUuODI3IDQuNzU3QzEuNzEzIDUuNDQ3IDAgNy45NDUgMCAxMC40OWMwIDEuNjY2LjcxMyAzLjI4MyAxLjk5OCA0LjQ0OC0uMTE5LjUtLjE5IDEtLjE5IDEuNDk5IDAgMy40MDEgMi43NTkgNS45NDYgNS45NDYgNS45NDYuNjQyIDAgMS4yNi0uMDk1IDEuODgtLjMwOWE1Ljk2IDUuOTYgMCAwMDQuMTYyIDEuNzEzeiI+PC9wYXRoPjwvc3ZnPg==',
  }
  const $ = (id) => root.getElementById(id)
  const escapeHTML = (value) =>
    String(value ?? '').replace(
      /[&<>"']/g,
      (c) =>
        ({
          '&': '&amp;',
          '<': '&lt;',
          '>': '&gt;',
          '"': '&quot;',
          "'": '&#39;',
        })[c],
    )
  const icon = (name) => `<img src="${ASSETS[name] || ''}" alt="">`
  root
    .querySelectorAll('[data-icon]')
    .forEach((el) => (el.src = ASSETS[el.dataset.icon]))
  $('anonymous-hint').hidden = authenticated
  const state = {
    groups: [],
    models: [],
    provider: '',
    billing: '',
    tag: '',
    group: '',
    query: '',
    unit: 1000000,
    compare: true,
    view: 'grid',
    sort: 'default',
    selected: new Set(),
    visible: [],
    loading: false,
    error: '',
  }
  const providerNames = {
    openai: 'OpenAI',
    anthropic: 'Anthropic',
    gemini: 'Google',
    google: 'Google',
    deepseek: 'DeepSeek',
  }
  const billingNames = {
    token: '按量计费',
    request: '按次计费',
    image: '按张计费',
    video: '视频计费',
  }
  const rows = [
    ['input_price', '输入'],
    ['output_price', '输出'],
    ['cache_read_price', '缓存读取'],
    ['cache_write_price', '缓存写入 / 5m'],
    ['cache_write_1h_price', '缓存写入 / 1h'],
    ['image_input_price', '图片输入'],
    ['image_output_price', '图片输出'],
    ['per_request_price', '单次请求'],
  ]
  const mode = (model) => model.pricing?.billing_mode || 'token'
  const baseMultiplier = (group) =>
    group.user_rate_multiplier ?? group.rate_multiplier ?? 1
  function multiplier(variant, key) {
    const g = variant.group,
      m = mode(variant.model)
    if (
      key === 'per_request_price' &&
      m === 'image' &&
      g.image_rate_independent
    )
      return g.image_rate_multiplier ?? 1
    if (
      key === 'per_request_price' &&
      m === 'video' &&
      g.video_rate_independent
    )
      return Math.max(0, g.video_rate_multiplier ?? 1)
    return baseMultiplier(g)
  }
  function basePricing(model) {
    const base = model.pricing
    const first = [...(base?.intervals || [])].sort(
      (a, b) => a.min_tokens - b.min_tokens,
    )[0]
    return base && first
      ? {
          ...base,
          ...resolveIntervalPrices(first, base),
          per_request_price: first.per_request_price ?? base.per_request_price,
        }
      : base
  }
  function price(variant, key, pricing = basePricing(variant.model)) {
    const value = pricing?.[key]
    return typeof value === 'number' && Number.isFinite(value)
      ? value * multiplier(variant, key)
      : null
  }
  function money(value, key = 'input_price') {
    if (value == null || !Number.isFinite(value)) return '—'
    return formatScaled(value, key === 'per_request_price' ? 1 : state.unit)
  }
  function discount(actual, reference) {
    if (actual == null || reference == null || reference <= 0) return '—'
    const ratio = actual / reference
    if (Math.abs(ratio - 1) < 1e-6)
      return '<span class="discount full">原价</span>'
    return `<span class="discount ${ratio > 1 ? 'premium' : ''}">${Number((ratio * 10).toFixed(2))}折</span>`
  }
  function brand(platform) {
    return `<span class="brand">${platform === 'openai' ? `<img src="${ASSETS.openai}" alt="OpenAI">` : `<span class="brand-fallback">${escapeHTML((providerNames[platform] || platform || '?').slice(0, 2))}</span>`}</span>`
  }
  function selectedVariants(item) {
    return item.variants.filter(
      (v) =>
        (!state.group || String(v.group.id) === state.group) &&
        (!state.billing || mode(v.model) === state.billing),
    )
  }
  function representative(item) {
    return selectedVariants(item)[0]
  }
  function filtered() {
    return state.models
      .filter(
        (item) =>
          (!state.provider || item.platform === state.provider) &&
          (!state.tag || CONFIG.modelTags[item.name] === state.tag) &&
          item.name.toLowerCase().includes(state.query.toLowerCase().trim()) &&
          selectedVariants(item).some(
            (v) => !state.billing || mode(v.model) === state.billing,
          ),
      )
      .sort((a, b) => {
        if (state.sort === 'name') return a.name.localeCompare(b.name)
        if (state.sort.startsWith('price-')) {
          const av = representative(a),
            bv = representative(b)
          const x =
              mode(av.model) === 'token' ? price(av, 'input_price') : null,
            y = mode(bv.model) === 'token' ? price(bv, 'input_price') : null
          if (x == null) return y == null ? 0 : 1
          if (y == null) return -1
          return state.sort === 'price-asc' ? x - y : y - x
        }
        return a.order - b.order
      })
  }
  function chip(value, label, active, type, count, logo = false) {
    return `<button class="chip ${active === value ? 'active' : ''}" data-filter="${type}" data-value="${escapeHTML(value)}" aria-pressed="${active === value}">${logo ? `<img src="${ASSETS.openai}" alt="">` : ''}<span>${escapeHTML(label)}</span>${count == null ? '' : `<span class="count">${count}</span>`}</button>`
  }
  function renderFilters() {
    const providers = [...new Set(state.models.map((m) => m.platform))]
    $('providers').innerHTML =
      chip('', '全部模型', state.provider, 'provider') +
      providers
        .map((p) =>
          chip(
            p,
            providerNames[p] || p,
            state.provider,
            'provider',
            null,
            p === 'openai',
          ),
        )
        .join('')
    const modes = [
      ...new Set(
        state.models.flatMap((m) => m.variants.map((v) => mode(v.model))),
      ),
    ]
    $('billing').innerHTML =
      chip('', '全部类型', state.billing, 'billing') +
      modes
        .map((m) => chip(m, billingNames[m] || m, state.billing, 'billing'))
        .join('')
    const tags = [
      ...new Set(
        state.models.map((m) => CONFIG.modelTags[m.name]).filter(Boolean),
      ),
    ]
    $('tags').innerHTML =
      chip('', '全部标签', state.tag, 'tag') +
      tags.map((t) => chip(t, t, state.tag, 'tag')).join('')
    $('groups').innerHTML =
      chip('', '全部分组', state.group, 'group', state.groups.length) +
      state.groups
        .map((g) =>
          chip(String(g.id), g.name, state.group, 'group', g.models.length),
        )
        .join('')
    $('group-count').textContent = `${state.groups.length} 个`
  }
  function priceTable(
    variant,
    pricing = basePricing(variant.model),
    reference = variant.model.official_pricing,
    detailed = false,
  ) {
    const hasToken = mode(variant.model) === 'token'
    // Official catalog prices are per token; request/media prices have a different unit.
    if (!hasToken) reference = null
    let displayRows = hasToken
      ? detailed
        ? rows.filter(
            ([key]) =>
              key !== 'per_request_price' &&
              (pricing?.[key] != null || reference?.[key] != null),
          )
        : rows.slice(0, 3)
      : rows.filter(([key]) => key === 'per_request_price')
    if (!displayRows.length) return '<div class="detail-note">暂无价格</div>'
    const suffix = hasToken
      ? `/${state.unit === 1000000 ? 'M' : 'K'}`
      : mode(variant.model) === 'image'
        ? '/张'
        : '/次'
    return `<div class="price-box"><table class="prices ${detailed ? 'detail-prices' : ''}"><thead><tr><th scope="col">价格项</th><th scope="col">平台价${suffix}</th>${state.compare ? `<th scope="col">参考价${suffix}</th><th scope="col">折扣</th>` : ''}</tr></thead><tbody>${displayRows
      .map(([key, label]) => {
        const actual = price(variant, key, pricing),
          ref = reference?.[key]
        return `<tr data-price="${key}"><td>${escapeHTML(label)}</td><td class="actual">${money(actual, key)}</td>${state.compare ? `<td class="reference ${actual != null && ref != null && actual < ref ? 'discounted' : ''}">${money(ref, key)}</td><td>${discount(actual, ref)}</td>` : ''}</tr>`
      })
      .join('')}</tbody></table></div>`
  }
  function card(item, index) {
    const variants = selectedVariants(item),
      v = representative(item),
      selected = state.selected.has(item.key)
    const intervals =
      v.group.long_context_pricing_enabled === false
        ? []
        : v.model.pricing?.intervals || []
    const groupLabel = state.group ? v.group.name : `${variants.length} 个分组`
    return `<article class="model-card ${selected ? 'selected' : ''}" data-model="${escapeHTML(item.name)}">
    <div class="card-head">${brand(item.platform)}<div class="card-identity"><h2>${escapeHTML(item.name)}</h2><div class="badges"><span class="badge group-badge" title="${escapeHTML(variants.map((v) => v.group.name).join(' / '))}">${escapeHTML(groupLabel)}</span>${intervals.length > 1 ? '<span class="badge tier-badge">阶梯定价</span>' : '<span class="badge tier-badge">标准定价</span>'}</div></div><div class="card-actions"><button class="icon-button quiet" data-copy="${index}" title="复制模型名称" aria-label="复制 ${escapeHTML(item.name)}">${icon('copy')}</button><input class="model-check" type="checkbox" data-select="${index}" ${selected ? 'checked' : ''} aria-label="选择 ${escapeHTML(item.name)}"></div></div>
    ${priceTable(v)}
    <div class="base-note">${state.group ? '当前分组' : `展示分组：${escapeHTML(v.group.name)}`} · 标准时段${intervals.length > 1 ? ' / 首档' : ''}</div>
    <div class="card-footer"><div class="badges"><span class="tag">${escapeHTML(billingNames[mode(v.model)] || mode(v.model))}</span>${CONFIG.modelTags[item.name] ? `<span class="tag content">${escapeHTML(CONFIG.modelTags[item.name])}</span>` : ''}</div><button class="details-button" data-detail="${index}">详情 ${icon('chevron-right')}</button></div></article>`
  }
  function listTable(items) {
    const unit = state.unit === 1000000 ? 'M' : 'K'
    return `<div class="table-wrap"><table class="model-table"><thead><tr><th></th><th>模型</th><th>分组</th><th>输入 /${unit}</th><th>输出 /${unit}</th><th>缓存读取 /${unit}</th><th>按次 / 媒体</th><th>操作</th></tr></thead><tbody>${items
      .map((item, i) => {
        const v = representative(item)
        return `<tr><td><input type="checkbox" class="model-check" data-select="${i}" ${state.selected.has(item.key) ? 'checked' : ''} aria-label="选择 ${escapeHTML(item.name)}"></td><td class="name">${escapeHTML(item.name)}</td><td>${escapeHTML(v.group.name)}</td>${['input_price', 'output_price', 'cache_read_price'].map((key) => `<td class="money">${mode(v.model) === 'token' ? money(price(v, key), key) : '—'}</td>`).join('')}<td class="money">${mode(v.model) === 'token' ? '—' : `${money(price(v, 'per_request_price'), 'per_request_price')}/${mode(v.model) === 'image' ? '张' : '次'}`}</td><td><button class="details-button" data-detail="${i}">详情 ${icon('chevron-right')}</button></td></tr>`
      })
      .join('')}</tbody></table></div>`
  }
  function updateSelection() {
    const visible = state.visible,
      checked = visible.filter((m) => state.selected.has(m.key)).length
    $('select-all').checked = visible.length > 0 && checked === visible.length
    $('select-all').indeterminate = checked > 0 && checked < visible.length
    $('select-all').disabled = !visible.length
    $('copy-selected').disabled = state.selected.size === 0
    $('copy-selected').querySelector('span').textContent = state.selected.size
      ? `复制 (${state.selected.size})`
      : '复制'
  }
  function render() {
    state.visible = state.loading || state.error ? [] : filtered()
    $('results').className = state.view === 'grid' ? 'grid' : ''
    $('results').setAttribute('aria-busy', String(state.loading))
    $('model-total').textContent = `共 ${state.models.length} 个模型`
    $('page-title').textContent = state.provider
      ? providerNames[state.provider] || state.provider
      : '全部供应商'
    $('result-count').innerHTML = state.loading
      ? '正在获取模型…'
      : state.error
        ? '数据加载失败'
        : `显示 <strong>${state.visible.length}</strong> / ${state.models.length} 个模型`
    $('clear-search').hidden = !state.query
    if (state.loading)
      $('results').innerHTML = Array(6)
        .fill('<div class="skeleton"></div>')
        .join('')
    else if (state.error)
      $('results').innerHTML =
        `<div class="state">${icon('unplug')}<h2>暂时无法获取模型</h2><p>${escapeHTML(state.error)}</p><button data-action="retry">重新加载</button></div>`
    else if (!state.visible.length)
      $('results').innerHTML =
        `<div class="state">${icon('search')}<h2>没有找到匹配的模型</h2><p>试试其他关键词或调整筛选条件。</p><button data-action="reset">重置筛选</button></div>`
    else
      $('results').innerHTML =
        state.view === 'grid'
          ? state.visible.map(card).join('')
          : listTable(state.visible)
    ;['grid', 'table'].forEach((view) => {
      const b = $(view + '-view')
      b.classList.toggle('active', state.view === view)
      b.setAttribute('aria-pressed', String(state.view === view))
    })
    updateSelection()
  }
  let requestId = 0
  async function load() {
    const id = ++requestId
    state.loading = true
    state.error = ''
    render()
    $('sync-status').textContent = '正在同步价格'
    $('refresh').disabled = true
    try {
      const payload = await fetchPlaza()
      if (!Array.isArray(payload?.groups))
        throw new Error('接口数据格式不正确。')
      if (disposed || id !== requestId) return
      $('description').innerHTML = DOMPurify.sanitize(
        marked.parse(payload.description || ''),
      )
      $('description').hidden = !payload.description?.trim()
      const groups = payload.groups
        .filter((g) => Array.isArray(g.models))
        .sort((a, b) => baseMultiplier(a) - baseMultiplier(b) || a.id - b.id)
      const map = new Map()
      groups.forEach((group) =>
        group.models.forEach((model) => {
          if (typeof model.name !== 'string') return
          const platform = model.platform || group.platform || 'unknown',
            key = JSON.stringify([platform, model.name])
          if (!map.has(key))
            map.set(key, {
              key,
              name: model.name,
              platform,
              order: map.size,
              variants: [],
            })
          map.get(key).variants.push({ model, group })
        }),
      )
      state.groups = groups
      state.models = [...map.values()]
      state.selected = new Set(
        [...state.selected].filter((key) => map.has(key)),
      )
      if (!groups.some((g) => String(g.id) === state.group)) state.group = ''
      if (!state.models.some((m) => m.platform === state.provider))
        state.provider = ''
      if (!state.models.some((m) => CONFIG.modelTags[m.name] === state.tag))
        state.tag = ''
      if (
        !state.models.some((m) =>
          m.variants.some((v) => mode(v.model) === state.billing),
        )
      )
        state.billing = ''
      renderFilters()
      const now = new Date().toLocaleTimeString('zh-CN', { hour12: false })
      $('sync-status').textContent = `${now} 已同步`
      $('updated').textContent = `价格更新于 ${now} · USD`
    } catch (error) {
      if (disposed || id !== requestId) return
      state.error =
        error.status === 404
          ? '模型广场未开启，请检查后台模型广场设置。'
          : error.code === 'ERR_CANCELED' || error.name === 'TimeoutError'
            ? '接口请求超时，请稍后重试。'
            : '无法获取模型数据，请检查服务连接后重试。'
      $('description').hidden = true
      $('sync-status').textContent = '同步失败'
    } finally {
      if (!disposed && id === requestId) {
        state.loading = false
        $('refresh').disabled = false
        render()
      }
    }
  }
  function resolveTier(tier, base) {
    return {
      ...base,
      ...resolveIntervalPrices(tier, base),
      per_request_price: tier.per_request_price ?? base.per_request_price,
    }
  }
  function tierLabel(tier) {
    return (
      tier.tier_label ||
      (tier.max_tokens == null
        ? `> ${tier.min_tokens.toLocaleString()} tokens`
        : `≤ ${tier.max_tokens.toLocaleString()} tokens`)
    )
  }
  let detailItem = null,
    detailGroup = ''
  function renderDetails() {
    if (!detailItem) return
    const item = detailItem,
      v =
        item.variants.find((v) => String(v.group.id) === detailGroup) ||
        item.variants[0],
      m = v.model,
      g = v.group
    detailGroup = String(g.id)
    $('detail-title').textContent = item.name
    $('detail-subtitle').textContent =
      `${providerNames[item.platform] || item.platform} · USD · ${billingNames[mode(m)] || mode(m)}`
    const platformTiers = m.pricing?.intervals || [],
      officialTiers = m.official_pricing?.intervals || []
    const tiers =
      g.long_context_pricing_enabled === false
        ? []
        : [...platformTiers].sort((a, b) => a.min_tokens - b.min_tokens)
    let html = `<div class="detail-select"><label>可用分组<select id="detail-group" aria-label="详情分组">${item.variants.map((v) => `<option value="${escapeHTML(v.group.id)}" ${v.group.id === g.id ? 'selected' : ''}>${escapeHTML(v.group.name)}</option>`).join('')}</select></label><span class="badge group-badge">分组倍率 ×${escapeHTML(baseMultiplier(g))}</span></div>`
    if (g.description)
      html += `<p class="detail-note">${escapeHTML(g.description)}</p>`
    html += `<h3 style="font-size:13px;margin-bottom:10px">标准时段价格${tiers.length > 1 ? ' · 首档' : ''}</h3>${priceTable(v, basePricing(m), m.official_pricing, true)}`
    const efforts = Object.entries(
      m.pricing?.reasoning_effort_multipliers || {},
    ).filter(
      ([, factor]) =>
        typeof factor === 'number' && Number.isFinite(factor) && factor > 0,
    )
    if (efforts.length)
      html += `<p class="detail-note">推理强度倍率：${efforts.map(([effort, factor]) => `${escapeHTML(effort)} ×${escapeHTML(factor)}`).join(' / ')}。</p>`
    if (mode(m) !== 'token')
      html +=
        '<p class="detail-note">当前按次或媒体计费，官方目录的每 token 参考价与此单位不同，不作折扣比较。</p>'
    if (tiers.length > 1) {
      html += `<p class="detail-note">${m.long_context_basis === 'marginal' ? '按区间分段计费，只有超出阈值的部分按对应档位单价计价。' : '整单按请求所在档位的单价计价。'} 以下价格已计入所选分组倍率。</p>`
      html += tiers
        .map((tier) => {
          const official = officialTiers.find(
            (t) =>
              t.min_tokens === tier.min_tokens &&
              t.max_tokens === tier.max_tokens,
          )
          return `<section class="tier-section"><h3>${escapeHTML(tierLabel(tier))}</h3>${priceTable(v, resolveTier(tier, m.pricing), official ? resolveTier(official, m.official_pricing) : null, true)}</section>`
        })
        .join('')
    } else if (
      g.long_context_pricing_enabled === false &&
      officialTiers.length > 1
    ) {
      html +=
        '<p class="detail-note">此分组未启用平台长上下文阶梯计费。参考价仍可能随上下文档位变化。</p>'
      if (state.compare && mode(m) === 'token')
        html += `<table class="schedule"><thead><tr><th>参考价档位</th><th>输入</th><th>输出</th></tr></thead><tbody>${officialTiers
          .map((t) => {
            const resolved = resolveTier(t, m.official_pricing)
            return `<tr><td>${escapeHTML(tierLabel(t))}</td><td>${money(resolved.input_price)}</td><td>${money(resolved.output_price)}</td></tr>`
          })
          .join('')}</tbody></table>`
    }
    if (g.peak_rate_enabled)
      html += `<p class="detail-note">分组高峰时段：${escapeHTML(g.peak_start)}–${escapeHTML(g.peak_end)}，高峰倍率 ×${escapeHTML(g.peak_rate_multiplier)}。上表展示标准时段价格。</p>`
    const schedule = m.time_pricing
    if (schedule?.periods?.length)
      html += `<section class="tier-section"><h3>分时价格</h3><p class="detail-note">${escapeHTML(schedule.timezone)}${schedule.weekdays_only ? ' · 仅工作日' : ''}，以下时段在标准分组倍率上叠加时段倍率。</p><table class="schedule"><thead><tr><th>时段</th><th>时段倍率</th><th>输入价格</th><th>输出价格</th></tr></thead><tbody>${schedule.periods
        .map((p) => {
          const factor =
            Math.round(baseMultiplier(g) * p.multiplier * 1000) / 1000
          return `<tr><td>${escapeHTML(p.start_time)}–${escapeHTML(p.end_time)}</td><td>×${escapeHTML(p.multiplier)}</td><td>${money(m.pricing?.input_price == null ? null : m.pricing.input_price * factor)}</td><td>${money(m.pricing?.output_price == null ? null : m.pricing.output_price * factor)}</td></tr>`
        })
        .join('')}</tbody></table></section>`
    html +=
      '<p class="detail-note">参考价来自接口返回的定价目录；“—”表示未提供。最终费用以实际调用账单为准。</p>'
    $('detail-body').innerHTML = html
  }
  function openDetails(index) {
    detailItem = state.visible[index]
    detailGroup = String(representative(detailItem).group.id)
    renderDetails()
    $('details').showModal()
  }
  let toastTimer
  function toast(message) {
    $('toast').textContent = message
    $('toast').hidden = false
    clearTimeout(toastTimer)
    toastTimer = setTimeout(() => ($('toast').hidden = true), 2400)
  }
  async function copyText(text) {
    try {
      if (navigator.clipboard && window.isSecureContext)
        await navigator.clipboard.writeText(text)
      else {
        const area = document.createElement('textarea')
        area.value = text
        area.style.cssText = 'position:fixed;opacity:0'
        document.body.append(area)
        area.select()
        const ok = document.execCommand('copy')
        area.remove()
        if (!ok) throw new Error()
      }
      toast('已复制模型名称')
    } catch {
      toast('复制失败，请手动复制模型名称')
    }
  }
  function reset() {
    Object.assign(state, {
      provider: '',
      billing: '',
      tag: '',
      group: '',
      query: '',
      sort: 'default',
    })
    $('search').value = ''
    $('sort').value = 'default'
    renderFilters()
    render()
  }
  function setFiltersOpen(open) {
    $('sidebar').classList.toggle('open', open)
    $('backdrop').classList.toggle('open', open)
  }
  root.addEventListener(
    'click',
    (event) => {
      const b = event.target.closest('button')
      if (!b) return
      if (b.dataset.filter) {
        state[b.dataset.filter] = b.dataset.value
        renderFilters()
        render()
      }
      if (b.dataset.copy != null)
        copyText(state.visible[Number(b.dataset.copy)].name)
      if (b.dataset.detail != null) openDetails(Number(b.dataset.detail))
      if (b.dataset.action === 'reset') reset()
      if (b.dataset.action === 'retry') load()
      if (b.dataset.action === 'close-detail') $('details').close()
      if (b.dataset.action === 'open-filters') setFiltersOpen(true)
      if (b.dataset.action === 'close-filters') setFiltersOpen(false)
    },
    { signal: lifecycle.signal },
  )
  root.addEventListener(
    'change',
    (event) => {
      const el = event.target
      if (el.dataset.select != null) {
        const item = state.visible[Number(el.dataset.select)]
        if (el.checked) state.selected.add(item.key)
        else state.selected.delete(item.key)
        el.closest('.model-card')?.classList.toggle('selected', el.checked)
        updateSelection()
      }
      if (el.id === 'detail-group') {
        detailGroup = el.value
        renderDetails()
      }
    },
    { signal: lifecycle.signal },
  )
  $('search').addEventListener('input', (event) => {
    state.query = event.target.value
    render()
  })
  $('clear-search').onclick = () => {
    state.query = ''
    $('search').value = ''
    render()
    $('search').focus()
  }
  $('reset').onclick = reset
  $('backdrop').onclick = () => setFiltersOpen(false)
  $('unit').onchange = (event) => {
    state.unit = Number(event.target.value)
    render()
    if ($('details').open) renderDetails()
  }
  $('sort').onchange = (event) => {
    state.sort = event.target.value
    render()
  }
  $('compare').onchange = (event) => {
    state.compare = event.target.checked
    render()
    if ($('details').open) renderDetails()
  }
  $('grid-view').onclick = () => {
    state.view = 'grid'
    render()
  }
  $('table-view').onclick = () => {
    state.view = 'table'
    render()
  }
  $('refresh').onclick = load
  $('select-all').onchange = (event) => {
    state.visible.forEach((item) => {
      if (event.target.checked) state.selected.add(item.key)
      else state.selected.delete(item.key)
    })
    render()
  }
  $('copy-selected').onclick = () =>
    copyText(
      [
        ...new Set(
          state.models
            .filter((m) => state.selected.has(m.key))
            .map((m) => m.name),
        ),
      ].join('\n'),
    )
  $('details').addEventListener('click', (event) => {
    if (event.target === $('details')) {
      const r = $('details').getBoundingClientRect()
      if (
        event.clientX < r.left ||
        event.clientX > r.right ||
        event.clientY < r.top ||
        event.clientY > r.bottom
      )
        $('details').close()
    }
  })
  document.addEventListener(
    'keydown',
    (event) => {
      if (event.key === 'Escape') setFiltersOpen(false)
    },
    { signal: lifecycle.signal },
  )
  renderFilters()
  load()
  return () => {
    disposed = true
    requestId++
    lifecycle.abort()
    clearTimeout(toastTimer)
    $('details').close()
    root.replaceChildren()
  }
}
