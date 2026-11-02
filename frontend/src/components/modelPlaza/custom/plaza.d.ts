import type { ModelPlazaResponse } from '@/api/modelPlaza'

export function mountPlaza(
  root: ShadowRoot,
  fetchPlaza: () => Promise<ModelPlazaResponse>,
  authenticated: boolean,
): () => void
