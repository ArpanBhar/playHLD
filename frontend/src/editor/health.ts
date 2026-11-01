import type { Kind, NodeMetrics } from '../simulation/types'

export interface NodeHealth {
  color: string
  tint: string
  load: number | null
  hitRate: number | null
  missRate: number | null
  queue: number
  rejected: number
  score: number
}

const clamp = (value: number) => Math.max(0, Math.min(1, value))

export function getNodeHealth(kind: Kind, metrics?: NodeMetrics): NodeHealth | null {
  if (!metrics) return null
  const load = metrics.processingUtilization == null ? null : clamp(metrics.processingUtilization)
  const hitRate = metrics.cacheLookups > 0 ? metrics.cacheHits / metrics.cacheLookups : null
  const missRate = hitRate == null ? null : 1 - hitRate
  const caching = kind === 'cache' || kind === 'cdn'
  const pressure = caching && missRate != null ? missRate : load ?? 0
  // Green through 60% processing occupancy / 30% misses, amber near the limit, red at 85% / 60%.
  const low = caching ? 0.3 : 0.6
  const high = caching ? 0.6 : 0.85
  const score = Math.max(clamp((pressure - low) / (high - low)), caching && load != null ? clamp((load - 0.6) / 0.25) : 0)
  const hue = Math.round(145 * (1 - score))
  return {
    color: `hsl(${hue} 64% 39%)`,
    tint: `hsl(${hue} 72% 96%)`,
    load, hitRate, missRate, queue: metrics.queueDepth, rejected: metrics.rejected, score,
  }
}
