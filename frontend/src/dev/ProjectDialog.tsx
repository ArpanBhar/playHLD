import { useState } from 'react'
import type { ProjectSummary, RunSummary, VersionSummary, WorkspaceSummary } from '../api/client'

interface Props {
  currentProjectId: string | null
  projects: ProjectSummary[]
  workspaces: WorkspaceSummary[]
  versions: VersionSummary[]
  runs: RunSummary[]
  name: string
  busy: boolean
  onName: (value: string) => void
  onClose: () => void
  onCreate: () => void
  onLoad: (id: string) => void
  onSaveVersion: () => void
  onLoadVersion: (id: string) => void
  onOpenDesigns: () => void
}

export default function ProjectDialog({ currentProjectId, projects, workspaces, versions, runs, name, busy, onName, onClose, onCreate, onLoad, onSaveVersion, onLoadVersion, onOpenDesigns }: Props) {
  const [compareIds, setCompareIds] = useState<string[]>([])
  const selectedRuns = compareIds.map(id => runs.find(run => run.id === id)).filter((run): run is RunSummary => !!run)
  const toggleRun = (id: string) => setCompareIds(ids => ids.includes(id) ? ids.filter(item => item !== id) : ids.length < 2 ? [...ids, id] : [ids[1], id])
  const comparison = selectedRuns.length === 2 ? compareRuns(selectedRuns[0], selectedRuns[1]) : null
  return <div className="dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) onClose() }}><section className="dialog project-dialog" role="dialog" aria-modal="true" aria-labelledby="projects-title">
    <button className="dialog-close" type="button" aria-label="Close projects" onClick={onClose}>×</button>
    <h2 id="projects-title">Dev projects</h2>
    <p>Save an editable draft and immutable versions in a private workspace.</p>
    <div className="project-create"><label className="field"><span>New project name</span><input value={name} maxLength={120} onChange={event => onName(event.target.value)} /></label><button className="primary-button" type="button" disabled={busy || !name.trim()} onClick={onCreate}>Create project</button></div>
    {workspaces.length > 0 && <p className="project-workspace">Workspace: {workspaces[0].name}</p>}
    <h3>Your projects</h3>
    <div className="saved-design-list project-list">{projects.length ? projects.map(project => <button key={project.id} type="button" className={project.id === currentProjectId ? 'selected' : ''} onClick={() => onLoad(project.id)}><strong>{project.name}</strong><span>{project.role} · {new Date(project.updatedAt).toLocaleString()}</span></button>) : <p>No projects yet. Create one from the current architecture.</p>}</div>
    {currentProjectId && <><div className="project-version-heading"><h3>Saved versions</h3><button className="secondary-button" type="button" disabled={busy} onClick={onSaveVersion}>Save version</button></div><div className="saved-design-list project-list">{versions.length ? versions.map(version => <button key={version.id} type="button" onClick={() => onLoadVersion(version.id)}><strong>Revision {version.revision}</strong><span>{new Date(version.createdAt).toLocaleString()} · Load as draft</span></button>) : <p>No versions saved yet.</p>}</div></>}
    {currentProjectId && <><h3>Verified runs</h3><p className="compare-help">Select two runs to compare. First selected is the baseline.</p><div className="saved-design-list project-list">{runs.length ? runs.map(run => <button type="button" aria-pressed={compareIds.includes(run.id)} className={`project-run ${compareIds.includes(run.id) ? 'selected' : ''}`} onClick={() => toggleRun(run.id)} key={run.id}><strong>{Math.round(run.summary.succeeded / Math.max(1, run.summary.generated) * 100)}% success · {run.summary.p95LatencyUs == null ? 'no p95' : `${(run.summary.p95LatencyUs / 1000).toFixed(1)} ms p95`}</strong><span>{new Date(run.createdAt).toLocaleString()} · {run.provenance} · version {run.versionId.slice(0, 8)}</span></button>) : <p>No saved runs yet. Run a simulation, then save it from the metrics panel.</p>}</div>{comparison && <div className="run-comparison"><strong>Run comparison</strong><p>{comparison.comparable ? 'Same workload and engine version.' : 'Workload or engine version differs; deltas are descriptive only.'}</p>{comparison.rows.map(row => <div key={row.label}><span>{row.label}</span><b>{row.delta}</b></div>)}</div>}</>}
    <button className="dialog-switch" type="button" onClick={onOpenDesigns}>Open personal designs</button>
  </section></div>
}

function compareRuns(baseline: RunSummary, candidate: RunSummary) {
  const comparable = baseline.engineVersion === candidate.engineVersion && JSON.stringify(baseline.workload) === JSON.stringify(candidate.workload)
  const rate = (run: RunSummary) => run.summary.succeeded * 1_000_000 / run.workload.durationUs
  const failure = (run: RunSummary) => (run.summary.failed + run.summary.unfinished) * 100 / Math.max(1, run.summary.generated)
  const values = [
    { label: 'Throughput', a: rate(baseline), b: rate(candidate), unit: 'req/s' },
    { label: 'P95 latency', a: baseline.summary.p95LatencyUs == null ? null : baseline.summary.p95LatencyUs / 1000, b: candidate.summary.p95LatencyUs == null ? null : candidate.summary.p95LatencyUs / 1000, unit: 'ms' },
    { label: 'Failed or unfinished', a: failure(baseline), b: failure(candidate), unit: '%' },
  ]
  return { comparable, rows: values.map(value => {
    if (value.a === null || value.b === null) return { label: value.label, delta: 'Not measurable' }
    const delta = value.b - value.a
    const percent = value.a === 0 ? 'no % baseline' : `${delta >= 0 ? '+' : ''}${(delta / value.a * 100).toFixed(1)}%`
    return { label: value.label, delta: `${delta >= 0 ? '+' : ''}${delta.toFixed(1)} ${value.unit} (${percent})` }
  }) }
}
