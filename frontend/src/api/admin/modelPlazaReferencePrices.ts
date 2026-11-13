import { apiClient } from '../client'

export interface ModelPlazaReferencePricing {
  input_price?: number
  output_price?: number
  cache_write_price?: number
  cache_write_1h_price?: number
  cache_read_price?: number
  image_input_price?: number
  image_output_price?: number
  per_request_price?: number
}

export interface ModelPlazaReferencePrice {
  id: number
  platform: string
  model_name: string
  pricing: ModelPlazaReferencePricing
  source_label: string
  source_url: string
  notes: string
  updated_by?: number
  created_at: string
  updated_at: string
}

export interface UpsertModelPlazaReferencePrice {
  platform: string
  model_name: string
  pricing: ModelPlazaReferencePricing
  source_label: string
  source_url: string
  notes: string
}

export interface ModelPlazaPricingCatalogItem {
  platform: string
  model_name: string
  pricing: ModelPlazaReferencePricing
}

export interface ModelPlazaReferencePricePage {
  items: ModelPlazaReferencePrice[]
  total: number
  page: number
  page_size: number
  pages: number
}

const modelPlazaReferencePricesAPI = {
  async list(params: { page: number; page_size: number; search?: string }): Promise<ModelPlazaReferencePricePage> {
    const { data } = await apiClient.get<ModelPlazaReferencePricePage>('/admin/model-plaza/reference-prices', { params })
    return data
  },

  async upsert(input: UpsertModelPlazaReferencePrice): Promise<ModelPlazaReferencePrice> {
    const { data } = await apiClient.put<ModelPlazaReferencePrice>('/admin/model-plaza/reference-prices', input)
    return data
  },

  async import(prices: UpsertModelPlazaReferencePrice[]): Promise<{ imported_count: number }> {
    const { data } = await apiClient.post<{ imported_count: number }>('/admin/model-plaza/reference-prices/import', { prices })
    return data
  },

  async remove(id: number): Promise<void> {
    await apiClient.delete(`/admin/model-plaza/reference-prices/${id}`)
  },

  async removeMany(ids: number[]): Promise<{ deleted_count: number }> {
    const { data } = await apiClient.post<{ deleted_count: number }>('/admin/model-plaza/reference-prices/batch-delete', { ids })
    return data
  },

  async catalog(): Promise<ModelPlazaPricingCatalogItem[]> {
    const { data } = await apiClient.get<{ models: ModelPlazaPricingCatalogItem[] }>('/admin/channels/pricing/catalog')
    return data.models
  }
}

export default modelPlazaReferencePricesAPI
