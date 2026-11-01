import type { Architecture, Challenge, Kind } from '../simulation/types'

export function resourceUsage(graph: Architecture) {
  const counts = {} as Record<Kind, number>
  let slots = 0
  for (const node of graph.nodes) {
    counts[node.kind] = (counts[node.kind] ?? 0) + 1
    if (node.kind !== 'source') {
      slots += node.config.processingSlots ?? 0
    }
  }
  return { nodes: graph.nodes.length, slots, counts }
}

export function problemGraphError(problem: Challenge, graph: Architecture): string | null {
  const usage = resourceUsage(graph)
  const budget = problem.resourceBudget
  if (usage.nodes > budget.maxNodes) return `Limit: ${budget.maxNodes} components.`
  if ((usage.counts.source ?? 0) !== 1 || (usage.counts.api ?? 0) < 1) return 'Keep one client and at least one API server.'
  for (const node of graph.nodes) {
    if (!problem.allowedKinds.includes(node.kind)) return `${node.kind} is unavailable for this problem.`
    const kindMax = budget.maxByKind[node.kind]
    if (kindMax !== undefined && usage.counts[node.kind] > kindMax) return `Limit: ${kindMax} ${node.kind.replace('_', ' ')} component${kindMax === 1 ? '' : 's'}.`
    if (node.kind === 'source') {
      if (node.id !== problem.workload.entryNodeId) return 'Keep the published client as the traffic entry.'
      continue
    }
    if (!Number.isInteger(node.config.processingSlots) || (node.config.processingSlots ?? 0) < 1) return 'Each component needs positive whole-number processing slots.'
    if (!Number.isInteger(node.config.maxInFlight) || (node.config.maxInFlight ?? 0) < (node.config.processingSlots ?? 0) || (node.config.maxInFlight ?? 0) > (node.config.processingSlots ?? 0) + node.config.queueLimit) return 'In-flight limit must fit between processing slots and processing slots plus queue limit.'
    const spec = problem.componentSpecs[node.kind]
    if (!spec) return `No specification for ${node.kind}.`
    if (node.config.meanServiceTimeUs !== spec.meanServiceTimeUs || node.config.queueLimit !== spec.queueLimit || (node.config.hitRate ?? 0) !== (spec.hitRate ?? 0) || (node.config.readServiceTimeUs ?? 0) !== 0 || (node.config.writeServiceTimeUs ?? 0) !== 0 || node.config.serviceTimeModel === 'exponential') return `${node.label} must use the problem's fixed performance settings.`
    if ((node.config.processingSlots ?? 0) > budget.maxProcessingSlotsPerNode) return `Limit: ${budget.maxProcessingSlotsPerNode} processing slots per component.`
  }
  if (usage.slots > budget.maxSlots) return `Processing slot budget exceeded: ${usage.slots}/${budget.maxSlots}.`
  return null
}
