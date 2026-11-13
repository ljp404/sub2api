import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const headerSource = readFileSync(resolve(dir, '../AppHeader.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('doc_url sanitization', () => {
  it('AppHeader imports sanitizeUrl', () => {
    expect(headerSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('AppHeader applies sanitizeUrl to docUrl', () => {
    expect(headerSource).toContain('sanitizeUrl(appStore.docUrl)')
  })

  it('places the configured AI experience link beside Model Plaza with a distinct icon', () => {
    const modelPlazaIndex = headerSource.indexOf('t(\'nav.modelPlaza\')')
    const aiExperienceIndex = headerSource.indexOf('t(\'nav.aiExperience\')')
    expect(aiExperienceIndex).toBeGreaterThan(modelPlazaIndex)
    expect(headerSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.drill_factory_url || \'\')')
    expect(headerSource).toContain('v-if="user && aiExperienceUrl"')
    expect(headerSource).toContain('target="_blank"')
    expect(headerSource).toContain('rel="noopener noreferrer"')
    expect(headerSource).toContain('<Icon name="sparkles" size="sm" />')
  })

  it('HomeView imports sanitizeUrl', () => {
    expect(homeViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('HomeView applies sanitizeUrl to docUrl', () => {
    expect(homeViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl')
  })

  it('KeyUsageView imports sanitizeUrl', () => {
    expect(keyUsageViewSource).toContain("import { sanitizeUrl } from '@/utils/url'")
  })

  it('KeyUsageView applies sanitizeUrl to docUrl', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.doc_url || appStore.docUrl')
  })
})
