import { describe, expect, it } from 'vitest'
import { getAdminSteps, getUserSteps } from '../steps'

const translations: Record<string, string> = {
  'onboarding.admin.welcome.title': '👋 欢迎使用 Sub2API',
  'onboarding.admin.welcome.description': '<p>Sub2API 管理 AI 服务。</p>',
  'onboarding.user.welcome.title': '👋 欢迎使用 Sub2API',
  'onboarding.user.welcome.description': '<p>欢迎来到 Sub2API AI 服务平台。</p>'
}

const t = (key: string) => translations[key] ?? key

describe('onboarding welcome branding', () => {
  it('uses the configured site name in the admin welcome step', () => {
    const welcome = getAdminSteps(t, false, 'YUNDATA')[0].popover

    expect(welcome?.title).toBe('👋 欢迎使用 YUNDATA')
    expect(welcome?.description).toBe('<p>YUNDATA 管理 AI 服务。</p>')
  })

  it('uses the configured site name in the user welcome step', () => {
    const welcome = getUserSteps(t, '海数云')[0].popover

    expect(welcome?.title).toBe('👋 欢迎使用 海数云')
    expect(welcome?.description).toBe('<p>欢迎来到 海数云 AI 服务平台。</p>')
  })

  it('escapes custom site names before inserting them into tour HTML', () => {
    const welcome = getAdminSteps(t, false, '<img src=x onerror=alert(1)> Y&DATA')[0].popover

    expect(welcome?.title).toContain('&lt;img src=x onerror=alert(1)&gt; Y&amp;DATA')
    expect(welcome?.description).toContain('&lt;img src=x onerror=alert(1)&gt; Y&amp;DATA')
    expect(welcome?.description).not.toContain('<img src=x onerror=alert(1)>')
  })

  it('falls back to the upstream name when no site name is configured', () => {
    const welcome = getAdminSteps(t)[0].popover

    expect(welcome?.title).toBe('👋 欢迎使用 Sub2API')
  })
})
