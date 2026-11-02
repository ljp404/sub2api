<template>
  <div ref="host" class="custom-model-plaza"></div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { getModelPlaza } from '@/api/modelPlaza'
import { useAuthStore } from '@/stores/auth'
import { mountPlaza } from './custom/plaza'
import markup from './custom/plaza.html?raw'
import styles from './custom/plaza.css?raw'

const host = ref<HTMLElement>()
const authStore = useAuthStore()
const controller = new AbortController()
let dispose: (() => void) | undefined

onMounted(() => {
  if (!host.value) return
  // Isolate the approved standalone design from the application's global styles.
  const root = host.value.attachShadow({ mode: 'open' })
  const style = document.createElement('style')
  style.textContent = styles
  root.append(style)
  const content = document.createElement('div')
  content.innerHTML = markup
  root.append(content)
  dispose = mountPlaza(
    root,
    () =>
      getModelPlaza({
        signal: AbortSignal.any([
          controller.signal,
          AbortSignal.timeout(15000),
        ]),
      }),
    authStore.isAuthenticated,
  )
})

onBeforeUnmount(() => {
  controller.abort()
  dispose?.()
})
</script>

<style scoped>
.custom-model-plaza {
  display: block;
  min-width: 0;
}
</style>
