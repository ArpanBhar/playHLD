/// <reference lib="webworker" />
import './wasm_exec.js'
import type { GoalOutcome, Result, Snapshot } from './types'

type GoRuntime = { importObject: WebAssembly.Imports; run: (instance: WebAssembly.Instance) => Promise<void> }
type WasmScope = DedicatedWorkerGlobalScope & {
  Go: new () => GoRuntime
  playHLDReady?: boolean
  playHLDCreate?: (config: string) => string
  playHLDStep?: (handle: string, batchSize: number) => string
  playHLDStepUntilSnapshot?: (handle: string, batchSize: number) => string
  playHLDUpdateArchitecture?: (handle: string, architecture: string) => string
  playHLDExportState?: (handle: string) => string
  playHLDImportState?: (handle: string, state: string) => string
  playHLDReset?: (handle: string) => string
  playHLDDispose?: (handle: string) => string
  playHLDEvaluateGoals?: (result: string, workload: string, goals: string) => string
}
type Reply = { ok: boolean; handle?: string; outcome?: { snapshots: Snapshot[]; processedEvents: number; done: boolean; result?: Result }; goals?: GoalOutcome[]; state?: string; error?: string }

const scope = self as unknown as WasmScope
let initialized = false
let handle: string | null = null
let runId = 0
let running = false
let goalRules: string | null = null
let workload: string | null = null
let mode: 'finite' | 'live' = 'finite'
let liveConfig: string | null = null
let liveEpochMs = 0
let liveOffsetUs = 0
let liveCurrentUs = 0
let liveLastSnapshot: Snapshot | null = null
let liveBase = { offered: 0, succeeded: 0, failed: 0 }
let generation = 0
let liveTimer: ReturnType<typeof setTimeout> | null = null
let pendingLive: { snapshot: Snapshot; after: () => void } | null = null

function decode(raw: string): Reply {
  const reply = JSON.parse(raw) as Reply
  if (!reply.ok) throw new Error(reply.error || 'Simulation failed')
  return reply
}

async function init() {
  if (initialized) return
  const go = new scope.Go()
  const source = await fetch('/wasm/simulator.wasm')
  if (!source.ok) throw new Error(`WASM download failed: ${source.status}`)
  const bytes = await source.arrayBuffer()
  const instance = await WebAssembly.instantiate(bytes, go.importObject)
  void go.run(instance.instance).catch(error => scope.postMessage({ type: 'ERROR', runId, message: String(error) }))
  for (let tries = 0; tries < 100 && !scope.playHLDReady; tries += 1) {
    await new Promise(resolve => setTimeout(resolve, 50))
  }
  if (!scope.playHLDReady) throw new Error('Go runtime did not become ready')
  initialized = true
}

function dispose() {
  running = false
  generation += 1
  if (liveTimer !== null) clearTimeout(liveTimer)
  liveTimer = null
  pendingLive = null
  if (handle && scope.playHLDDispose) decode(scope.playHLDDispose(handle))
  handle = null
}

function createHandle(config: string) {
  const created = decode(scope.playHLDCreate!(config)).handle
  if (!created) throw new Error('Engine did not return a handle')
  return created
}

function scheduleLiveFrame() {
  if (!pendingLive || !running) return
  const expectedGeneration = generation
  const due = liveEpochMs + pendingLive.snapshot.atUs / 1000
  liveTimer = setTimeout(() => {
    if (!running || expectedGeneration !== generation || !pendingLive) return
    const { snapshot, after } = pendingLive
    pendingLive = null
    liveTimer = null
    liveCurrentUs = snapshot.atUs
    liveLastSnapshot = snapshot
    scope.postMessage({ type: 'SNAPSHOTS', runId, snapshots: [snapshot], processedEvents: 0 })
    after()
  }, Math.max(0, due - performance.now()))
}

function queueLiveFrame(snapshot: Snapshot, after: () => void) {
  pendingLive = { snapshot: { ...snapshot, atUs: liveOffsetUs + snapshot.atUs, offered: liveBase.offered + snapshot.offered, succeeded: liveBase.succeeded + snapshot.succeeded, failed: liveBase.failed + snapshot.failed }, after }
  scheduleLiveFrame()
}

