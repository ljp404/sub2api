import { describe, expect, it } from 'vitest'
import type { GroupPlatform } from '@/types'
import { resolveChannelPlatforms } from './channelPlatforms'

const platformOrder: GroupPlatform[] = ['anthropic', 'openai', 'gemini', 'minimax']
const groups: { id: number; platform: GroupPlatform }[] = [
  { id: 1, platform: 'composite' },
  { id: 2, platform: 'anthropic' }
]
const channel = {
  group_ids: [1, 2],
  model_pricing: [],
  model_mapping: { openai: { alias: 'model' } },
  features_config: {}
}

describe('channel platform selection', () => {
  it('retains an explicit selection despite composite and other group associations', () => {
    expect([...resolveChannelPlatforms({
      ...channel,
      features_config: { enabled_platforms: ['openai', 'gemini'] }
    }, groups, platformOrder)]).toEqual(['openai', 'gemini'])
  })

  it('retains an empty selection instead of restoring platforms from groups or mappings', () => {
    expect([...resolveChannelPlatforms({
      ...channel,
      features_config: { enabled_platforms: [] }
    }, groups, platformOrder)]).toEqual([])
  })

  it('retains explicitly selected platforms without pricing or model mappings', () => {
    expect([...resolveChannelPlatforms({
      ...channel, group_ids: [], model_mapping: {},
      features_config: { enabled_platforms: ['minimax'] }
    }, groups, platformOrder)]).toEqual(['minimax'])
  })

  it('ignores platform IDs absent from the current catalog', () => {
    expect([...resolveChannelPlatforms({
      ...channel,
      features_config: { enabled_platforms: ['openai', 'unknown', 1] }
    }, groups, platformOrder)]).toEqual(['openai'])
  })

  it('keeps legacy composite expansion when no explicit selection was saved', () => {
    expect([...resolveChannelPlatforms(channel, groups, platformOrder)]).toEqual(platformOrder)
  })

  it('keeps legacy concrete group and mapping selections', () => {
    expect([...resolveChannelPlatforms({
      ...channel, group_ids: [2]
    }, groups, platformOrder)]).toEqual(['anthropic', 'openai'])
  })
})
