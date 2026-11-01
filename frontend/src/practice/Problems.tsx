import { useState, type PointerEvent as ReactPointerEvent } from 'react'
import { ArrowRight, Check, ChevronDown, ChevronRight, ChevronUp, RotateCcw, X } from 'lucide-react'
import type { Architecture, Challenge, GoalOutcome } from '../simulation/types'
import { resourceUsage } from './constraints'

interface SharedProps {
  challenges: Challenge[]
  selected: Challenge | null
  onSelect: (problem: Challenge) => void
}

export function ProblemsCatalog({ challenges, onSelect }: Pick<SharedProps, 'challenges' | 'onSelect'>) {
  return <main className="problems-catalog" aria-label="Problems">
    <div className="problems-catalog-inner">
      <div className="problems-intro"><h1>Problems</h1><p>Build a system from the client and API server. Add and connect components, then meet every performance target within the resource budget.</p></div>
      <div className="problems-list">
        {challenges.map((problem, index) => <button className="problem-card" type="button" key={problem.id} onClick={() => onSelect(problem)}>
          <span className="problem-card-index">{String(index + 1).padStart(2, '0')}</span>
          <span className="problem-card-content"><span className="problem-card-top"><strong>{problem.title}</strong><small>{problem.difficulty}</small></span><span className="problem-card-description">{problem.scenario}</span><span className="problem-card-meta">{problem.workload.arrivalRateRps.toLocaleString()} req/s <i /> P95 ≤ {problem.goals.find(goal => goal.id === 'p95')?.target} ms <i /> P99 ≤ {problem.goals.find(goal => goal.id === 'p99')?.target} ms</span></span>
          <ArrowRight size={18} aria-hidden="true" />
        </button>)}
      </div>
    </div>
  </main>
}

