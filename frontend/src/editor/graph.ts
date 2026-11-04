import type { Edge, Node } from '@xyflow/react'
import type { Architecture, ArchitectureEdge, ArchitectureNode, Kind, NodeConfig, NodeMetrics } from '../simulation/types'
import { getNodeHealth, type NodeHealth } from './health'

export type EditorNode = Node<{ label: string; kind: Kind; health: NodeHealth | null; configuredHitRate: number | null; sourceRps: number | null }>
export type EditorEdge = Edge

const labels: Record<Kind, string> = {
  source: 'Clients', load_balancer: 'Load balancer', api: 'API service', worker: 'Worker',
  cache: 'Cache', cdn: 'CDN', database: 'Database', object_storage: 'Object storage', message_queue: 'Message queue',
}
const kinds = Object.keys(labels) as Kind[]
const isCaching = (kind: Kind) => kind === 'cache' || kind === 'cdn'

export const initialArchitecture: Architecture = {
  schemaVersion: 3,
  nodes: [
    { id: 'client', kind: 'source', label: 'Clients', position: { x: 0, y: 190 }, config: { meanServiceTimeUs: 0, queueLimit: 0 } },
    { id: 'lb', kind: 'load_balancer', label: 'Load balancer', position: { x: 230, y: 190 }, config: { processingSlots: 100, maxInFlight: 1100, meanServiceTimeUs: 300, queueLimit: 1000, routingPolicy: 'round_robin' } },
    { id: 'api-a', kind: 'api', label: 'API A', position: { x: 460, y: 70 }, config: { processingSlots: 20, maxInFlight: 220, meanServiceTimeUs: 20_000, queueLimit: 200 } },
    { id: 'api-b', kind: 'api', label: 'API B', position: { x: 460, y: 310 }, config: { processingSlots: 20, maxInFlight: 220, meanServiceTimeUs: 20_000, queueLimit: 200 } },
    { id: 'cache', kind: 'cache', label: 'Cache', position: { x: 700, y: 190 }, config: { processingSlots: 100, maxInFlight: 1100, meanServiceTimeUs: 500, queueLimit: 1000, hitRate: 0.8 } },
    { id: 'db', kind: 'database', label: 'Database', position: { x: 940, y: 190 }, config: { processingSlots: 20, maxInFlight: 520, meanServiceTimeUs: 5000, queueLimit: 500 } },
  ],
  edges: [
    { id: 'client-lb', source: 'client', target: 'lb', route: 'always', delayUs: 0 },
    { id: 'lb-api-a', source: 'lb', target: 'api-a', route: 'always', delayUs: 0 },
    { id: 'lb-api-b', source: 'lb', target: 'api-b', route: 'always', delayUs: 0 },
    { id: 'api-a-cache', source: 'api-a', target: 'cache', route: 'always', delayUs: 0 },
    { id: 'api-b-cache', source: 'api-b', target: 'cache', route: 'always', delayUs: 0 },
    { id: 'cache-db', source: 'cache', target: 'db', route: 'cache_miss', delayUs: 0 },
  ],
}

export function toFlowNodes(architecture: Architecture, metrics?: Record<string, NodeMetrics>, sourceRps?: number): EditorNode[] {
  return architecture.nodes.map(node => ({ id: node.id, type: 'architecture', position: node.position, measured: { width: 155, height: 67 }, data: { label: node.label, kind: node.kind, health: getNodeHealth(node.kind, metrics?.[node.id]), configuredHitRate: isCaching(node.kind) ? node.config.hitRate ?? 0 : null, sourceRps: node.kind === 'source' ? sourceRps ?? null : null } }))
}

export function toFlowEdges(architecture: Architecture): EditorEdge[] {
  return architecture.edges.map(edge => ({ id: edge.id, source: edge.source, target: edge.target, type: 'smoothstep', animated: false, label: edge.route === 'always' ? undefined : edge.route.replace('_', ' ') }))
}

