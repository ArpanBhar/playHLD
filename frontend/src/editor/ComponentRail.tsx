import { ChevronLeft, ChevronRight, Database, Globe2, HardDrive, Layers3, Network, Rows3, Server, Workflow } from 'lucide-react'
import type { Kind } from '../simulation/types'

interface Props { onAdd: (kind: Kind) => void; allowedKinds?: Kind[]; collapsed: boolean; onToggle: () => void; onResizeStart: (event: React.PointerEvent<HTMLDivElement>) => void }

export default function ComponentRail({ onAdd, allowedKinds, collapsed, onToggle, onResizeStart }: Props) {
  const allowed = (kind: Kind) => !allowedKinds || allowedKinds.includes(kind)
  return <aside className={`component-rail ${collapsed ? 'is-collapsed' : ''}`} aria-label="Components">
    <div className="panel-heading"><h2>Components</h2><button type="button" className="panel-toggle" aria-label={collapsed ? 'Expand components' : 'Collapse components'} title={collapsed ? 'Expand components' : 'Collapse components'} onClick={onToggle}>{collapsed ? <ChevronRight size={15} /> : <ChevronLeft size={15} />}</button></div>
    {!collapsed && <div className="rail-content">
    <p className="rail-help">Add a component, then connect its ports.</p>
    <button type="button" className="component-option" disabled={!allowed('load_balancer')} onClick={() => onAdd('load_balancer')}><Network size={16} />Load balancer</button>
    <button type="button" className="component-option" disabled={!allowed('api')} onClick={() => onAdd('api')}><Server size={16} />API service</button>
    <button type="button" className="component-option" disabled={!allowed('worker')} onClick={() => onAdd('worker')}><Workflow size={16} />Worker</button>
    <button type="button" className="component-option" disabled={!allowed('cache')} onClick={() => onAdd('cache')}><Layers3 size={16} />Cache</button>
    <button type="button" className="component-option" disabled={!allowed('cdn')} onClick={() => onAdd('cdn')}><Globe2 size={16} />CDN</button>
    <button type="button" className="component-option" disabled={!allowed('database')} onClick={() => onAdd('database')}><Database size={16} />Database</button>
    <button type="button" className="component-option" disabled={!allowed('object_storage')} onClick={() => onAdd('object_storage')}><HardDrive size={16} />Object storage</button>
    <button type="button" className="component-option" disabled={!allowed('message_queue')} onClick={() => onAdd('message_queue')}><Rows3 size={16} />Message queue</button>
    <div className="rail-foot">Select a node to edit its settings.</div>
    </div>}
    {!collapsed && <div role="separator" aria-label="Resize components vertically" aria-orientation="horizontal" className="mobile-resizer" onPointerDown={onResizeStart} />}
  </aside>
}

