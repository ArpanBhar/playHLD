export type Kind = 'source' | 'load_balancer' | 'api' | 'worker' | 'cache' | 'cdn' | 'database' | 'object_storage' | 'message_queue'

export interface NodeConfig {
  processingSlots?: number
  maxInFlight?: number
  meanServiceTimeUs: number
  queueLimit: number
  hitRate?: number
  readServiceTimeUs?: number
  writeServiceTimeUs?: number
  routingPolicy?: 'round_robin' | 'weighted'
  serviceTimeModel?: 'constant' | 'exponential'
}

export interface ArchitectureNode {
  id: string
  kind: Kind
  label: string
  position: { x: number; y: number }
  config: NodeConfig
}

export interface ArchitectureEdge {
  id: string
  source: string
  target: string
  route: 'always' | 'cache_hit' | 'cache_miss' | 'weighted'
  weight?: number
  delayUs: number
}

export interface Architecture {
  schemaVersion: 3
  nodes: ArchitectureNode[]
  edges: ArchitectureEdge[]
}

export interface TrafficSegment { startUs: number; endUs: number; rateRps: number }
export type WorkloadProfile = 'steady' | 'ramp' | 'burst'

export interface NodeMetrics {
  completions: number
  rejected: number
  queueDepth: number
  maxQueueDepth: number
  processingUtilization: number | null
  cacheHits: number
  cacheLookups: number
}

export interface Snapshot {
  atUs: number
  windowDurationUs: number
  offered: number
  succeeded: number
  failed: number
  offeredRps: number
  throughputRps: number
  nodeMetrics: Record<string, NodeMetrics>
  edgeTraversals?: Record<string, number>
}

export interface Result {
  resultSchemaVersion: number
  engineVersion: string
  status: string
  snapshots: Snapshot[]
  summary: {
    generated: number
    succeeded: number
    failed: number
    unfinished: number
    processedEvents: number
    p95LatencyUs: number | null
    p99LatencyUs: number | null
  }
  warnings: string[] | null
}

export interface GoalRule { id: string; label: string; metric: string; operator: 'gte' | 'lte'; target: number; nodeId?: string }
export interface GoalOutcome { id: string; label: string; status: 'met' | 'unmet' | 'not_measurable'; observed: number | null; target: number; unit: string; explanation: string }
export interface Challenge {
  id: string; version: number; title: string; difficulty: string; scenario: string
  starter: Architecture; workload: { schemaVersion: number; seed: string; entryNodeId: string; durationUs: number; drainTimeoutUs: number; arrivalRateRps: number; readFraction: number; segments?: TrafficSegment[] }
  goals: GoalRule[]; hints: string[]; allowedKinds: Kind[]
  resourceBudget: { maxNodes: number; maxSlots: number; maxProcessingSlotsPerNode: number; maxByKind: Partial<Record<Kind, number>> }
  componentSpecs: Partial<Record<Kind, { meanServiceTimeUs: number; queueLimit: number; hitRate?: number }>>
  solution: { explanation: string; architecture: Architecture }
}

export type WorkerEvent =
  | { type: 'READY'; runId: number }
  | { type: 'SNAPSHOTS'; runId: number; snapshots: Snapshot[]; processedEvents: number }
  | { type: 'COMPLETED'; runId: number; result: Result; goals?: GoalOutcome[] }
  | { type: 'ERROR'; runId: number; message: string }
  | { type: 'PAUSED'; runId: number }
  | { type: 'CANCELED'; runId: number }
  | { type: 'CONFIG_ERROR'; runId: number; message: string }
  | { type: 'CONFIG_APPLIED'; runId: number }
  | { type: 'LIVE_CYCLE'; runId: number; p95LatencyUs: number | null; p99LatencyUs: number | null }

