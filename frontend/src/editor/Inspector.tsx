import type { ArchitectureNode, NodeConfig, WorkloadProfile } from '../simulation/types'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { getNodeHealth } from './health'
import type { NodeMetrics } from '../simulation/types'

interface Props {
  node: ArchitectureNode | undefined
  onRename: (label: string) => void
  onConfig: (field: keyof NodeConfig, value: number | string) => void
  onDelete: () => void
  rateRps: number
  durationSeconds: number
  seed: string
  readPercent: number
  onWorkload: (values: { rateRps?: number; durationSeconds?: number; seed?: string; readPercent?: number }) => void
  workloadLocked?: boolean
  profile: WorkloadProfile
  onProfile: (value: WorkloadProfile) => void
  arrivalModel: 'fixed' | 'poisson'
  onArrivalModel: (value: 'fixed' | 'poisson') => void
  metrics?: NodeMetrics
  collapsed: boolean
  onToggle: () => void
  onResizeStart: (event: React.PointerEvent<HTMLDivElement>) => void
  continuousMode?: boolean
  fixedPerformance?: boolean
  deletionLocked?: boolean
  maxProcessingSlotsPerNode?: number
  scenario: '' | 'url_shortener' | 'key_value' | 'async_jobs'
  onScenario: (value: '' | 'url_shortener' | 'key_value' | 'async_jobs') => void
  keySpace: number
  onKeySpace: (value: number) => void
}

const kindLabel = { source: 'Source', api: 'API service', worker: 'Worker', load_balancer: 'Load balancer', cache: 'Cache', cdn: 'CDN', database: 'Database', object_storage: 'Object storage', message_queue: 'Message queue' }