function restartLive(expectedRunId: number, expectedGeneration: number) {
  if (!running || expectedRunId !== runId || expectedGeneration !== generation || !liveConfig) return
  try {
    const previous = handle
    const state = previous && scope.playHLDExportState ? decode(scope.playHLDExportState(previous)).state : undefined
    const replacement = createHandle(liveConfig)
    try { if (state && scope.playHLDImportState) decode(scope.playHLDImportState(replacement, state)) }
    catch (error) { decode(scope.playHLDDispose!(replacement)); throw error }
    if (previous) decode(scope.playHLDDispose!(previous))
    handle = replacement
    liveOffsetUs = liveCurrentUs
    if (liveLastSnapshot) liveBase = { offered: liveLastSnapshot.offered, succeeded: liveLastSnapshot.succeeded, failed: liveLastSnapshot.failed }
    setTimeout(() => pumpLive(expectedRunId, expectedGeneration), 0)
  } catch (error) {
    running = false
    scope.postMessage({ type: 'ERROR', runId, message: error instanceof Error ? error.message : String(error) })
  }
}

function pumpLive(expectedRunId: number, expectedGeneration: number) {
  if (!running || !handle || expectedRunId !== runId || expectedGeneration !== generation || pendingLive) return
  try {
    const outcome = decode(scope.playHLDStepUntilSnapshot!(handle, 2_000)).outcome!
    if (outcome.snapshots?.length) {
      queueLiveFrame(outcome.snapshots[0], () => setTimeout(() => pumpLive(expectedRunId, expectedGeneration), 0))
    } else if (outcome.done) {
      const final = outcome.result?.snapshots.at(-1)
      if (!final) throw new Error('Live simulation returned no final snapshot')
      queueLiveFrame(final, () => {
        scope.postMessage({ type: 'LIVE_CYCLE', runId, p95LatencyUs: outcome.result?.summary.p95LatencyUs ?? null, p99LatencyUs: outcome.result?.summary.p99LatencyUs ?? null })
        restartLive(expectedRunId, expectedGeneration)
      })
    } else {
      setTimeout(() => pumpLive(expectedRunId, expectedGeneration), 0)
    }
  } catch (error) {
    running = false
    scope.postMessage({ type: 'ERROR', runId, message: error instanceof Error ? error.message : String(error) })
  }
}

function pump(expectedRunId: number) {
  if (!running || !handle || expectedRunId !== runId) return
  try {
    const outcome = decode(scope.playHLDStep!(handle, 2_000)).outcome!
    if (outcome.snapshots?.length) scope.postMessage({ type: 'SNAPSHOTS', runId, snapshots: outcome.snapshots, processedEvents: outcome.processedEvents })
    if (outcome.done) {
      running = false
      const goals = goalRules && workload && outcome.result ? decode(scope.playHLDEvaluateGoals!(JSON.stringify(outcome.result), workload, goalRules)).goals : undefined
      scope.postMessage({ type: 'COMPLETED', runId, result: outcome.result, goals })
      return
    }
    setTimeout(() => pump(expectedRunId), 0)
  } catch (error) {
    running = false
    scope.postMessage({ type: 'ERROR', runId, message: error instanceof Error ? error.message : String(error) })
  }
}