export function ProblemDescription({ selected, graph, goals, onReset, onLoadSolution, mobileExpanded, onToggleMobileExpanded, onResizeStart }: { selected: Challenge; graph: Architecture; goals: GoalOutcome[] | null; onReset: () => void; onLoadSolution: () => void; mobileExpanded: boolean; onToggleMobileExpanded: () => void; onResizeStart: (event: ReactPointerEvent<HTMLDivElement>) => void }) {
  const [tab, setTab] = useState<'description' | 'resources' | 'solution'>('description')
  const usage = resourceUsage(graph)
  const budget = selected.resourceBudget
  return <aside className={`problem-description ${mobileExpanded ? 'mobile-expanded' : ''}`} aria-label="Problem description">
    <div className="description-tabs" role="tablist" aria-label="Problem information"><button type="button" role="tab" aria-selected={tab === 'description'} className={tab === 'description' ? 'active' : ''} onClick={() => setTab('description')}>Description</button><button type="button" role="tab" aria-selected={tab === 'resources'} className={tab === 'resources' ? 'active' : ''} onClick={() => setTab('resources')}>Resources</button><button type="button" role="tab" aria-selected={tab === 'solution'} className={tab === 'solution' ? 'active' : ''} onClick={() => setTab('solution')}>Solution</button><button type="button" className="mobile-description-toggle" aria-label={mobileExpanded ? 'Collapse description' : 'Expand description'} aria-expanded={mobileExpanded} onClick={onToggleMobileExpanded}>{mobileExpanded ? <ChevronUp size={15} /> : <ChevronDown size={15} />}</button></div>
    <div className="description-scroll">{tab === 'description' ? <>
      <div className="description-topline"><span>{selected.difficulty}</span><span>{selected.workload.arrivalRateRps.toLocaleString()} req/s</span></div>
      <h1>{selected.title}</h1><p className="description-scenario">{selected.scenario}</p>
      <h2>Targets</h2><div className="description-goals">{selected.goals.map(goal => { const outcome = goals?.find(item => item.id === goal.id); return <div className={`description-goal ${outcome?.status ?? ''}`} key={goal.id}><span>{outcome?.status === 'met' ? <Check size={14} /> : outcome?.status === 'unmet' ? <X size={14} /> : '○'}</span><div>{goal.label}{outcome?.observed != null && <small>Measured {Number(outcome.observed.toFixed(1))} {outcome.unit}</small>}</div></div> })}</div>
      <h2>Starting point</h2><p>The canvas contains a client and an API server. Connect them directly or add components. Any valid design that meets every target within the budget can pass.</p>
      <details className="description-hints"><summary>Hints</summary><ol>{selected.hints.map(hint => <li key={hint}>{hint}</li>)}</ol></details>
    </> : tab === 'resources' ? <>
      <h1>Resource limits</h1><p>Each box is one component instance. Add another box to scale out, then connect a load balancer to split traffic. Component speed, queue size, and cache hit rate are fixed for this problem.</p>
      <div className="description-budget"><span>Component instances <b>{usage.nodes} / {budget.maxNodes}</b></span><span>Processing slots <b>{usage.slots} / {budget.maxSlots}</b></span><span>Slots per component <b>≤ {budget.maxProcessingSlotsPerNode}</b></span></div>
      <h2>Component allowances</h2><div className="description-kind-limits">{Object.entries(budget.maxByKind).map(([kind, limit]) => <span key={kind}>{kind.replace('_', ' ')} <b>{usage.counts[kind as keyof typeof usage.counts] ?? 0} / {limit}</b></span>)}</div>
    </> : <>
      <h1>One working design</h1><p>{selected.solution.explanation}</p>
      <p className="solution-note">This example meets the published goals. Your own design can use a different route.</p>
      <h2>Components</h2><div className="solution-components">{selected.solution.architecture.nodes.map(node => <div key={node.id}><strong>{node.label}</strong><span>{node.kind.replace('_', ' ')}{node.kind === 'source' ? '' : ` · ${node.config.processingSlots ?? 1} processing slots`}</span></div>)}</div>
      <h2>Connections</h2><div className="solution-connections">{selected.solution.architecture.edges.map(edge => <div key={edge.id}>{selected.solution.architecture.nodes.find(node => node.id === edge.source)?.label} <span>→</span> {selected.solution.architecture.nodes.find(node => node.id === edge.target)?.label}{edge.route === 'cache_miss' && <small>miss</small>}</div>)}</div>
    </>}</div>
    <div className="description-footer">{tab === 'solution' ? <button type="button" className="solution-footer-action" onClick={onLoadSolution}>Load example <ArrowRight size={13} /></button> : <span>Draft saved locally</span>}<button type="button" onClick={onReset}><RotateCcw size={13} /> Reset design</button></div>
    <div role="separator" aria-label="Resize problem description vertically" aria-orientation="horizontal" className="mobile-resizer" onPointerDown={onResizeStart} />
  </aside>
}

interface ProblemWorkspaceProps extends SharedProps {
  graph: Architecture
  drawerOpen: boolean
  onClose: () => void
}

export function ProblemWorkspace({ challenges, selected, graph, drawerOpen, onClose, onSelect }: ProblemWorkspaceProps) {
  if (!selected) return null
  const usage = resourceUsage(graph)
  const budget = selected.resourceBudget
  return <>
    <section className="problem-workspace-bar" aria-label="Current problem">
      <div className="problem-workspace-title"><strong>{selected.title}</strong><span>{selected.difficulty} · {selected.workload.arrivalRateRps.toLocaleString()} req/s</span></div>
      <div className="problem-budget-inline" title="Component instances and processing slots used">Budget <b>{usage.nodes}/{budget.maxNodes}</b> instances <b>{usage.slots}/{budget.maxSlots}</b> slots</div>
    </section>
    {drawerOpen && <div className="problems-drawer-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) onClose() }}>
      <aside id="problems-drawer" className="problems-drawer" aria-label="Problem list">
        <div className="problems-drawer-head"><strong>Problems</strong><button type="button" className="panel-toggle" onClick={onClose} aria-label="Close problems"><X size={17} /></button></div>
        <div className="problems-drawer-scroll">
          <div className="problems-drawer-list">{challenges.map((problem, index) => <button key={problem.id} type="button" className={`problem-drawer-item ${problem.id === selected.id ? 'selected' : ''}`} onClick={() => { if (problem.id !== selected.id) onSelect(problem); onClose() }}><span>{String(index + 1).padStart(2, '0')}</span><strong>{problem.title}</strong><ChevronRight size={14} /></button>)}</div>
        </div>
      </aside>
    </div>}
  </>
}
