import { describe, expect, it } from 'vitest'
import { parseArchitecture } from './graph'

describe('architecture migration', () => {
  it('turns hidden v1 API replicas into visible instances behind a balancer', () => {
    const graph = parseArchitecture({
      schemaVersion: 1,
      nodes: [
        { id: 'client', kind: 'source', label: 'Clients', position: { x: 0, y: 0 }, config: { queueLimit: 0 } },
        { id: 'api', kind: 'api', label: 'API', position: { x: 300, y: 0 }, config: { replicas: 2, concurrencyPerReplica: 3, meanServiceTimeUs: 2000, queueLimit: 10 } },
      ],
      edges: [{ id: 'client-api', source: 'client', target: 'api', route: 'always', delayUs: 0 }],
    })
    const balancer = graph.nodes.find(node => node.kind === 'load_balancer')
    expect(graph.schemaVersion).toBe(3)
    expect(balancer).toBeDefined()
    expect(graph.nodes.filter(node => node.kind === 'api')).toHaveLength(2)
    expect(graph.nodes.filter(node => node.kind === 'api').map(node => node.config.processingSlots)).toEqual([3, 3])
    expect(graph.nodes.filter(node => node.kind === 'api').map(node => node.config.maxInFlight)).toEqual([13, 13])
    expect(graph.edges.find(edge => edge.source === 'client')?.target).toBe(balancer?.id)
    expect(graph.edges.filter(edge => edge.source === balancer?.id)).toHaveLength(2)
    expect(JSON.stringify(graph)).not.toMatch(/replicas|concurrencyPerReplica|"concurrency"/)
  })

  it('converts v2 concurrency into processing slots and an independent admission limit', () => {
    const graph = parseArchitecture({ schemaVersion: 2, nodes: [
      { id: 'api', kind: 'api', label: 'API', position: { x: 0, y: 0 }, config: { concurrency: 3, meanServiceTimeUs: 2000, queueLimit: 10 } },
    ], edges: [] })
    expect(graph.nodes[0].config).toMatchObject({ processingSlots: 3, maxInFlight: 13 })
  })

  it('rejects the old capacity field in v3 diagrams', () => {
    expect(() => parseArchitecture({ schemaVersion: 3, nodes: [
      { id: 'api', kind: 'api', label: 'API', position: { x: 0, y: 0 }, config: { concurrency: 3, processingSlots: 3, maxInFlight: 13, meanServiceTimeUs: 2000, queueLimit: 10 } },
    ], edges: [] })).toThrow('legacy capacity fields')
  })

  it('defaults a missing v3 admission limit without changing provisioned slots', () => {
    const graph = parseArchitecture({ schemaVersion: 3, nodes: [
      { id: 'api', kind: 'api', label: 'API', position: { x: 0, y: 0 }, config: { processingSlots: 4, meanServiceTimeUs: 2000, queueLimit: 10 } },
    ], edges: [] })
    expect(graph.nodes[0].config).toMatchObject({ processingSlots: 4, maxInFlight: 14 })
  })
})