scope.onmessage = async event => {
  const message = event.data as { type: string; runId: number; config?: string; goals?: string }
  try {
    if (message.type === 'INIT') {
      await init()
      scope.postMessage({ type: 'READY', runId: message.runId })
    } else if (message.type === 'CREATE') {
      if (!initialized || !message.config) throw new Error('Worker is not ready')
      dispose()
      mode = 'finite'
      runId = message.runId
      goalRules = message.goals || null
      workload = JSON.stringify((JSON.parse(message.config) as { workload: unknown }).workload)
      handle = createHandle(message.config)
      running = true
      setTimeout(() => pump(runId), 0)
    } else if (message.type === 'LIVE_CREATE') {
      if (!initialized || !message.config) throw new Error('Worker is not ready')
      dispose()
      mode = 'live'
      runId = message.runId
      liveConfig = message.config
      handle = createHandle(message.config)
      liveOffsetUs = 0
      liveCurrentUs = 0
      liveLastSnapshot = null
      liveBase = { offered: 0, succeeded: 0, failed: 0 }
      liveEpochMs = performance.now()
      running = true
      const expectedGeneration = generation
      setTimeout(() => pumpLive(runId, expectedGeneration), 0)
    } else if (message.type === 'LIVE_UPDATE') {
      if (mode !== 'live' || message.runId !== runId || !message.config || !handle) return
      const current = JSON.parse(liveConfig!) as { architecture: { nodes: { id: string; kind: string }[] }; workload: unknown }
      const next = JSON.parse(message.config) as { architecture: { nodes: { id: string; kind: string }[] }; workload: unknown }
      const sameWorkload = JSON.stringify(current.workload) === JSON.stringify(next.workload)
      const compatibleNodes = current.architecture.nodes.every(previous => next.architecture.nodes.some(node => node.id === previous.id && node.kind === previous.kind))
      if (sameWorkload && compatibleNodes) {
        try { decode(scope.playHLDUpdateArchitecture!(handle, JSON.stringify(next.architecture))) }
        catch (error) { scope.postMessage({ type: 'CONFIG_ERROR', runId, message: error instanceof Error ? error.message : String(error) }); return }
        liveConfig = message.config
        scope.postMessage({ type: 'CONFIG_APPLIED', runId })
        return
      }
      let replacement: string | null = null
      try {
        replacement = createHandle(message.config)
        const previousWorkload = current.workload as { scenario?: string; keySpace?: number }
        const nextWorkload = next.workload as { scenario?: string; keySpace?: number }
        if (previousWorkload.scenario && previousWorkload.scenario === nextWorkload.scenario && (previousWorkload.keySpace ?? 64) === (nextWorkload.keySpace ?? 64) && scope.playHLDExportState && scope.playHLDImportState) {
          const state = decode(scope.playHLDExportState(handle)).state
          if (state) decode(scope.playHLDImportState(replacement, state))
        }
      }
      catch (error) { if (replacement) decode(scope.playHLDDispose!(replacement)); scope.postMessage({ type: 'CONFIG_ERROR', runId, message: error instanceof Error ? error.message : String(error) }); return }
      if (liveTimer !== null) clearTimeout(liveTimer)
      liveTimer = null
      pendingLive = null
      generation += 1
      decode(scope.playHLDDispose!(handle))
      handle = replacement
      liveConfig = message.config
      liveOffsetUs = liveCurrentUs
      if (liveLastSnapshot) liveBase = { offered: liveLastSnapshot.offered, succeeded: liveLastSnapshot.succeeded, failed: liveLastSnapshot.failed }
      liveEpochMs = performance.now() - liveCurrentUs / 1000
      scope.postMessage({ type: 'CONFIG_APPLIED', runId })
      if (running) { const expectedGeneration = generation; setTimeout(() => pumpLive(runId, expectedGeneration), 0) }
    } else if (message.type === 'RUN') {
      if (!handle) throw new Error('No run to resume')
      running = true
      if (mode === 'live') {
        liveEpochMs = performance.now() - liveCurrentUs / 1000
        if (pendingLive) scheduleLiveFrame()
        else { const expectedGeneration = generation; setTimeout(() => pumpLive(runId, expectedGeneration), 0) }
      } else setTimeout(() => pump(runId), 0)
    } else if (message.type === 'PAUSE') {
      running = false
      if (liveTimer !== null) clearTimeout(liveTimer)
      liveTimer = null
      scope.postMessage({ type: 'PAUSED', runId })
    } else if (message.type === 'RESET') {
      if (!handle) throw new Error('No run to reset')
      running = false
      decode(scope.playHLDReset!(handle))
    } else if (message.type === 'CANCEL' || message.type === 'DISPOSE') {
      dispose()
      scope.postMessage({ type: 'CANCELED', runId })
    } else {
      throw new Error('Unknown worker command')
    }
  } catch (error) {
    scope.postMessage({ type: 'ERROR', runId: message.runId, message: error instanceof Error ? error.message : String(error) })
  }
}

