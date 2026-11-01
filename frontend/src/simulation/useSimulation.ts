import { useCallback, useEffect, useRef, useState } from 'react'
import type { Architecture, GoalOutcome, GoalRule, Result, Snapshot, TrafficSegment, WorkerEvent, WorkloadProfile } from './types'
import { compileTraffic } from './workload'

type Status = 'loading' | 'ready' | 'running' | 'paused' | 'stopped' | 'completed' | 'error'

function liveConfig(architecture: Architecture, rateRps: number, durationSeconds: number, seed: string, readPercent: number, profile: WorkloadProfile, arrivalModel: 'fixed' | 'poisson', fixedSegments?: TrafficSegment[]) {
  const entryNodeId = architecture.nodes.find(node => node.kind === 'source')?.id
  if (!entryNodeId) throw new Error('Add a Source node before running.')
  // Each engine instance is bounded; the worker starts the next window automatically.
  const durationUs = Math.max(1, Math.min(durationSeconds * 1_000_000, Math.floor(80_000_000_000 / rateRps)))
  const cycleSeconds = durationUs / 1_000_000
  const segments = fixedSegments ?? (cycleSeconds >= 4 ? compileTraffic(profile, cycleSeconds, rateRps) : undefined)
  return JSON.stringify({ architecture, workload: { schemaVersion: 1, seed, entryNodeId, durationUs, drainTimeoutUs: 0, arrivalRateRps: rateRps, readFraction: readPercent / 100, ...(segments ? { segments } : {}), ...(arrivalModel === 'poisson' ? { arrivalModel } : {}) }, snapshotIntervalUs: 1_000_000 })
}

