import { memo } from 'react'
import { Handle, Position, type NodeProps } from '@xyflow/react'
import { ArrowRightFromLine, Database, Globe2, HardDrive, Layers3, Network, Rows3, Server, Workflow } from 'lucide-react'
import type { EditorNode } from './graph'

function ArchitectureNode({ data, selected }: NodeProps<EditorNode>) {
  const isSource = data.kind === 'source'
  const icon = { source: ArrowRightFromLine, api: Server, worker: Workflow, load_balancer: Network, cache: Layers3, cdn: Globe2, database: Database, object_storage: HardDrive, message_queue: Rows3 }[data.kind]
  const Icon = icon
  const kindLabel = { source: 'Source', api: 'API service', worker: 'Worker', load_balancer: 'Load balancer', cache: 'Cache', cdn: 'CDN', database: 'Database', object_storage: 'Object storage', message_queue: 'Message queue' }[data.kind]
  const health = data.health
  const caching = data.kind === 'cache' || data.kind === 'cdn'
  const mainMetric = caching
    ? health?.missRate == null ? `Target miss ${Math.round((1 - (data.configuredHitRate ?? 0)) * 100)}%` : `Miss ${Math.round(health.missRate * 100)}%`
    : data.kind === 'source' ? data.sourceRps == null ? null : `${Math.round(data.sourceRps).toLocaleString()} req/s` : health?.load == null ? 'Busy —' : `Busy ${Math.round(health.load * 100)}%`
  return (
    <div className={`architecture-node ${selected ? 'selected' : ''}`} style={health ? { '--health-color': health.color, '--health-tint': health.tint } as React.CSSProperties : undefined} title={health ? `${mainMetric}; queue ${health.queue}; rejected ${health.rejected}${health.hitRate == null ? '' : `; hit ${Math.round(health.hitRate * 100)}%`}` : undefined}>
      {!isSource && <Handle type="target" position={data.kind === 'database' ? Position.Top : Position.Left} />}
      <span className="node-icon" aria-hidden="true"><Icon size={16} strokeWidth={1.8} /></span>
      <span className="node-name">{data.label}</span>
      <span className="node-kind">{kindLabel}</span>
      {mainMetric && <span className="node-health"><span className="node-health-indicator" />{mainMetric}{caching && health?.hitRate != null && <span className="node-health-extra"> · Hit {Math.round(health.hitRate * 100)}%</span>}</span>}
      <Handle type="source" position={caching ? Position.Bottom : Position.Right} />
    </div>
  )
}

export default memo(ArchitectureNode)

