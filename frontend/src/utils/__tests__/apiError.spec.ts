import { describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/common'
import zh from '@/i18n/locales/zh/common'
import { extractI18nErrorMessage } from '@/utils/apiError'

// Match the production Vite flag used by the runtime-only i18n build.
vi.hoisted(() => {
  Object.assign(globalThis, { __INTLIFY_JIT_COMPILATION__: true })
})

function translateError(locale: 'zh' | 'en', error: unknown): string {
  const { global } = createI18n({
    legacy: false,
    locale,
    fallbackLocale: false,
    missingWarn: false,
    fallbackWarn: false,
    messages: { en, zh }
  })

  return extractI18nErrorMessage(error, global.t, 'auth.errors', global.t('auth.loginFailed'))
}

describe('authentication error localization', () => {
  const verificationError = {
    code: 400,
    reason: 'INVALID_VERIFY_CODE',
    message: 'invalid or expired verification code'
  }

  it('shows the Chinese verification error instead of the backend message', () => {
    expect(translateError('zh', verificationError)).toBe('验证码无效或已过期')
  })

  it('uses the English verification message when English is selected', () => {
    expect(translateError('en', verificationError)).toBe('Invalid or expired verification code')
  })

  it('also translates errors carrying a string code', () => {
    expect(translateError('zh', {
      code: 'INVALID_VERIFY_CODE',
      message: verificationError.message
    })).toBe('验证码无效或已过期')
  })

  it('preserves the backend message for an unknown reason', () => {
    expect(translateError('zh', {
      reason: 'UNKNOWN_AUTH_ERROR',
      message: 'Unexpected authentication error'
    })).toBe('Unexpected authentication error')
  })

  it('uses the localized fallback when no error message is available', () => {
    expect(translateError('zh', {})).toBe(zh.auth.loginFailed)
  })
})