export function useSimulation(enabled = true) {
  const workerRef = useRef<Worker | null>(null)
  const runIdRef = useRef(0)
  const wasLiveRef = useRef(false)
  const [status, setStatus] = useState<Status>('loading')
  const [live, setLive] = useState(false)
  const [warning, setWarning] = useState<string | null>(null)
  const [liveP95LatencyUs, setLiveP95LatencyUs] = useState<number | null>(null)
  const [liveP99LatencyUs, setLiveP99LatencyUs] = useState<number | null>(null)
  const [snapshots, setSnapshots] = useState<Snapshot[]>([])
  const [result, setResult] = useState<Result | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [goals, setGoals] = useState<GoalOutcome[] | null>(null)

  useEffect(() => {
    if (!enabled) return
    setStatus('loading')
    const worker = new Worker(new URL('./worker.ts', import.meta.url), { type: 'module' })
    workerRef.current = worker
    worker.onmessage = (event: MessageEvent<WorkerEvent>) => {
      const message = event.data
      if (message.type !== 'READY' && message.runId !== runIdRef.current) return
      if (message.type === 'READY') setStatus('ready')
      if (message.type === 'SNAPSHOTS') setSnapshots(current => [...current, ...message.snapshots].slice(-300))
      if (message.type === 'COMPLETED') {
        setResult(message.result)
        setGoals(message.goals || null)
        setSnapshots(message.result.snapshots)
        setStatus('completed')
      }
      if (message.type === 'PAUSED') setStatus('paused')
      if (message.type === 'CANCELED') { setStatus(wasLiveRef.current ? 'stopped' : 'ready'); setLive(false); setWarning(null); wasLiveRef.current = false }
      if (message.type === 'CONFIG_ERROR') setWarning(`Live graph update pending: ${message.message}`)
      if (message.type === 'CONFIG_APPLIED') { setWarning(null); setLiveP95LatencyUs(null); setLiveP99LatencyUs(null) }
      if (message.type === 'LIVE_CYCLE') { setLiveP95LatencyUs(message.p95LatencyUs); setLiveP99LatencyUs(message.p99LatencyUs) }
      if (message.type === 'ERROR') { setError(message.message); setStatus('error'); setLive(false); wasLiveRef.current = false }
    }
    worker.onerror = event => { setError(event.message || 'Worker crashed'); setStatus('error') }
    worker.postMessage({ type: 'INIT', runId: 0 })
    return () => { worker.terminate(); workerRef.current = null }
  }, [enabled])

  const run = useCallback((architecture: Architecture, rateRps: number, durationSeconds: number, seed: string, readPercent: number, rules?: GoalRule[], drainTimeoutUs = 5_000_000, segments?: TrafficSegment[], arrivalModel: 'fixed' | 'poisson' = 'fixed') => {
    if (!workerRef.current || status === 'loading') return
    const entryNodeId = architecture.nodes.find(node => node.kind === 'source')?.id
    if (!entryNodeId) { setError('Add a Source node before running.'); setStatus('error'); return }
    runIdRef.current += 1
    const runId = runIdRef.current
    setError(null)
    setResult(null)
    setGoals(null)
    setWarning(null)
    setLiveP95LatencyUs(null)
    setLiveP99LatencyUs(null)
    setLive(false)
    wasLiveRef.current = false
    setSnapshots([])
    setStatus('running')
    workerRef.current.postMessage({ type: 'CREATE', runId, goals: rules ? JSON.stringify(rules) : undefined, config: JSON.stringify({
      architecture,
      workload: { schemaVersion: 1, seed, entryNodeId, durationUs: durationSeconds * 1_000_000, drainTimeoutUs, arrivalRateRps: rateRps, readFraction: readPercent / 100, ...(segments ? { segments } : {}), ...(arrivalModel === 'poisson' ? { arrivalModel } : {}) },
      snapshotIntervalUs: 1_000_000,
    }) })
  }, [status])

  const startLive = useCallback((architecture: Architecture, rateRps: number, durationSeconds: number, seed: string, readPercent: number, profile: WorkloadProfile, arrivalModel: 'fixed' | 'poisson', fixedSegments?: TrafficSegment[]) => {
    if (!workerRef.current || status === 'loading') return
    let config: string
    try { config = liveConfig(architecture, rateRps, durationSeconds, seed, readPercent, profile, arrivalModel, fixedSegments) }
    catch (error) { setError(error instanceof Error ? error.message : String(error)); setStatus('error'); return }
    runIdRef.current += 1
    setError(null); setWarning(null); setLiveP95LatencyUs(null); setLiveP99LatencyUs(null); setResult(null); setGoals(null); setSnapshots([])
    setLive(true); wasLiveRef.current = true; setStatus('running')
    workerRef.current.postMessage({ type: 'LIVE_CREATE', runId: runIdRef.current, config })
  }, [status])

  const updateLive = useCallback((architecture: Architecture, rateRps: number, durationSeconds: number, seed: string, readPercent: number, profile: WorkloadProfile, arrivalModel: 'fixed' | 'poisson', fixedSegments?: TrafficSegment[]) => {
    if (!workerRef.current || !wasLiveRef.current) return
    try { workerRef.current.postMessage({ type: 'LIVE_UPDATE', runId: runIdRef.current, config: liveConfig(architecture, rateRps, durationSeconds, seed, readPercent, profile, arrivalModel, fixedSegments) }) }
    catch (error) { setWarning(error instanceof Error ? error.message : String(error)) }
  }, [])

  const pause = useCallback(() => workerRef.current?.postMessage({ type: 'PAUSE', runId: runIdRef.current }), [])
  const resume = useCallback(() => { setStatus('running'); workerRef.current?.postMessage({ type: 'RUN', runId: runIdRef.current }) }, [])
  const cancel = useCallback(() => { workerRef.current?.postMessage({ type: 'CANCEL', runId: runIdRef.current }); setStatus(wasLiveRef.current ? 'stopped' : 'ready'); setLive(false); setWarning(null) }, [])
  const clear = useCallback(() => {
    const previousRunId = runIdRef.current
    runIdRef.current += 1
    workerRef.current?.postMessage({ type: 'CANCEL', runId: previousRunId })
    wasLiveRef.current = false
    setStatus(current => current === 'loading' ? current : 'ready')
    setLive(false); setWarning(null); setError(null); setResult(null); setGoals(null); setSnapshots([]); setLiveP95LatencyUs(null); setLiveP99LatencyUs(null)
  }, [])

  return { status, live, warning, liveP95LatencyUs, liveP99LatencyUs, snapshots, result, error, goals, run, startLive, updateLive, pause, resume, cancel, clear }
}
