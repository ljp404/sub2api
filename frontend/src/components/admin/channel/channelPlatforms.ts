import type { Channel } from '@/api/admin/channels'
import type { GroupPlatform } from '@/types'

export function resolveChannelPlatforms(
  channel: Pick<Channel, 'group_ids' | 'model_pricing' | 'model_mapping' | 'features_config'>,
  groups: readonly { id: number; platform: GroupPlatform }[],
  platformOrder: readonly GroupPlatform[]
): Set<GroupPlatform> {
  const configured = channel.features_config?.enabled_platforms
  if (Array.isArray(configured)) {
    return new Set(platformOrder.filter(platform => configured.includes(platform)))
  }

  // Older channels inferred their enabled platforms from related configuration.
  const groupPlatforms = new Map(groups.map(group => [group.id, group.platform]))
  const platforms = new Set<GroupPlatform>()
  for (const groupId of channel.group_ids || []) {
    const platform = groupPlatforms.get(groupId)
    if (platform === 'composite') {
      platformOrder.forEach(value => platforms.add(value))
    } else if (platform) {
      platforms.add(platform)
    }
  }
  for (const pricing of channel.model_pricing || []) {
    if (platformOrder.includes(pricing.platform as GroupPlatform)) {
      platforms.add(pricing.platform as GroupPlatform)
    }
  }
  for (const platform of Object.keys(channel.model_mapping || {})) {
    if (platformOrder.includes(platform as GroupPlatform)) {
      platforms.add(platform as GroupPlatform)
    }
  }
  return platforms
}
