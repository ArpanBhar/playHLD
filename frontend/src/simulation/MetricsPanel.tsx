import { useMemo } from 'react'
import { ChevronDown, ChevronUp, Pause, Play, Square } from 'lucide-react'
import type { Result, Snapshot } from './types'

interface Props {
  status: string
  live: boolean
  warning: string | null
  liveP95LatencyUs: number | null
  liveP99LatencyUs: number | null
  snapshots: Snapshot[]
  result: Result | null
  error: string | null
  durationSeconds: number
  onPause: () => void
  onResume: () => void
  onCancel: () => void
  onSaveRun?: () => void
  savingRun?: boolean
  collapsed: boolean
  onToggle: () => void
  onResizeStart: (event: React.PointerEvent<HTMLDivElement>) => void
}

export default function MetricsPanel({ status, live, warning, liveP95LatencyUs, liveP99LatencyUs, snapshots, result, error, durationSeconds, onPause, onResume, onCancel, onSaveRun, savingRun = false, collapsed, onToggle, onResizeStart }: Props) {
  const latest = snapshots.at(-1)
  const chart = useMemo(() => {
    const endUs = live ? Math.max(60_000_000, snapshots.at(-1)?.atUs ?? 0) : durationSeconds * 1_000_000
    const startUs = live ? Math.max(0, endUs - 60_000_000) : 0
    const visible = live ? snapshots.filter(snapshot => snapshot.atUs >= startUs) : snapshots
    const maximum = Math.max(100, ...visible.map(snapshot => Math.max(snapshot.offeredRps, snapshot.throughputRps)))
    const yMaximum = Math.ceil(maximum / 100) * 100
    const points = (key: 'offeredRps' | 'throughputRps') => visible.map(snapshot => {
      const x = 42 + Math.min(1, (snapshot.atUs - startUs) / (endUs - startUs)) * 938
      const y = 89 - (snapshot[key] / yMaximum) * 75
      return `${x.toFixed(1)},${y.toFixed(1)}`
    }).join(' ')
    return { yMaximum, startSeconds: startUs / 1_000_000, spanSeconds: (endUs - startUs) / 1_000_000, offered: points('offeredRps'), throughput: points('throughputRps') }
  }, [snapshots, durationSeconds, live])
  const maxQueue = latest ? Math.max(0, ...Object.values(latest.nodeMetrics).map(node => node.maxQueueDepth)) : 0
  return <section className={`metrics-panel ${collapsed ? 'is-collapsed' : ''}`} aria-label="Simulation metrics">
    <div className="metrics-heading"><h2>Simulation metrics</h2><div className="metrics-actions">{onSaveRun && <button className="secondary-button" type="button" disabled={status !== 'completed' || savingRun} onClick={onSaveRun}>{savingRun ? 'Saving run…' : 'Save verified run'}</button>}<span className={`status-label status-${status}`}>{status === 'ready' ? 'Ready to simulate' : status}</span><button type="button" className="panel-toggle" aria-label={collapsed ? 'Expand simulation metrics' : 'Collapse simulation metrics'} onClick={onToggle}>{collapsed ? <ChevronUp size={15} /> : <ChevronDown size={15} />}</button></div></div>
    {!collapsed && <>
    <div className="metric-grid">
      <Metric label="Throughput" value={latest ? Math.round(latest.throughputRps).toLocaleString() : '—'} unit="req/s" />
      <Metric label="P95 latency" value={(result?.summary.p95LatencyUs ?? liveP95LatencyUs) == null ? live ? '…' : '—' : ((result?.summary.p95LatencyUs ?? liveP95LatencyUs)! / 1000).toFixed(1)} unit="ms" />
      <Metric label="P99 latency" value={(result?.summary.p99LatencyUs ?? liveP99LatencyUs) == null ? live ? '…' : '—' : ((result?.summary.p99LatencyUs ?? liveP99LatencyUs)! / 1000).toFixed(1)} unit="ms" />
      <Metric label="Max queue depth" value={latest ? maxQueue.toLocaleString() : '—'} unit="requests" />
      <Metric label="Failed" value={result ? result.summary.failed.toLocaleString() : latest ? latest.failed.toLocaleString() : '—'} unit="requests" />
    </div>
    {result && result.summary.jobsEnqueued > 0 && <div className="job-summary"><span>Jobs enqueued <strong>{result.summary.jobsEnqueued.toLocaleString()}</strong></span><span>Verdicts stored <strong>{result.summary.jobsCompleted.toLocaleString()}</strong></span><span>Jobs failed <strong>{result.summary.jobsFailed.toLocaleString()}</strong></span><span>Jobs unfinished <strong>{result.summary.jobsUnfinished.toLocaleString()}</strong></span></div>}
    <div className="chart-and-note">
      <div className="chart-wrap" role="img" aria-label="Offered and completed requests per second over simulated time">
        {snapshots.length > 0 ? <svg className="chart-svg" viewBox="0 0 1000 115" preserveAspectRatio="none" aria-hidden="true">
          {[0, 1, 2, 3].map(index => <g key={index}><line x1="42" x2="980" y1={14 + index * 25} y2={14 + index * 25} stroke="#414141" strokeDasharray="3 4" /><text x="37" y={17 + index * 25} textAnchor="end">{Math.round(chart.yMaximum * (1 - index / 3))}</text></g>)}
          <line x1="42" x2="980" y1="89" y2="89" stroke="#646464" />
          {[0, 1, 2, 3, 4].map(index => <text key={index} x={42 + index * 234.5} y="108" textAnchor={index === 0 ? 'start' : index === 4 ? 'end' : 'middle'}>{Math.round(chart.startSeconds + chart.spanSeconds * index / 4)}s</text>)}
          <polyline points={chart.offered} fill="none" stroke="#929292" strokeWidth="2" vectorEffect="non-scaling-stroke" />
          <polyline points={chart.throughput} fill="none" stroke="#2bc464" strokeWidth="2.5" vectorEffect="non-scaling-stroke" />
        </svg> : <div className="chart-empty">Run a simulation to see traffic over time.</div>}
      </div>
      <div className="chart-legend"><span><i className="legend-dot offered" />Offered</span><span><i className="legend-dot throughput" />Throughput</span><p>Live simulation animates sampled requests. Red: processing busy ≥85% or cache miss ≥60%.</p></div>
    </div>
    <div className="playback-bar">
      {status === 'running' && live ? <button type="button" className="icon-button" aria-label="Pause simulation" onClick={onPause}><Pause size={15} /></button> : status === 'paused' && live ? <button type="button" className="icon-button" aria-label="Resume simulation" onClick={onResume}><Play size={15} /></button> : <span className="flow-caption">{status === 'running' ? 'Evaluating…' : status === 'completed' ? 'Evaluation complete' : 'Run simulation to animate requests'}</span>}
      <button type="button" className="icon-button" aria-label="Cancel simulation" onClick={onCancel} disabled={status !== 'running' && status !== 'paused'}><Square size={15} /></button>
      <div className={`playback-progress ${live ? 'live-progress' : ''} ${status === 'paused' || status === 'stopped' ? 'is-paused' : ''}`}><span style={live ? undefined : { width: `${Math.min(100, ((latest?.atUs || 0) / (durationSeconds * 1_000_000)) * 100)}%` }} /></div>
      <span className="playback-time">{latest ? `${(latest.atUs / 1_000_000).toFixed(0)}s` : '0s'}</span>
    </div>
    {error && <p className="error-message" role="alert">{error}</p>}
    {warning && <p className="warning-message" role="status">{warning}</p>}
    {result?.warnings?.length ? <p className="warning-message">{result.warnings.join(' ')}</p> : null}
    {result?.functionalTrace?.length ? <div className="functional-results"><h3>Functional requests</h3><p>{result.summary.correct.toLocaleString()} correct · {result.summary.incorrect.toLocaleString()} incorrect · first {result.functionalTrace.length} completed requests shown</p><div className="functional-trace-list">{result.functionalTrace.map(trace => <div key={trace.requestId} className={trace.correct ? 'trace-correct' : 'trace-incorrect'}><strong>#{trace.requestId} {trace.operation} {trace.key}</strong><span>{trace.path.join(' → ')}</span><small>Output: {trace.output} · Expected: {trace.expected}</small></div>)}</div></div> : null}
    </>}
    {!collapsed && <div role="separator" aria-label="Resize simulation metrics vertically" aria-orientation="horizontal" className="mobile-resizer" onPointerDown={onResizeStart} />}
  </section>
}

function Metric({ label, value, unit }: { label: string; value: string; unit: string }) {
  return <div className="metric"><span>{label}</span><strong>{value}</strong><small>{unit}</small></div>
}

