import type { TrafficSegment, WorkloadProfile } from './types'

export function compileTraffic(profile: WorkloadProfile, durationSeconds: number, rateRps: number): TrafficSegment[] | undefined {
  if (profile === 'steady') return undefined
  const durationUs = durationSeconds * 1_000_000
  if (profile === 'ramp') {
    return Array.from({ length: 4 }, (_, index) => ({
      startUs: Math.floor(index * durationUs / 4),
      endUs: Math.floor((index + 1) * durationUs / 4),
      rateRps: Math.max(1, Math.min(100_000, Math.round(rateRps * (index + 1) / 4))),
    }))
  }
  return [
    { startUs: 0, endUs: Math.floor(durationUs / 3), rateRps },
    { startUs: Math.floor(durationUs / 3), endUs: Math.floor(2 * durationUs / 3), rateRps: Math.min(100_000, rateRps * 2) },
    { startUs: Math.floor(2 * durationUs / 3), endUs: durationUs, rateRps },
  ]
}
