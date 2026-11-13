import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const componentPath = resolve(dirname(fileURLToPath(import.meta.url)), '../AppSidebar.vue')
const componentSource = readFileSync(componentPath, 'utf8')
const stylePath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../style.css')
const styleSource = readFileSync(stylePath, 'utf8')
const zhCommonPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/zh/common.ts')
const enCommonPath = resolve(dirname(fileURLToPath(import.meta.url)), '../../../i18n/locales/en/common.ts')
const zhCommonSource = readFileSync(zhCommonPath, 'utf8')
const enCommonSource = readFileSync(enCommonPath, 'utf8')

describe('AppSidebar custom SVG styles', () => {
  it('does not override uploaded SVG fill or stroke colors', () => {
    expect(componentSource).toContain('.sidebar-svg-icon {')
    expect(componentSource).toContain('color: currentColor;')
    expect(componentSource).toContain('display: block;')
    expect(componentSource).not.toContain('stroke: currentColor;')
    expect(componentSource).not.toContain('fill: none;')
  })
})

describe('AppSidebar scroll position persistence', () => {
  it('binds a template ref to the sidebar nav element', () => {
    expect(componentSource).toContain('ref="sidebarNavRef"')
    expect(componentSource).toContain('sidebar-nav')
  })

  it('declares sidebarNavRef in script setup', () => {
    expect(componentSource).toContain("const sidebarNavRef = ref<HTMLElement | null>(null)")
  })

  it('saves scroll position on beforeUnmount', () => {
    expect(componentSource).toContain('onBeforeUnmount')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('sidebarNavRef.value.scrollTop')
  })

  it('restores scroll position on mount', () => {
    expect(componentSource).toContain('onMounted')
    expect(componentSource).toContain('appStore.sidebarScrollTop')
    expect(componentSource).toContain('nextTick')
  })
})

describe('AppSidebar collapsible groups', () => {
  it('lets the user collapse a group even while a child route is active', () => {
    // The expand state must come from the user's override first, falling back
    // to the active-route heuristic only when the user has not clicked yet.
    expect(componentSource).toContain('const groupExpandOverrides = ref<Map<string, boolean>>(new Map())')
    expect(componentSource).not.toContain('expandedGroups.value.has(item.path) || isGroupActive(item)')
  })
})

describe('AppSidebar usage navigation', () => {
  it('keeps the parent usage item inactive on the nested hold review route', () => {
    expect(componentSource).toContain("if (path === '/admin/usage') return route.path === path")
    expect(componentSource).toContain("label: t('admin.usage.imageHolds.nav'), icon: HoldReviewIcon")
  })
})

describe('AppSidebar reference pricing navigation', () => {
  it('uses a distinct currency icon and the concise reference pricing label', () => {
    expect(componentSource).toContain(
      "{ path: '/admin/channels/reference-prices', label: t('nav.modelPlazaReferencePrices'), icon: ReferencePriceIcon }"
    )
    expect(componentSource).toContain('const ReferencePriceIcon =')
    expect(componentSource).toContain("{ path: '/admin/channels/pricing', label: t('nav.channelPricing'), icon: PriceTagIcon }")
    expect(zhCommonSource).toContain("modelPlazaReferencePrices: '参考定价'")
    expect(enCommonSource).toContain("modelPlazaReferencePrices: 'Reference Pricing'")
  })
})

describe('AppSidebar drill factory entry', () => {
  it('does not render a duplicate entry in the sidebar', () => {
    expect(componentSource).not.toContain('drillFactoryUrl')
    expect(componentSource).not.toContain('drillFactoryNavItems')
    expect(componentSource).not.toContain('drill-factory-menu')
  })
})

describe('AppSidebar channel gateway entry', () => {
  it('adds a configured admin-only external link under Channel Management', () => {
    expect(componentSource).toContain('sanitizeUrl(adminSettingsStore.channelGatewayUrl || \'\')')
    expect(componentSource).toContain("label: t('nav.channelGateway')")
    expect(componentSource).toContain("path: '/external/channel-gateway'")
    expect(componentSource).toContain('externalUrl: channelGatewayUrl.value')
    expect(componentSource).toContain('icon: ChannelGatewayIcon')
    expect(componentSource).toContain("h(Icon, { name: 'swap' })")
    expect(componentSource).toContain('v-for="child in item.children.filter((entry) => entry.externalUrl)"')
    expect(componentSource).toContain('target="_blank"')
    expect(zhCommonSource).toContain("channelGateway: '渠道网关'")
    expect(enCommonSource).toContain("channelGateway: 'Channel Gateway'")
    const channelManagement = componentSource.indexOf("label: t('nav.channelManagement')")
    const gatewayEntry = componentSource.indexOf("label: t('nav.channelGateway')")
    const adminTemplate = componentSource.indexOf('<!-- Admin View: Admin menu first')
    expect(channelManagement).toBeGreaterThan(adminTemplate)
    expect(gatewayEntry).toBeGreaterThan(channelManagement)
  })
})

describe('AppSidebar header styles', () => {
  it('does not clip the version badge dropdown', () => {
    const sidebarHeaderBlockMatch = styleSource.match(/\.sidebar-header\s*\{[\s\S]*?\n {2}\}/)
    const sidebarBrandBlockMatch = componentSource.match(/\.sidebar-brand\s*\{[\s\S]*?\n\}/)

    expect(sidebarHeaderBlockMatch).not.toBeNull()
    expect(sidebarBrandBlockMatch).not.toBeNull()
    expect(sidebarHeaderBlockMatch?.[0]).not.toContain('@apply overflow-hidden;')
    expect(sidebarBrandBlockMatch?.[0]).not.toContain('overflow: hidden;')
  })
})

describe('AppSidebar subscription feature flag', () => {
  it('gates the My Subscriptions entry behind the subscription public-settings flag', () => {
    expect(componentSource).toContain('const flagSubscription = makeSidebarFlag(FeatureFlags.subscription)')
    expect(componentSource).toMatch(/path: '\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('also hides the admin Subscription Management entry on recharge-only sites', () => {
    expect(componentSource).toMatch(/path: '\/admin\/subscriptions'[^\n]*featureFlag: flagSubscription/)
  })

  it('derives the purchase entry label from the site billing mode', () => {
    expect(componentSource).toContain("import { resolveSiteBillingMode } from '@/utils/siteBillingMode'")
    expect(componentSource).toMatch(/case 'recharge_only':\s*return t\('nav\.recharge'\)/)
    expect(componentSource).toMatch(/case 'subscription_only':\s*return t\('nav\.subscribe'\)/)
    expect(componentSource).toMatch(/path: '\/purchase'[^\n]*label: purchaseNavLabel\.value/)
  })
})
