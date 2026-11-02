<template>
  <div class="auth-shell">
    <header class="auth-header">
      <router-link to="/home" class="auth-brand" :aria-label="t('auth.presentation.home')">
        <img :src="siteLogo || '/logo.svg'" alt="" class="auth-brand-logo" />
        <span class="auth-brand-name">{{ siteName }}</span>
        <span class="auth-brand-divider" aria-hidden="true"></span>
        <span class="auth-brand-caption">AI PLATFORM</span>
      </router-link>
      <router-link to="/home" class="auth-home-link">
        {{ t('auth.presentation.home') }}
        <Icon name="arrowUp" size="sm" class="rotate-45" aria-hidden="true" />
      </router-link>
    </header>

    <main class="auth-main">
      <section class="auth-intro" :aria-label="t('auth.presentation.platform')">
        <div class="auth-eyebrow">
          <span class="auth-eyebrow-mark" aria-hidden="true"></span>
          MODEL ACCESS. SIMPLIFIED.
        </div>
        <h1 class="auth-headline">{{ t('auth.presentation.platform') }}</h1>
        <p class="auth-tagline">{{ t('auth.presentation.tagline') }}</p>

        <div class="auth-network" :aria-label="t('auth.presentation.network')">
          <div class="auth-network-heading">
            <span>MODEL NETWORK</span>
            <span class="auth-network-symbol" aria-hidden="true">[ + ]</span>
          </div>
          <div class="auth-providers">
            <div v-for="provider in providers" :key="provider.model" class="auth-provider">
              <ModelIcon :model="provider.model" size="28px" />
              <span>{{ provider.name }}</span>
            </div>
          </div>
          <div class="auth-network-bus" aria-hidden="true">
            <span class="auth-network-signal"></span>
          </div>
          <div class="auth-gateway">
            <div class="auth-gateway-logo">
              <img :src="siteLogo || '/logo.svg'" alt="" />
            </div>
            <div class="auth-gateway-brand">
              <strong>{{ siteName }}</strong>
              <span>{{ t('auth.presentation.gateway') }}</span>
            </div>
            <Icon name="link" size="md" class="auth-gateway-icon" aria-hidden="true" />
          </div>
          <div class="auth-capabilities">
            <span><Icon name="chat" size="md" aria-hidden="true" />{{ t('auth.presentation.chat') }}</span>
            <span><Icon name="sparkles" size="md" aria-hidden="true" />{{ t('auth.presentation.image') }}</span>
            <span><Icon name="terminal" size="md" aria-hidden="true" />{{ t('auth.presentation.code') }}</span>
          </div>
        </div>

        <div class="auth-principles">
          <span><Icon name="key" size="sm" aria-hidden="true" />{{ t('auth.presentation.access') }}</span>
          <span><Icon name="chart" size="sm" aria-hidden="true" />{{ t('auth.presentation.usage') }}</span>
          <span><Icon name="grid" size="sm" aria-hidden="true" />{{ t('auth.presentation.models') }}</span>
        </div>
      </section>

      <section class="auth-form-panel" :aria-label="t('auth.presentation.account')">
        <div class="auth-form-topline">
          <span class="auth-form-mark"><Icon name="lock" size="md" aria-hidden="true" /></span>
          <span>{{ t('auth.presentation.account') }}</span>
        </div>
        <div class="auth-form-content"><slot /></div>
        <div v-if="$slots.footer" class="auth-form-footer"><slot name="footer" /></div>
      </section>
    </main>

    <footer class="auth-page-footer">
      <span>&copy; {{ currentYear }} {{ siteName }}. All rights reserved.</span>
      <span class="auth-footer-note">BUILT FOR YOUR NEXT IDEA</span>
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores'
import { sanitizeUrl } from '@/utils/url'
import Icon from '@/components/icons/Icon.vue'
import ModelIcon from '@/components/common/ModelIcon.vue'

const { t } = useI18n()
const appStore = useAppStore()

const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => sanitizeUrl(appStore.siteLogo || '', { allowRelative: true, allowDataUrl: true }))
const currentYear = computed(() => new Date().getFullYear())
const providers = [
  { model: 'gpt', name: 'OpenAI' },
  { model: 'claude', name: 'Claude' },
  { model: 'gemini', name: 'Gemini' }
]

onMounted(() => {
  appStore.fetchPublicSettings()
})
</script>

<style scoped src="./auth-layout.css"></style>