export function newNode(kind: Kind, index: number): ArchitectureNode {
  const id = `${kind}-${crypto.randomUUID().slice(0, 8)}`
  const serviceTime: Record<Kind, number> = { source: 0, load_balancer: 300, api: 20_000, worker: 10_000, cache: 500, cdn: 300, database: 5_000, object_storage: 5_000, message_queue: 1_000 }
  return {
    id, kind, label: labels[kind],
    position: { x: 110 + (index % 3) * 260, y: 100 + Math.floor(index / 3) * 140 },
    config: kind === 'source'
      ? { meanServiceTimeUs: 0, queueLimit: 0 }
      : { processingSlots: 10, maxInFlight: 110, meanServiceTimeUs: serviceTime[kind], queueLimit: 100, ...(isCaching(kind) ? { hitRate: 0.8 } : {}), ...(kind === 'load_balancer' ? { routingPolicy: 'round_robin' as const } : {}) },
  }
}

type LegacyNode = Omit<ArchitectureNode, 'config'> & { config: NodeConfig & { replicas?: number; concurrencyPerReplica?: number; concurrency?: number } }

// Convert saved v1 drafts into visible component instances and explicit balancers.
function migrateLegacyArchitecture(graph: { nodes: LegacyNode[]; edges: ArchitectureEdge[] }): Architecture {
  const nodeIds = new Set(graph.nodes.map(node => node.id))
  const edgeIds = new Set(graph.edges.map(edge => edge.id))
  const unique = (used: Set<string>, base: string) => {
    const stem = base.slice(0, 68)
    let id = stem, suffix = 2
    while (used.has(id)) id = `${stem}-${suffix++}`
    used.add(id)
    return id
  }
  const nodes: ArchitectureNode[] = []
  const copies = new Map<string, string[]>()
  const entryFor = new Map<string, string>()
  for (const node of graph.nodes) {
    if (node.kind === 'source') {
      nodes.push({ ...node, config: { meanServiceTimeUs: 0, queueLimit: 0 } })
      copies.set(node.id, [node.id])
      continue
    }
    const count = node.config.replicas ?? 1
    const concurrency = node.config.concurrencyPerReplica ?? node.config.concurrency ?? 1
    if (!Number.isInteger(count) || count < 1 || count > 100 || !Number.isInteger(concurrency) || concurrency < 1 || concurrency > 1000) throw new Error('Old architecture has invalid replica settings.')
    const { replicas: _replicas, concurrencyPerReplica: _oldConcurrency, concurrency: _oldSlots, ...config } = node.config
    const ids: string[] = []
    for (let index = 0; index < count; index++) {
      const id = index === 0 ? node.id : unique(nodeIds, `${node.id}-${index + 1}`)
      ids.push(id)
      nodes.push({ ...node, id, label: count === 1 ? node.label : `${node.label} ${index + 1}`, position: { x: node.position.x, y: node.position.y + (index - (count - 1) / 2) * 110 }, config: { ...config, processingSlots: concurrency, maxInFlight: Math.min(100000, concurrency + config.queueLimit) } })
    }
    copies.set(node.id, ids)
    if (count > 1) {
      const routerId = unique(nodeIds, `${node.id}-balancer`)
      entryFor.set(node.id, routerId)
      const processingSlots = Math.min(1000, Math.max(10, count * concurrency))
      nodes.push({ id: routerId, kind: 'load_balancer', label: `${node.label} balancer`, position: { x: node.position.x - 210, y: node.position.y }, config: { processingSlots, maxInFlight: processingSlots + 1000, meanServiceTimeUs: 300, queueLimit: 1000, routingPolicy: 'round_robin' } })
    }
  }
  const edges: ArchitectureEdge[] = graph.edges.flatMap(edge => (copies.get(edge.source) ?? [edge.source]).map((source, index) => ({
    ...edge, id: index === 0 ? edge.id : unique(edgeIds, `${edge.id}-${index + 1}`), source, target: entryFor.get(edge.target) ?? edge.target,
  })))
  for (const [originalId, routerId] of entryFor) {
    for (const target of copies.get(originalId) ?? []) edges.push({ id: unique(edgeIds, `${routerId}-${target}`), source: routerId, target, route: 'always', delayUs: 0 })
  }
  if (nodes.length > 100 || edges.length > 300) throw new Error('Converted architecture exceeds the diagram size limit.')
  return { schemaVersion: 3, nodes, edges }
}