export default function Inspector({ node, onRename, onConfig, onDelete, rateRps, durationSeconds, seed, readPercent, onWorkload, workloadLocked = false, profile, onProfile, arrivalModel, onArrivalModel, metrics, collapsed, onToggle, onResizeStart, continuousMode = false, fixedPerformance = false, deletionLocked = false, maxProcessingSlotsPerNode, scenario, onScenario, keySpace, onKeySpace }: Props) {
  const health = node ? getNodeHealth(node.kind, metrics) : null
  return <aside className={`inspector ${collapsed ? 'is-collapsed' : ''}`} aria-label="Inspector">
    <div className="inspector-head"><div className="panel-heading"><h2>{node ? node.label : 'Workload'}</h2><button type="button" className="panel-toggle" aria-label={collapsed ? 'Expand inspector' : 'Collapse inspector'} title={collapsed ? 'Expand inspector' : 'Collapse inspector'} onClick={onToggle}>{collapsed ? <ChevronLeft size={15} /> : <ChevronRight size={15} />}</button></div>{!collapsed && <span>{node ? kindLabel[node.kind] : 'Simulation input'}</span>}</div>
    {!collapsed && <>
    {node && node.kind !== 'source' && <div className="inspector-section health-section"><h3>Simulation health</h3><div className="health-stats"><span>Processing busy <strong>{health?.load == null ? '—' : `${Math.round(health.load * 100)}%`}</strong></span><span>Queue <strong>{metrics?.queueDepth ?? '—'}</strong></span><span>Rejected <strong>{metrics?.rejected ?? '—'}</strong></span><span>Faults <strong>{metrics?.failures ?? '—'}</strong></span>{(node.kind === 'cache' || node.kind === 'cdn') && <><span>Hit rate <strong>{health?.hitRate == null ? '—' : `${Math.round(health.hitRate * 100)}%`}</strong></span><span>Miss rate <strong>{health?.missRate == null ? '—' : `${Math.round(health.missRate * 100)}%`}</strong></span></>}</div><p className="field-note">Busy processing slots in the latest interval, not measured CPU usage.</p></div>}
    {node && <div className="inspector-section">
      <label className="field"><span>Name</span><input value={node.label} maxLength={120} onChange={event => onRename(event.target.value)} /></label>
      {node.kind !== 'source' && <>
        <NumberField label="Processing slots" value={node.config.processingSlots ?? 1} min={1} max={maxProcessingSlotsPerNode ?? 1000} onChange={value => onConfig('processingSlots', value)} />
        <NumberField label="Max in-flight requests" value={node.config.maxInFlight ?? (node.config.processingSlots ?? 1) + node.config.queueLimit} min={node.config.processingSlots ?? 1} max={Math.min(100000, (node.config.processingSlots ?? 1) + node.config.queueLimit)} onChange={value => onConfig('maxInFlight', value)} />
        <p className="field-note">Processing slots are provisioned parallel capacity for this instance. The in-flight limit only controls admission to processing and its queue; raising it does not add processing capacity. Add another connected box to scale out.</p>
        <NumberField label="Service time (ms)" value={node.config.meanServiceTimeUs / 1000} min={0.001} max={300000} step={1} disabled={fixedPerformance} onChange={value => onConfig('meanServiceTimeUs', Math.round(value * 1000))} />
        <label className="field"><span>Service time model</span><select value={node.config.serviceTimeModel ?? 'constant'} disabled={fixedPerformance} onChange={event => onConfig('serviceTimeModel', event.target.value)}><option value="constant">Constant</option><option value="exponential">Exponential</option></select></label>
        <NumberField label="Failure probability (%)" value={Math.round((node.config.failureRate ?? 0) * 100)} min={0} max={100} disabled={fixedPerformance} onChange={value => onConfig('failureRate', value / 100)} />
        <NumberField label="Queue limit" value={node.config.queueLimit} min={0} max={100000} disabled={fixedPerformance} onChange={value => onConfig('queueLimit', value)} />
      </>}
      {(node.kind === 'cache' || node.kind === 'cdn') && (scenario ? <><NumberField label="Entries" value={node.config.cacheCapacity ?? 32} min={1} max={100000} disabled={fixedPerformance} onChange={value => onConfig('cacheCapacity', value)} /><NumberField label="TTL (seconds, 0 = none)" value={(node.config.cacheTtlUs ?? 0) / 1_000_000} min={0} max={300} disabled={fixedPerformance} onChange={value => onConfig('cacheTtlUs', Math.round(value * 1_000_000))} /><p className="field-note">Hit rate is measured from keys, cache capacity, TTL, and requests.</p></> : <NumberField label="Read hit rate (%)" value={Math.round((node.config.hitRate ?? 0) * 100)} min={0} max={100} disabled={fixedPerformance} onChange={value => onConfig('hitRate', value / 100)} />)}
      {(node.kind === 'database' || node.kind === 'object_storage') && <>
        <NumberField label="Read time override (ms)" value={(node.config.readServiceTimeUs ?? 0) / 1000} min={0} max={300000} disabled={fixedPerformance} onChange={value => onConfig('readServiceTimeUs', Math.round(value * 1000))} />
        <NumberField label="Write time override (ms)" value={(node.config.writeServiceTimeUs ?? 0) / 1000} min={0} max={300000} disabled={fixedPerformance} onChange={value => onConfig('writeServiceTimeUs', Math.round(value * 1000))} />
      </>}
      {node.kind === 'load_balancer' && <label className="field"><span>Routing policy</span><select value={node.config.routingPolicy ?? 'round_robin'} onChange={event => onConfig('routingPolicy', event.target.value)}><option value="round_robin">Round robin</option><option value="weighted">Weighted</option></select></label>}
      {fixedPerformance && node.kind !== 'source' && <p className="field-note">Component speed, queue size, and {scenario ? 'cache capacity' : 'cache hit rate'} are fixed by this problem.</p>}
      {deletionLocked ? <p className="field-note">This component is required. Other components can be deleted.</p> : <button type="button" className="text-danger" onClick={onDelete}>Delete component</button>}
    </div>}
    <div className="inspector-section workload-section">
      <h3>Traffic</h3>
      {workloadLocked ? <div className="fixed-workload"><span>Published workload</span><strong>{rateRps.toLocaleString()} req/s · {durationSeconds}s</strong><small>{readPercent}% reads · seed {seed}</small><p>{scenario === 'async_jobs' ? 'Submissions are acknowledged before jobs complete. ' : scenario ? `Stateful requests over ${keySpace} keys. ` : ''}Use Evaluate to check the fixed pattern and limits.</p></div> : <>
      <fieldset disabled={workloadLocked} className="workload-fields">
      <label className="field"><span>Request behavior</span><select value={scenario} onChange={event => onScenario(event.target.value as '' | 'url_shortener' | 'key_value' | 'async_jobs')}><option value="">Capacity only</option><option value="url_shortener">URL mapping (create / redirect)</option><option value="key_value">Key-value (put / get)</option><option value="async_jobs">Asynchronous jobs (submit / verdict)</option></select></label>
      {scenario && scenario !== 'async_jobs' && <NumberField label="Key space" value={keySpace} min={1} max={10000} onChange={onKeySpace} />}
      <label className="field"><span>Traffic profile</span><select value={profile} onChange={event => onProfile(event.target.value as WorkloadProfile)}><option value="steady">Steady</option><option value="ramp">Ramp up</option><option value="burst">Middle burst</option></select></label>
      <label className="field"><span>Arrival pattern</span><select value={arrivalModel} onChange={event => onArrivalModel(event.target.value as 'fixed' | 'poisson')}><option value="fixed">Even spacing</option><option value="poisson">Poisson (seeded)</option></select></label>
      <NumberField label="Arrival rate (req/s)" value={rateRps} min={1} max={100000} onChange={value => onWorkload({ rateRps: value })} />
      <NumberField label={continuousMode ? 'Pattern length (seconds)' : 'Duration (seconds)'} value={durationSeconds} min={1} max={300} onChange={value => onWorkload({ durationSeconds: value })} />
      <NumberField label="Read traffic (%)" value={readPercent} min={0} max={100} onChange={value => onWorkload({ readPercent: value })} />
      <label className="field"><span>Seed</span><input inputMode="numeric" pattern="[0-9]+" value={seed} onChange={event => onWorkload({ seed: event.target.value })} /></label>
      </fieldset>
      <p className="field-note">{workloadLocked ? 'This challenge uses a fixed workload and seed.' : continuousMode ? 'Live traffic repeats this pattern until you stop it. A fixed run uses this length for capture.' : profile === 'steady' ? 'Use the same seed to compare changes fairly.' : profile === 'ramp' ? 'Rate rises in four equal time steps to the selected rate.' : 'Rate doubles during the middle third, up to 100,000 req/s.'}</p>
      </>}
    </div>
    </>}
    {!collapsed && <div role="separator" aria-label="Resize inspector vertically" aria-orientation="horizontal" className="mobile-resizer" onPointerDown={onResizeStart} />}
  </aside>
}

function NumberField({ label, value, min, max, step = 1, disabled = false, onChange }: { label: string; value: number; min: number; max: number; step?: number; disabled?: boolean; onChange: (value: number) => void }) {
  return <label className="field"><span>{label}</span><input type="number" value={value} min={min} max={max} step={step} disabled={disabled} onChange={event => onChange(Number(event.target.value))} /></label>
}