function migrateV2Architecture(graph: { nodes: LegacyNode[]; edges: ArchitectureEdge[] }): Architecture {
  return { schemaVersion: 3, nodes: graph.nodes.map(node => {
    if (node.kind === 'source') return node
    const { concurrency, ...config } = node.config
    const processingSlots = concurrency ?? 1
    return { ...node, config: { ...config, processingSlots, maxInFlight: Math.min(100000, processingSlots + config.queueLimit) } }
  }), edges: graph.edges }
}

export function parseArchitecture(value: unknown): Architecture {
  if (!value || typeof value !== 'object') throw new Error('File does not contain an architecture object.')
  const raw = value as { schemaVersion?: number; nodes?: ArchitectureNode[]; edges?: ArchitectureEdge[] }
  if (!Array.isArray(raw.nodes) || !Array.isArray(raw.edges)) throw new Error('Architecture needs nodes and connections.')
  if (raw.schemaVersion !== 1 && raw.schemaVersion !== 2 && raw.schemaVersion !== 3) throw new Error('Only architecture schema versions 1, 2, and 3 are supported.')
  const graph: Architecture = raw.schemaVersion === 1
    ? migrateLegacyArchitecture({ nodes: raw.nodes as LegacyNode[], edges: raw.edges })
    : raw.schemaVersion === 2
      ? migrateV2Architecture({ nodes: raw.nodes as LegacyNode[], edges: raw.edges })
      : { schemaVersion: 3, nodes: raw.nodes, edges: raw.edges }
  if (graph.nodes.length > 100 || graph.edges.length > 300) throw new Error('Graph exceeds the size limits.')
  const ids = new Set<string>()
  const nodes = graph.nodes.map(node => {
    if (!node || typeof node.id !== 'string' || !/^[A-Za-z0-9_-]{1,80}$/.test(node.id) || ids.has(node.id)) throw new Error('Node IDs must be unique and valid.')
    if (!kinds.includes(node.kind)) throw new Error(`Component ${node.kind} is not supported.`)
    if (!node.position || !Number.isFinite(node.position.x) || !Number.isFinite(node.position.y)) throw new Error('Node position is invalid.')
    if (!node.config || 'replicas' in node.config || 'concurrencyPerReplica' in node.config || 'concurrency' in node.config) throw new Error('Each component must use visible processing slots; legacy capacity fields are not supported in version 3.')
    if (node.kind !== 'source' && (!Number.isInteger(node.config.processingSlots) || (node.config.processingSlots ?? 0) < 1 || (node.config.processingSlots ?? 0) > 1000)) throw new Error('Each component needs 1–1000 processing slots.')
    if (node.kind !== 'source' && (!Number.isInteger(node.config.queueLimit) || node.config.queueLimit < 0 || node.config.queueLimit > 100000)) throw new Error('Queue limit must be 0–100000.')
    const maxInFlight = node.kind === 'source' ? undefined : node.config.maxInFlight ?? Math.min(100000, (node.config.processingSlots ?? 0) + node.config.queueLimit)
    if (node.kind !== 'source' && (!Number.isInteger(maxInFlight) || (maxInFlight ?? 0) < (node.config.processingSlots ?? 0) || (maxInFlight ?? 0) > Math.min(100000, (node.config.processingSlots ?? 0) + node.config.queueLimit))) throw new Error('In-flight limit must be between processing slots and processing slots plus queue limit.')
    ids.add(node.id)
    return node.kind === 'source' ? node : { ...node, config: { ...node.config, maxInFlight } }
  })
  const edgeIds = new Set<string>()
  const edges = graph.edges.map(edge => {
    if (!edge || typeof edge.id !== 'string' || edgeIds.has(edge.id) || !ids.has(edge.source) || !ids.has(edge.target) || !['always', 'cache_hit', 'cache_miss', 'weighted', 'read', 'write'].includes(edge.route)) throw new Error('Connection is invalid or has a missing endpoint.')
    edgeIds.add(edge.id)
    return edge
  })
  return { schemaVersion: 3, nodes, edges }
}

