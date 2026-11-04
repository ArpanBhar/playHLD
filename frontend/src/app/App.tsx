import { lazy, Suspense, useCallback, useEffect, useRef, useState, type CSSProperties, type PointerEvent as ReactPointerEvent } from 'react'
import type { Connection } from '@xyflow/react'
import { flushSync } from 'react-dom'
import { ArrowDownToLine, ArrowUpFromLine, ChevronDown, ChevronUp, FolderOpen, PanelLeftOpen, Pause, Play, Redo2, Save, Square, Undo2 } from 'lucide-react'
import { useLocation, useNavigate, useNavigationType } from 'react-router-dom'
import { apiRequest, type Account, type SavedDesign, type DesignSummary, type WorkspaceSummary, type ProjectSummary, type ProjectDetails, type VersionSummary, type RunSummary } from '../api/client'
import ProjectDialog from '../dev/ProjectDialog'
import ComponentRail from '../editor/ComponentRail'
import LandingPage from '../landing/LandingPage'
import { loadWorkspaceLayout, saveWorkspaceLayout, type WorkspaceLayout } from './workspaceLayout'
import Inspector from '../editor/Inspector'
import { ProblemsCatalog, ProblemWorkspace, ProblemDescription } from '../practice/Problems'
import { problemGraphError, resourceUsage } from '../practice/constraints'
import { initialArchitecture, newNode, parseArchitecture } from '../editor/graph'
import MetricsPanel from '../simulation/MetricsPanel'
import { useSimulation } from '../simulation/useSimulation'
import { compileTraffic } from '../simulation/workload'
import type { Architecture, ArchitectureEdge, Challenge, GoalOutcome, Kind, NodeConfig, Result, WorkloadProfile } from '../simulation/types'

type Mode = 'Practice' | 'Dev' | 'Contest'
const EditorCanvas = lazy(() => import('../editor/EditorCanvas'))
const localDraftKey = 'playhld:draft:v3'
const practiceDraftKey = (id: string, version: number) => `playhld:practice:${id}:v${version}`
const legacyPracticeDraftKey = (id: string, version: number) => `systemforge:practice:${id}:v${version}`

function loadDraft(): Architecture {
  for (const key of [localDraftKey, 'systemforge:draft:v3', 'systemforge:draft:v2', 'systemforge:draft:v1']) {
    try {
      const raw = localStorage.getItem(key)
      if (raw) return parseArchitecture(JSON.parse(raw))
    } catch { /* Try an older compatible draft. */ }
  }
  return initialArchitecture
}

export default function App() {
  const navigate = useNavigate()
  const navigationType = useNavigationType()
  const transitionTo = (destination: string) => {
    if (window.location.pathname === destination) return
    if (document.startViewTransition && !window.matchMedia('(prefers-reduced-motion: reduce)').matches) {
      document.startViewTransition(() => flushSync(() => navigate(destination)))
    } else navigate(destination)
  }
  const location = useLocation()
  const path = location.pathname
  const page = path === '/' ? 'home' : path === '/problems' ? 'problems' : path.startsWith('/problems/') ? 'practice-editor' : path === '/practice' || path.startsWith('/practice/') ? 'legacy-practice' : path === '/dev' ? 'dev' : path === '/contest' ? 'contest' : 'missing'
  const routeProblemId = page === 'practice-editor' ? decodeURIComponent(path.slice('/problems/'.length)) : null
  const [initialLayout] = useState(loadWorkspaceLayout)
  const [panelSizes, setPanelSizes] = useState(initialLayout.panelSizes)
  const [descriptionWidth, setDescriptionWidth] = useState(initialLayout.descriptionWidth)
  const [mobileHeights, setMobileHeights] = useState(initialLayout.mobileHeights)
  const [collapsed, setCollapsed] = useState(initialLayout.collapsed)
  const [descriptionExpanded, setDescriptionExpanded] = useState(initialLayout.descriptionExpanded)
  const layoutToSave = useRef<WorkspaceLayout>(initialLayout)
  const [problemsOpen, setProblemsOpen] = useState(false)
  const workspaceRef = useRef<HTMLElement>(null)
  const studioRef = useRef<HTMLDivElement>(null)
  const [mode, setMode] = useState<Mode>(path.startsWith('/problems') || path.startsWith('/practice') ? 'Practice' : path === '/contest' ? 'Contest' : 'Dev')
  const [architecture, setArchitecture] = useState<Architecture>(loadDraft)
  const architectureRef = useRef(architecture)
  const devArchitectureRef = useRef(architecture)
  const devWorkloadRef = useRef({ rateRps: 500, durationSeconds: 30, readPercent: 100, profile: 'steady' as WorkloadProfile, arrivalModel: 'fixed' as 'fixed' | 'poisson', seed: '42', scenario: '' as '' | 'url_shortener' | 'key_value' | 'async_jobs', keySpace: 64 })
  const dragStartRef = useRef<Architecture | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>('api')
  const [rateRps, setRateRps] = useState(500)
  const [durationSeconds, setDurationSeconds] = useState(30)
  const [readPercent, setReadPercent] = useState(100)
  const [profile, setProfile] = useState<WorkloadProfile>('steady')
  const [arrivalModel, setArrivalModel] = useState<'fixed' | 'poisson'>('fixed')
  const [seed, setSeed] = useState('42')
  const [scenario, setScenario] = useState<'' | 'url_shortener' | 'key_value' | 'async_jobs'>('')
  const [keySpace, setKeySpace] = useState(64)
  const [notice, setNotice] = useState<string | null>(null)
  const [account, setAccount] = useState<Account | null>(null)
  const [authOpen, setAuthOpen] = useState(false)
  const [authMode, setAuthMode] = useState<'login' | 'register'>('login')
  const [authEmail, setAuthEmail] = useState('')
  const [authPassword, setAuthPassword] = useState('')
  const [authError, setAuthError] = useState<string | null>(null)
  const [authBusy, setAuthBusy] = useState(false)
  const [designsOpen, setDesignsOpen] = useState(false)
  const [savedDesigns, setSavedDesigns] = useState<DesignSummary[]>([])
  const [designName, setDesignName] = useState('Untitled architecture')
  const [remoteId, setRemoteId] = useState<string | null>(null)
  const [remoteVersion, setRemoteVersion] = useState<number | null>(null)
  const [projectsOpen, setProjectsOpen] = useState(false)
  const [projects, setProjects] = useState<ProjectSummary[]>([])
  const [workspaces, setWorkspaces] = useState<WorkspaceSummary[]>([])
  const [versions, setVersions] = useState<VersionSummary[]>([])
  const [runs, setRuns] = useState<RunSummary[]>([])
  const [newProjectName, setNewProjectName] = useState('New architecture project')
  const [projectId, setProjectId] = useState<string | null>(null)
  const [projectRevision, setProjectRevision] = useState<number | null>(null)
  const [projectSavedGraph, setProjectSavedGraph] = useState<string | null>(null)
  const [projectBusy, setProjectBusy] = useState(false)
  const [runBusy, setRunBusy] = useState(false)
  const [versionId, setVersionId] = useState<string | null>(null)
  const [versionRevision, setVersionRevision] = useState<number | null>(null)
  const [lastRunInput, setLastRunInput] = useState<{ rateRps: number; durationSeconds: number; readPercent: number; seed: string; profile: WorkloadProfile; arrivalModel: 'fixed' | 'poisson' } | null>(null)
  const [history, setHistory] = useState<Architecture[]>([])
  const [future, setFuture] = useState<Architecture[]>([])
  const [challenges, setChallenges] = useState<Challenge[]>([])
  const [challengeLoadState, setChallengeLoadState] = useState<'loading' | 'ready' | 'error'>('loading')
  const challengesRequested = useRef(false)
  const [challenge, setChallenge] = useState<Challenge | null>(null)
  const [lastRunGraph, setLastRunGraph] = useState<string | null>(null)
  const [lastRunSignature, setLastRunSignature] = useState<string | null>(null)
  const [verifiedGoals, setVerifiedGoals] = useState<GoalOutcome[] | null>(null)
  const [verifying, setVerifying] = useState(false)
  const fileInput = useRef<HTMLInputElement>(null)
  const simulation = useSimulation(page === 'dev' || page === 'practice-editor')
  const liveSignature = JSON.stringify({ nodes: architecture.nodes.map(node => ({ id: node.id, kind: node.kind, config: node.config })), edges: architecture.edges, rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace })
  const currentRunSignature = JSON.stringify({ architecture, rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace })
  const fixedRunCurrent = lastRunSignature === currentRunSignature
  const displayStatus = simulation.live || fixedRunCurrent || simulation.status === 'loading' ? simulation.status : 'ready'
  const liveSignatureRef = useRef(liveSignature)
  const selected = architecture.nodes.find(node => node.id === selectedId)
  const problemUsage = resourceUsage(architecture)
  const addableProblemKinds = mode === 'Practice' && challenge ? challenge.allowedKinds.filter(kind => kind !== 'source' && problemUsage.nodes < challenge.resourceBudget.maxNodes && (problemUsage.counts[kind] ?? 0) < (challenge.resourceBudget.maxByKind[kind] ?? Infinity)) : undefined
  const currentRunGoals = fixedRunCurrent && simulation.status === 'completed' ? verifiedGoals || simulation.goals : null
  const displaySnapshots = simulation.live || fixedRunCurrent ? simulation.snapshots.length ? simulation.snapshots : simulation.result?.snapshots ?? [] : []
  const displayIndex = simulation.status === 'completed'
    ? Math.max(0, displaySnapshots.map(snapshot => snapshot.atUs <= (lastRunInput?.durationSeconds ?? durationSeconds) * 1_000_000).lastIndexOf(true))
    : displaySnapshots.length - 1
  const latestSnapshot = displaySnapshots[displayIndex]
  const previousSnapshot = displaySnapshots[displayIndex - 1]
  const selectedWindowMetrics = selectedId ? latestSnapshot?.nodeMetrics[selectedId] : undefined
  const selectedFinalMetrics = selectedId && fixedRunCurrent && simulation.status === 'completed' ? displaySnapshots.at(-1)?.nodeMetrics[selectedId] : undefined
  const selectedMetrics = selectedWindowMetrics && selectedFinalMetrics ? { ...selectedWindowMetrics, completions: selectedFinalMetrics.completions, rejected: selectedFinalMetrics.rejected, failures: selectedFinalMetrics.failures, cacheHits: selectedFinalMetrics.cacheHits, cacheLookups: selectedFinalMetrics.cacheLookups, maxQueueDepth: selectedFinalMetrics.maxQueueDepth } : selectedWindowMetrics

  useEffect(() => {
    layoutToSave.current = { panelSizes, descriptionWidth, mobileHeights, collapsed, descriptionExpanded }
    const timer = window.setTimeout(() => saveWorkspaceLayout(layoutToSave.current), 180)
    return () => window.clearTimeout(timer)
  }, [panelSizes, descriptionWidth, mobileHeights, collapsed, descriptionExpanded])

  useEffect(() => {
    const flushLayout = () => saveWorkspaceLayout(layoutToSave.current)
    window.addEventListener('pagehide', flushLayout)
    return () => window.removeEventListener('pagehide', flushLayout)
  }, [])

  useEffect(() => {
    if (!simulation.live) { liveSignatureRef.current = liveSignature; return }
    if (liveSignature === liveSignatureRef.current) return
    liveSignatureRef.current = liveSignature
    const timer = window.setTimeout(() => simulation.updateLive(architecture, rateRps, durationSeconds, seed, readPercent, profile, arrivalModel, mode === 'Practice' ? challenge?.workload.segments : undefined, scenario ? { scenario, keySpace } : undefined), 120)
    return () => window.clearTimeout(timer)
  }, [liveSignature, simulation.live, simulation.updateLive])

  useEffect(() => {
    if (lastRunSignature && lastRunSignature !== currentRunSignature && !simulation.live) {
      simulation.clear()
      setLastRunSignature(null)
      setLastRunGraph(null)
      setVerifiedGoals(null)
    }
  }, [currentRunSignature, lastRunSignature, simulation.live, simulation.clear])

  const togglePanel = (panel: keyof typeof collapsed) => setCollapsed(current => ({ ...current, [panel]: !current[panel] }))
  const beginResize = (panel: keyof typeof panelSizes, event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return
    event.preventDefault()
    const start = panel === 'metrics' ? event.clientY : event.clientX
    const initial = panelSizes[panel]
    const workspace = workspaceRef.current?.getBoundingClientRect()
    const move = (pointer: PointerEvent) => {
      const delta = (panel === 'inspector' || panel === 'metrics' ? -1 : 1) * ((panel === 'rail' || panel === 'inspector' ? pointer.clientX : pointer.clientY) - start)
      const maximum = panel === 'rail' ? Math.max(130, (workspace?.width ?? 1200) - (collapsed.inspector ? 34 : panelSizes.inspector) - 280)
        : panel === 'inspector' ? Math.max(190, (workspace?.width ?? 1200) - (collapsed.rail ? 34 : panelSizes.rail) - 280)
          : Math.max(170, (workspace?.height ?? 700) - 170)
      const minimum = panel === 'rail' ? 132 : panel === 'inspector' ? 190 : 160
      setPanelSizes(current => ({ ...current, [panel]: Math.round(Math.max(minimum, Math.min(maximum, initial + delta))) }))
    }
    const stop = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', stop); document.body.classList.remove('resizing-panels') }
    document.body.classList.add('resizing-panels')
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }
  const beginMobileResize = (panel: keyof typeof mobileHeights, event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0) return
    event.preventDefault()
    const start = event.clientY
    const initial = mobileHeights[panel]
    const minimum = panel === 'rail' ? 50 : panel === 'editor' ? 250 : panel === 'description' ? 130 : 110
    const maximum = panel === 'rail' ? 240 : panel === 'editor' ? 900 : 700
    const move = (pointer: PointerEvent) => setMobileHeights(current => ({ ...current, [panel]: Math.round(Math.max(minimum, Math.min(maximum, initial + pointer.clientY - start))) }))
    const stop = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', stop); document.body.classList.remove('resizing-panels') }
    document.body.classList.add('resizing-panels')
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }
  const beginDescriptionResize = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || !studioRef.current) return
    event.preventDefault()
    const rect = studioRef.current.getBoundingClientRect()
    const move = (pointer: PointerEvent) => {
      const availableWidth = rect.width - 16
      const minimum = 260
      const maximum = Math.max(minimum, availableWidth - 416)
      const width = Math.max(minimum, Math.min(maximum, pointer.clientX - rect.left - 8))
      setDescriptionWidth(Math.round(width / availableWidth * 1000) / 10)
    }
    const stop = () => {
      window.removeEventListener('pointermove', move)
      window.removeEventListener('pointerup', stop)
      document.body.classList.remove('resizing-panels')
    }
    document.body.classList.add('resizing-panels')
    window.addEventListener('pointermove', move)
    window.addEventListener('pointerup', stop, { once: true })
  }

  useEffect(() => {
    if (mode !== 'Dev') return
    devArchitectureRef.current = architecture
    try { localStorage.setItem(localDraftKey, JSON.stringify(architecture)) }
    catch { setNotice('Browser storage is full; export the graph to keep it.') }
  }, [architecture, mode])

  useEffect(() => {
    if (mode === 'Dev') devWorkloadRef.current = { rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace }
  }, [mode, rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace])

  useEffect(() => {
    if (mode !== 'Practice' || !challenge) return
    try { localStorage.setItem(practiceDraftKey(challenge.id, challenge.version), JSON.stringify(architecture)) }
    catch { setNotice('Browser storage is full; export the graph to keep it.') }
  }, [architecture, challenge, mode])

  useEffect(() => {
    if (page !== 'problems' && page !== 'practice-editor') return
    if (challengesRequested.current) return
    challengesRequested.current = true
    void apiRequest<{ items: Challenge[] }>('/api/practice/challenges').then(page => { setChallenges(page.items); setChallengeLoadState('ready') }).catch(() => { setChallengeLoadState('error'); setNotice('Problems could not be loaded.') })
  }, [page])

  useEffect(() => {
    if (!document.cookie.split('; ').some(item => item.startsWith('sf_csrf='))) return
    void apiRequest<Account>('/api/auth/me').then(setAccount).catch(() => setAccount(null))
  }, [])

  const submitAuth = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();setAuthBusy(true);setAuthError(null)
    try {
      const user = await apiRequest<Account>(`/api/auth/${authMode}`, 'POST', { email: authEmail, password: authPassword })
      setAccount(user);setAuthOpen(false);setAuthPassword('');setNotice(`Signed in as ${user.email}. You can save this draft to your account.`)
    } catch (error) { setAuthError(error instanceof Error ? error.message : 'Sign-in failed.') }
    finally { setAuthBusy(false) }
  }

  const signOut = async () => {
    try { await apiRequest('/api/auth/logout', 'POST');setAccount(null);setRemoteId(null);setRemoteVersion(null);setProjectId(null);setProjectRevision(null);setProjectSavedGraph(null);setNotice('Signed out. This browser still holds the current local draft.') }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Could not sign out.') }
  }

  const saveDraft = async () => {
    const draftKey = mode === 'Practice' && challenge ? practiceDraftKey(challenge.id, challenge.version) : localDraftKey
    try { localStorage.setItem(draftKey, JSON.stringify(architecture)) }
    catch { setNotice('Browser storage is full; export the graph to keep it.');return }
    if (!account) { setNotice('Draft saved in this browser. Sign in to save it to your account.');return }
    if (mode === 'Dev' && projectId && projectRevision !== null) {
      if (projectSavedGraph === JSON.stringify(architecture)) { setNotice('Project draft is already saved.'); return }
      try {
        const saved = await apiRequest<{ revision: number }>(`/api/projects/${projectId}/draft`, 'PUT', { architecture, expectedRevision: projectRevision })
        setProjectRevision(saved.revision); setProjectSavedGraph(JSON.stringify(architecture)); setVersionId(null); setVersionRevision(null); setNotice(`Project draft saved at revision ${saved.revision}.`)
      } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save project draft.') }
      return
    }
    try {
      const saved = remoteId
        ? await apiRequest<SavedDesign>(`/api/designs/${remoteId}`, 'PUT', { name: designName, architecture, expectedVersion: remoteVersion })
        : await apiRequest<SavedDesign>('/api/designs', 'POST', { name: designName, architecture })
      setRemoteId(saved.id);setRemoteVersion(saved.version);setNotice(`Saved ${saved.name} to your account (version ${saved.version}).`)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save design.') }
  }

  const openDesigns = async () => {
    if (!account) { setNotice('Sign in to browse saved designs.');return }
    try { const page=await apiRequest<{items:DesignSummary[]}>('/api/designs?limit=50');setSavedDesigns(page.items);setDesignsOpen(true) }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Could not load designs.') }
  }

  const loadDesign = async (id: string) => {
    try {
      const saved=await apiRequest<SavedDesign>(`/api/designs/${id}`)
      const graph=parseArchitecture(saved.architecture)
      commit(() => graph);setSelectedId(null);setDesignName(saved.name);setRemoteId(saved.id);setRemoteVersion(saved.version);setProjectId(null);setProjectRevision(null);setProjectSavedGraph(null);setVersionId(null);setVersionRevision(null);setDesignsOpen(false);setMode('Dev');transitionTo('/dev');setNotice(`Loaded ${saved.name}.`)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not load design.') }
  }

  const commit = useCallback((edit: (current: Architecture) => Architecture) => {
    const current = architectureRef.current
    const next = edit(current)
    if (next === current) return
    architectureRef.current = next
    setArchitecture(next)
    setHistory(items => [...items.slice(-49), current])
    setFuture([])
  }, [])

  const openProjects = async () => {
    if (!account) { setAuthOpen(true); setNotice('Sign in to use Dev projects.'); return }
    try {
      const [workspacePage, projectPage] = await Promise.all([apiRequest<{items:WorkspaceSummary[]}>('/api/workspaces'), apiRequest<{items:ProjectSummary[]}>('/api/projects')])
      setWorkspaces(workspacePage.items); setProjects(projectPage.items)
      if (projectId) {
        const [versionsPage, runsPage] = await Promise.all([apiRequest<{items:VersionSummary[]}>(`/api/projects/${projectId}/versions`), apiRequest<{items:RunSummary[]}>(`/api/projects/${projectId}/runs`)])
        setVersions(versionsPage.items); setRuns(runsPage.items)
      }
      setProjectsOpen(true)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not load projects.') }
  }

  const loadProject = async (id: string) => {
    try {
      const [project, versionPage, runPage] = await Promise.all([apiRequest<ProjectDetails>(`/api/projects/${id}`), apiRequest<{items:VersionSummary[]}>(`/api/projects/${id}/versions`), apiRequest<{items:RunSummary[]}>(`/api/projects/${id}/runs`)])
      const graph = parseArchitecture(project.branch.architecture)
      architectureRef.current = graph; devArchitectureRef.current = graph; setArchitecture(graph)
      setProjectId(project.id); setProjectRevision(project.branch.revision); setProjectSavedGraph(JSON.stringify(project.branch.architecture))
      setVersions(versionPage.items); setRuns(runPage.items)
      const currentVersion = versionPage.items.find(item => item.revision === project.branch.revision)
      setVersionId(currentVersion?.id || null); setVersionRevision(currentVersion?.revision ?? null)
      setRemoteId(null); setRemoteVersion(null); setDesignName(project.name)
      setHistory([]); setFuture([]); setSelectedId(null); setMode('Dev'); transitionTo('/dev'); setProjectsOpen(false)
      setNotice(`Loaded project ${project.name}.`)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not load project.') }
  }

  const createProject = async () => {
    if (!account || !newProjectName.trim()) return
    setProjectBusy(true)
    try {
      let workspace = workspaces[0]
      if (!workspace) {
        workspace = await apiRequest<WorkspaceSummary>('/api/workspaces', 'POST', { name: 'Personal workspace' })
        setWorkspaces([workspace])
      }
      const project = await apiRequest<ProjectDetails>('/api/projects', 'POST', { workspaceId: workspace.id, name: newProjectName.trim(), architecture })
      setProjects(items => [{ id: project.id, workspaceId: workspace.id, name: project.name, description: project.description, updatedAt: new Date().toISOString(), role: 'owner' }, ...items])
      setProjectId(project.id); setProjectRevision(project.branch.revision); setProjectSavedGraph(JSON.stringify(architecture))
      setDesignName(project.name); setRemoteId(null); setRemoteVersion(null); setVersions([]); setRuns([]); setVersionId(null); setVersionRevision(null); setProjectsOpen(false); setMode('Dev'); transitionTo('/dev')
      setNotice(`Created project ${project.name}. Save a version to preserve this revision.`)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not create project.') }
    finally { setProjectBusy(false) }
  }

  const ensureVersion = async (): Promise<string> => {
    if (!projectId || projectRevision === null) throw new Error('Create a Dev project first.')
    let revision = projectRevision
    if (projectSavedGraph !== JSON.stringify(architecture)) {
      const saved = await apiRequest<{revision:number}>(`/api/projects/${projectId}/draft`, 'PUT', { architecture, expectedRevision: revision })
      revision = saved.revision; setProjectRevision(revision); setProjectSavedGraph(JSON.stringify(architecture)); setVersionId(null); setVersionRevision(null)
    } else if (versionId && versionRevision === revision) return versionId
    const version = await apiRequest<VersionSummary>(`/api/projects/${projectId}/versions`, 'POST', { expectedRevision: revision })
    setVersions(items => [version, ...items]); setVersionId(version.id); setVersionRevision(revision)
    return version.id
  }

  const saveVersion = async () => {
    setProjectBusy(true)
    try { await ensureVersion(); setNotice('This project revision has an immutable saved version.') }
    catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save version.') }
    finally { setProjectBusy(false) }
  }

  const saveRun = async () => {
    if (!projectId || !lastRunInput || !simulation.result || simulation.status !== 'completed' || !fixedRunCurrent) return
    if (lastRunGraph !== JSON.stringify(architecture) || lastRunInput.rateRps !== rateRps || lastRunInput.durationSeconds !== durationSeconds || lastRunInput.readPercent !== readPercent || lastRunInput.seed !== seed || lastRunInput.profile !== profile || lastRunInput.arrivalModel !== arrivalModel) {
      setNotice('Architecture or workload changed since this run. Run the simulation again before saving.'); return
    }
    setRunBusy(true)
    try {
      const savedVersionID = await ensureVersion()
      const segments = compileTraffic(lastRunInput.profile, lastRunInput.durationSeconds, lastRunInput.rateRps)
      const workload = { schemaVersion: 1, seed: lastRunInput.seed, entryNodeId: architecture.nodes.find(node => node.kind === 'source')?.id, durationUs: lastRunInput.durationSeconds * 1_000_000, drainTimeoutUs: 5_000_000, arrivalRateRps: lastRunInput.rateRps, readFraction: lastRunInput.readPercent / 100, ...(segments ? { segments } : {}), ...(lastRunInput.arrivalModel === 'poisson' ? { arrivalModel: 'poisson' as const } : {}), ...(scenario ? { scenario, keySpace } : {}) }
      const saved = await apiRequest<{id:string;result:Result;versionId:string;createdAt:string;configHash:string;engineVersion:string;provenance:string}>(`/api/projects/${projectId}/runs`, 'POST', { versionId: savedVersionID, workload })
      setRuns(items => items.some(item => item.id === saved.id) ? items : [{ id: saved.id, versionId: saved.versionId, engineVersion: saved.engineVersion, configHash: saved.configHash, provenance: saved.provenance, createdAt: saved.createdAt, workload, summary: saved.result.summary }, ...items])
      setNotice(JSON.stringify(saved.result.summary) === JSON.stringify(simulation.result.summary) ? 'Verified run saved with its immutable graph version and workload.' : 'Server run saved, but its metrics differ from the browser run. Review engine versions and inputs.')
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not save run.') }
    finally { setRunBusy(false) }
  }

  const loadVersion = async (id: string) => {
    if (!projectId) return
    try {
      const version = await apiRequest<{architecture:Architecture;revision:number}>(`/api/projects/${projectId}/versions/${id}`)
      const graph = parseArchitecture(version.architecture)
      commit(() => graph); setSelectedId(null); setProjectsOpen(false)
      setNotice(`Loaded revision ${version.revision} as an editable draft. Save it to create a new revision.`)
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not load version.') }
  }

  const selectChallenge = (item: Challenge, updateRoute = true) => {
    simulation.clear()
    if (mode === 'Dev') {
      devArchitectureRef.current = architectureRef.current
      devWorkloadRef.current = { rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace }
    }
    let graph = parseArchitecture(item.starter)
    for (const key of [practiceDraftKey(item.id, item.version), legacyPracticeDraftKey(item.id, item.version)]) {
      try {
        const saved = localStorage.getItem(key)
        if (!saved) continue
        const draft = parseArchitecture(JSON.parse(saved))
        if (!problemGraphError(item, draft)) { graph = draft; break }
      } catch { /* Try the older key if this draft is damaged. */ }
    }
    architectureRef.current = graph; setArchitecture(graph); setHistory([]); setFuture([])
    setChallenge(item); setMode('Practice'); setProblemsOpen(false); setSelectedId(graph.nodes.find(node => node.kind !== 'source')?.id || null)
    setRateRps(item.workload.arrivalRateRps); setDurationSeconds(item.workload.durationUs / 1_000_000)
    setReadPercent((item.workload.readFraction ?? 1) * 100); setSeed(item.workload.seed); setProfile('steady'); setArrivalModel('fixed')
    setScenario(item.workload.scenario ?? ''); setKeySpace(item.workload.keySpace ?? 64)
    setLastRunGraph(null); setLastRunSignature(null); setVerifiedGoals(null); setNotice(null)
    if (updateRoute) transitionTo(`/problems/${encodeURIComponent(item.id)}`)
  }

  const resetChallenge = () => {
    if (!challenge) return
    simulation.clear()
    const graph = parseArchitecture(challenge.starter)
    architectureRef.current = graph; setArchitecture(graph); setHistory([]); setFuture([])
    setSelectedId(graph.nodes.find(node => node.kind !== 'source')?.id || null)
    setLastRunGraph(null); setLastRunSignature(null); setVerifiedGoals(null)
    localStorage.removeItem(practiceDraftKey(challenge.id, challenge.version))
    localStorage.removeItem(legacyPracticeDraftKey(challenge.id, challenge.version))
  }

  const loadSolution = () => {
    if (!challenge) return
    const graph = parseArchitecture(challenge.solution.architecture)
    simulation.clear()
    commit(() => graph)
    setSelectedId(graph.nodes.find(node => node.kind === 'api')?.id ?? null)
    setLastRunGraph(null); setLastRunSignature(null)
    setVerifiedGoals(null)
    setNotice('Example design loaded. You can change it and evaluate the goals.')
  }

  const changeMode = (next: Mode, updateRoute = true) => {
    if (next !== mode || next === 'Practice') simulation.clear()
    if (next === 'Dev' && mode === 'Practice') {
      architectureRef.current = devArchitectureRef.current; setArchitecture(devArchitectureRef.current)
      setSelectedId(null); setHistory([]); setFuture([]); setLastRunGraph(null)
      const saved = devWorkloadRef.current
      setRateRps(saved.rateRps); setDurationSeconds(saved.durationSeconds); setReadPercent(saved.readPercent); setProfile(saved.profile); setArrivalModel(saved.arrivalModel); setSeed(saved.seed); setScenario(saved.scenario); setKeySpace(saved.keySpace)
    }
    if (next === 'Practice') { setChallenge(null); setProblemsOpen(false) }
    setMode(next)
    if (next === 'Contest') setNotice('Contest mode is still under development.')
    else setNotice(null)
    if (updateRoute) transitionTo(next === 'Practice' ? '/problems' : next === 'Dev' ? '/dev' : '/contest')
  }

  useEffect(() => {
    if (page === 'legacy-practice') {
      navigate(path === '/practice' ? '/problems' : `/problems/${path.slice('/practice/'.length)}`, { replace: true })
    } else if (page === 'practice-editor') {
      const item = challenges.find(candidate => candidate.id === routeProblemId)
      if (item && (mode !== 'Practice' || challenge?.id !== item.id)) selectChallenge(item, false)
      else if (!item && challengeLoadState === 'ready') navigate('/problems', { replace: true })
    } else if (page === 'problems') {
      if (mode !== 'Practice' || challenge) changeMode('Practice', false)
    } else if (page === 'dev') {
      if (mode !== 'Dev') changeMode('Dev', false)
    } else if (page === 'contest') {
      if (mode !== 'Contest') changeMode('Contest', false)
    } else if (page === 'home' || page === 'missing') {
      if (simulation.status === 'running' || simulation.status === 'paused') simulation.clear()
      setProblemsOpen(false)
    }
  }, [path, challenges, challengeLoadState])

  const verifyAttempt = async () => {
    if (!challenge || !currentRunGoals) return
    if (!account) { setAuthOpen(true); setNotice('Sign in to verify and save your attempt.'); return }
    setVerifying(true)
    try {
      const attempt = await apiRequest<{ id: string; goals: GoalOutcome[]; result: Result; completed: boolean }>(`/api/practice/challenges/${challenge.id}/attempts`, 'POST', { architecture })
      setVerifiedGoals(attempt.goals)
      setNotice(attempt.completed ? 'Challenge completed and saved with a server-verified run.' : 'Server-verified attempt saved. Keep iterating on the architecture.')
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not verify attempt.') }
    finally { setVerifying(false) }
  }

  const addNode = (kind: Kind) => {
    if (kind === 'source') { setNotice('The client is the single traffic source for this diagram.'); return }
    if (mode === 'Practice' && challenge && !challenge.allowedKinds.includes(kind)) return
    const spec = mode === 'Practice' ? challenge?.componentSpecs[kind] : undefined
    const generated = newNode(kind, architecture.nodes.length)
    const node = { ...generated, config: spec ? { ...generated.config, processingSlots: 1, maxInFlight: 1 + spec.queueLimit, meanServiceTimeUs: spec.meanServiceTimeUs, queueLimit: spec.queueLimit, ...((kind === 'cache' || kind === 'cdn') ? { hitRate: spec.hitRate ?? 0, cacheCapacity: spec.cacheCapacity ?? 0, cacheTtlUs: spec.cacheTtlUs ?? 0 } : {}) } : generated.config, position: { x: Math.max(0, ...architecture.nodes.map(item => item.position.x)) + 230, y: selected?.position.y ?? 170 } }
    if (mode === 'Practice' && challenge) {
      const error = problemGraphError(challenge, { ...architecture, nodes: [...architecture.nodes, node] })
      if (error) { setNotice(error); return }
    }
    commit(current => ({ ...current, nodes: [...current.nodes, node] }))
    setSelectedId(node.id)
  }

  const connect = (connection: Connection) => {
    if (!connection.source || !connection.target || connection.source === connection.target) return
    const sourceNode=architecture.nodes.find(node=>node.id===connection.source)
    const targetNode=architecture.nodes.find(node=>node.id===connection.target)
    if (!sourceNode||!targetNode||targetNode.kind==='source') {setNotice('Choose a processing component as the target.');return}
    const existing=architecture.edges.filter(edge=>edge.source===connection.source)
    if (existing.some(edge => edge.target === connection.target)) { setNotice('These components are already connected.'); return }
    const caching = sourceNode.kind === 'cache' || sourceNode.kind === 'cdn'
    const conditionalService = scenario && (sourceNode.kind === 'api' || sourceNode.kind === 'worker')
    if (sourceNode.kind!=='load_balancer'&&sourceNode.kind!=='message_queue'&&!caching&&!conditionalService&&existing.length>0) {setNotice('This component already has an outgoing route.');return}
    if (conditionalService && existing.length >= 2) { setNotice('A service can have one read route and one write route.'); return }
    if (caching&&existing.length>=2) {setNotice('A cache or CDN can have one hit route and one miss route.');return}
    const id = `edge-${crypto.randomUUID().slice(0, 8)}`
    const route: ArchitectureEdge['route']=caching ? existing.some(edge=>edge.route==='cache_miss') ? 'cache_hit' : 'cache_miss' : sourceNode.kind==='load_balancer'&&sourceNode.config.routingPolicy==='weighted' ? 'weighted' : conditionalService && existing.length ? existing[0].route === 'read' ? 'write' : existing[0].route === 'write' ? 'read' : targetNode.kind === 'cache' || targetNode.kind === 'cdn' ? 'read' : 'write' : 'always'
    commit(current => {
      const added = { id, source: connection.source!, target: connection.target!, route, delayUs: 0, ...(route==='weighted'?{weight:1/(existing.length+1)}:{}) }
      const edges = [...current.edges.map(edge => conditionalService && existing.length && edge.id === existing[0].id && edge.route === 'always' ? { ...edge, route: route === 'read' ? 'write' as const : 'read' as const } : edge), added]
      if (route === 'weighted') {
        const count = edges.filter(edge => edge.source === connection.source).length
        return { ...current, edges: edges.map(edge => edge.source === connection.source ? { ...edge, weight: 1 / count } : edge) }
      }
      return { ...current, edges }
    })
  }

  const moveNode = (id: string, position: { x: number; y: number }) => {
    const current = architectureRef.current
    const next = { ...current, nodes: current.nodes.map(node => node.id === id ? { ...node, position } : node) }
    architectureRef.current = next
    setArchitecture(next)
  }

  const moveStop = (id: string, position: { x: number; y: number }) => {
    moveNode(id, position)
    const before = dragStartRef.current
    dragStartRef.current = null
    if (before && before.nodes.find(node => node.id === id)?.position.x !== position.x || before && before.nodes.find(node => node.id === id)?.position.y !== position.y) {
      setHistory(items => [...items.slice(-49), before!])
      setFuture([])
    }
  }

  const removeNode = (id: string) => {
    if (architecture.nodes.find(node => node.id === id)?.kind === 'source' || mode === 'Practice' && architecture.nodes.find(node => node.id === id)?.kind === 'api' && architecture.nodes.filter(node => node.kind === 'api').length === 1) { setNotice('Keep the client and at least one API server for a problem. Other components can be removed.'); return }
    commit(current => ({ ...current, nodes: current.nodes.filter(node => node.id !== id), edges: current.edges.filter(edge => edge.source !== id && edge.target !== id) }))
    if (selectedId === id) setSelectedId(null)
  }

  const renameNode = (label: string) => commit(current => ({ ...current, nodes: current.nodes.map(node => node.id === selectedId ? { ...node, label } : node) }))
  const changeConfig = (field: keyof NodeConfig, value: number | string) => commit(current => {
    if (mode === 'Practice' && field !== 'processingSlots' && field !== 'maxInFlight' && field !== 'routingPolicy') return current
    if ((field === 'processingSlots' || field === 'maxInFlight' || field === 'queueLimit' || field === 'cacheCapacity') && (!Number.isInteger(Number(value)) || Number(value) < (field === 'queueLimit' ? 0 : 1))) return current
    if ((field === 'failureRate' || field === 'hitRate') && (!Number.isFinite(Number(value)) || Number(value) < 0 || Number(value) > 1)) return current
    if (field === 'cacheTtlUs' && (!Number.isInteger(Number(value)) || Number(value) < 0 || Number(value) > 300_000_000)) return current
    if (field === 'processingSlots' && Number(value) > 1000 || field === 'queueLimit' && Number(value) > 100000) return current
    const next = {
      ...current,
      nodes: current.nodes.map(node => {
        if (node.id !== selectedId) return node
        const config = { ...node.config, [field]: value }
        const slots = field === 'processingSlots' ? Number(value) : node.config.processingSlots ?? 1
        const queue = field === 'queueLimit' ? Number(value) : node.config.queueLimit
        const oldMax = node.config.maxInFlight ?? (node.config.processingSlots ?? 1) + node.config.queueLimit
        if (field === 'maxInFlight' && (Number(value) < slots || Number(value) > Math.min(100000, slots + queue))) return node
        if (field === 'processingSlots') config.maxInFlight = oldMax === (node.config.processingSlots ?? 1) + node.config.queueLimit ? Math.min(100000, slots + queue) : Math.min(100000, slots + queue, Math.max(slots, oldMax))
        if (field === 'queueLimit') config.maxInFlight = Math.min(oldMax, slots + queue)
        return { ...node, config }
      }),
      edges: field === 'routingPolicy' ? current.edges.map(edge => edge.source === selectedId ? { ...edge, route: value === 'weighted' ? 'weighted' as const : 'always' as const, weight: value === 'weighted' ? 1 / current.edges.filter(item => item.source === selectedId).length : undefined } : edge) : current.edges,
    }
    if (mode === 'Practice' && challenge) {
      const error = problemGraphError(challenge, next)
      if (error) { setNotice(error); return current }
    }
    return next
  })

  const removeEdge = (id: string) => commit(current => {
    const removed = current.edges.find(edge => edge.id === id)
    if (!removed) return current
    const edges = current.edges.filter(edge => edge.id !== id)
    if (current.nodes.find(node => node.id === removed.source)?.config.routingPolicy !== 'weighted') return { ...current, edges }
    const count = edges.filter(edge => edge.source === removed.source).length
    return { ...current, edges: edges.map(edge => edge.source === removed.source ? { ...edge, weight: count ? 1 / count : undefined } : edge) }
  })

  const changeEdgeRoute = (id: string, route: ArchitectureEdge['route']) => commit(current => {
    const edge = current.edges.find(item => item.id === id)
    if (!edge || !['always', 'read', 'write'].includes(route)) return current
    const siblings = current.edges.filter(item => item.source === edge.source && item.id !== id)
    if (siblings.length && route === 'always') { setNotice('Two service routes must be read and write.'); return current }
    return { ...current, edges: current.edges.map(item => item.id === id ? { ...item, route } : siblings.some(sibling => sibling.id === item.id) && item.route === route ? { ...item, route: route === 'read' ? 'write' : 'read' } as ArchitectureEdge : item) }
  })

  const undo = () => {
    if (!history.length) return
    const previous = history.at(-1)!
    setHistory(items => items.slice(0, -1))
    setFuture(items => [...items, architecture])
    architectureRef.current = previous
    setArchitecture(previous)
  }

  const redo = () => {
    if (!future.length) return
    const next = future.at(-1)!
    setFuture(items => items.slice(0, -1))
    setHistory(items => [...items, architecture])
    architectureRef.current = next
    setArchitecture(next)
  }

  const exportGraph = () => {
    const bundle = { format: 'playhld-workspace-v1', architecture, workload: { rateRps, durationSeconds, readPercent, profile, arrivalModel, seed, scenario, keySpace } }
    const blob = new Blob([JSON.stringify(bundle, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const anchor = document.createElement('a')
    anchor.href = url
    anchor.download = 'playHLD-workspace.json'
    anchor.click()
    setTimeout(() => URL.revokeObjectURL(url), 1000)
  }

  const importGraph = async (file: File | undefined) => {
    if (!file) return
    if (file.size > 1_048_576) { setNotice('The graph file is larger than 1 MiB.'); return }
    try {
      const parsed = JSON.parse(await file.text())
      const graph = parseArchitecture(parsed?.architecture ?? parsed)
      commit(() => graph)
      if (parsed?.format === 'playhld-workspace-v1' && parsed.workload && mode === 'Dev') {
        const w = parsed.workload
        if (Number.isInteger(w.rateRps) && w.rateRps >= 1 && w.rateRps <= 100000) setRateRps(w.rateRps)
        if (Number.isInteger(w.durationSeconds) && w.durationSeconds >= 1 && w.durationSeconds <= 300) setDurationSeconds(w.durationSeconds)
        if (Number.isInteger(w.readPercent) && w.readPercent >= 0 && w.readPercent <= 100) setReadPercent(w.readPercent)
        if (w.profile === 'steady' || w.profile === 'ramp' || w.profile === 'burst') setProfile(w.profile)
        if (w.arrivalModel === 'fixed' || w.arrivalModel === 'poisson') setArrivalModel(w.arrivalModel)
        if (typeof w.seed === 'string' && /^\d+$/.test(w.seed)) setSeed(w.seed)
        if (w.scenario === '' || w.scenario === 'url_shortener' || w.scenario === 'key_value' || w.scenario === 'async_jobs') setScenario(w.scenario)
        if (Number.isInteger(w.keySpace) && w.keySpace >= 1 && w.keySpace <= 10000) setKeySpace(w.keySpace)
      }
      setSelectedId(null)
      setNotice('Workspace imported. Run simulation to validate routes and capacity.')
    } catch (error) { setNotice(error instanceof Error ? error.message : 'Could not import graph.') }
    if (fileInput.current) fileInput.current.value = ''
  }

  const run = (fixed = false) => {
    if (mode === 'Practice' && !challenge) { setNotice('Choose a challenge first.'); return }
    if (mode === 'Practice' && challenge) { const error = problemGraphError(challenge, architecture); if (error) { setNotice(error); return } }
    const effectiveRate = mode === 'Practice' && challenge ? challenge.workload.arrivalRateRps : rateRps
    const effectiveDuration = mode === 'Practice' && challenge ? challenge.workload.durationUs / 1_000_000 : durationSeconds
    const effectiveRead = mode === 'Practice' && challenge ? (challenge.workload.readFraction ?? 1) * 100 : readPercent
    const effectiveSeed = mode === 'Practice' && challenge ? challenge.workload.seed : seed
    if (!Number.isInteger(effectiveRate) || effectiveRate < 1 || effectiveRate > 100_000 || !Number.isInteger(effectiveDuration) || effectiveDuration < 1 || effectiveDuration > 300 || !Number.isInteger(effectiveRead) || effectiveRead < 0 || effectiveRead > 100 || !/^\d+$/.test(effectiveSeed)) {
      setNotice('Enter a valid rate, duration, read percentage, and decimal seed before running.')
      return
    }
    setNotice(null)
    setVerifiedGoals(null); setLastRunGraph(JSON.stringify(architecture)); setLastRunSignature(currentRunSignature)
    setLastRunInput({ rateRps: effectiveRate, durationSeconds: effectiveDuration, readPercent: effectiveRead, seed: effectiveSeed, profile, arrivalModel })
    const segments = mode === 'Practice' ? challenge?.workload.segments : compileTraffic(profile, effectiveDuration, effectiveRate)
    const functional = scenario ? { scenario, keySpace, hotKeyFraction: mode === 'Practice' ? challenge?.workload.hotKeyFraction ?? 0.8 : 0.8 } : undefined
    if (fixed) simulation.run(architecture, effectiveRate, effectiveDuration, effectiveSeed, effectiveRead, mode === 'Practice' ? challenge?.goals : undefined, mode === 'Practice' ? challenge?.workload.drainTimeoutUs : undefined, segments, mode === 'Practice' ? 'fixed' : arrivalModel, functional)
    else simulation.startLive(architecture, effectiveRate, effectiveDuration, effectiveSeed, effectiveRead, profile, mode === 'Practice' ? 'fixed' : arrivalModel, mode === 'Practice' ? challenge?.workload.segments : undefined, functional)
  }

  const editorPage = page === 'dev' || page === 'practice-editor' && challenge?.id === routeProblemId
  const practiceEditor = page === 'practice-editor' && challenge?.id === routeProblemId
  const showBrowseNav = page === 'home' || page === 'problems'
  const openPage = (destination: string) => transitionTo(destination)

  if (page === 'legacy-practice') return <div className="app-shell" />
  if (page === 'practice-editor' && !editorPage) return challengeLoadState === 'error'
    ? <div className="app-shell"><main className="portal-page"><div><h1>Problems unavailable</h1><p>The problem list could not be loaded.</p><button type="button" className="secondary-button" onClick={() => openPage('/problems')}>View problems</button></div></main></div>
    : <div className="app-shell" />

  return <>
    <div className={`app-shell page-${page} ${navigationType === 'POP' ? 'page-pop' : ''}`}>
      <header className={`app-header ${showBrowseNav ? 'browse-header' : ''} ${simulation.status === 'running' || simulation.status === 'paused' ? 'simulation-active' : ''}`}>
        <button type="button" className="brand-mark" aria-label="playHLD home" title="playHLD home" onClick={() => openPage('/')}><img src="/favicon.svg" alt="" /></button>
        {showBrowseNav && <nav className="page-nav" aria-label="Pages"><button type="button" className={page === 'problems' ? 'active' : ''} aria-current={page === 'problems' ? 'page' : undefined} onClick={() => openPage('/problems')}>Problems</button><button type="button" onClick={() => openPage('/dev')}>Dev</button><button type="button" onClick={() => openPage('/contest')}>Contest</button></nav>}
        {practiceEditor && <button type="button" className="header-problems-button" aria-expanded={problemsOpen} aria-controls="problems-drawer" onClick={() => setProblemsOpen(value => !value)}><PanelLeftOpen size={16} />Problems</button>}
        {page !== 'home' && page !== 'problems' && <strong className="header-page-title">{page === 'practice-editor' ? challenge?.title ?? 'Problem' : page === 'dev' ? designName : page === 'contest' ? 'Contest' : ''}</strong>}
        {editorPage && <div className="header-run-controls" aria-label="Simulation controls">
          <button type="button" className="run-icon-button" aria-label={simulation.status === 'running' ? 'Pause simulation' : simulation.status === 'paused' ? 'Resume simulation' : 'Run simulation'} title={simulation.status === 'running' ? 'Pause simulation' : simulation.status === 'paused' ? 'Resume simulation' : 'Run simulation'} onClick={() => simulation.status === 'running' ? simulation.pause() : simulation.status === 'paused' ? simulation.resume() : run()} disabled={simulation.status === 'loading'}>{simulation.status === 'running' ? <Pause size={17} /> : <Play size={17} fill="currentColor" />}</button>
          {(simulation.status === 'running' || simulation.status === 'paused') && <button type="button" className="run-icon-button" aria-label="Stop simulation" title="Stop simulation" onClick={simulation.cancel}><Square size={15} fill="currentColor" /></button>}
          <button type="button" className="submit-run-button" onClick={() => run(true)} disabled={!simulation.live && simulation.status === 'running'}>Evaluate</button>
        </div>}
        <div className="header-spacer" />
        {editorPage && <button type="button" className="header-save" onClick={() => void saveDraft()} title="Save draft"><Save size={15} /><span>Save</span></button>}
        {practiceEditor && currentRunGoals && <button type="button" className="header-verify" onClick={() => void verifyAttempt()} disabled={verifying}>{verifying ? 'Verifying…' : 'Verify & save'}</button>}
        {account ? <button type="button" className="account-button" title={account.email} onClick={() => void signOut()}>Sign out</button> : <button type="button" className="account-button" onClick={() => setAuthOpen(true)}>Sign in</button>}
      </header>

      {page === 'home' ? <LandingPage onNavigate={openPage} />
      : page === 'problems' ? <ProblemsCatalog key={path} challenges={challenges} onSelect={selectChallenge} />
      : page === 'contest' ? <main key={path} className="portal-page"><div><h1>Contest</h1><p>Contest mode is under development.</p><button type="button" className="secondary-button" onClick={() => openPage('/')}>Back home</button></div></main>
      : editorPage ? <div key={path} ref={studioRef} className={`studio-layout ${practiceEditor ? 'practice-layout' : 'dev-layout'}`} style={practiceEditor ? { '--description-width': `${descriptionWidth}%`, '--mobile-description-height': `${mobileHeights.description}px` } as CSSProperties : undefined}>
      {practiceEditor && challenge && <ProblemDescription selected={challenge} graph={architecture} goals={currentRunGoals} onReset={resetChallenge} onLoadSolution={loadSolution} mobileExpanded={descriptionExpanded} onToggleMobileExpanded={() => setDescriptionExpanded(value => !value)} onResizeStart={event => { setDescriptionExpanded(false); beginMobileResize('description', event) }} />}
      {practiceEditor && <div className="description-resizer" role="separator" aria-label="Resize problem description" aria-orientation="vertical" aria-valuenow={descriptionWidth} tabIndex={0} onPointerDown={beginDescriptionResize} onKeyDown={event => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
        event.preventDefault()
        const width = studioRef.current?.getBoundingClientRect().width ?? 1200
        const min = 260 / (width - 16) * 100
        const max = Math.max(min, (width - 432) / (width - 16) * 100)
        setDescriptionWidth(current => Math.max(min, Math.min(max, current + (event.key === 'ArrowRight' ? 2 : -2))))
      }}><span aria-hidden="true" /></div>}
      <div className="studio-main">
      {practiceEditor && <ProblemWorkspace challenges={challenges} selected={challenge} graph={architecture} drawerOpen={problemsOpen} onClose={() => setProblemsOpen(false)} onSelect={selectChallenge} />}
      <main ref={workspaceRef} className="workspace" style={{ '--rail-width': `${collapsed.rail ? 34 : panelSizes.rail}px`, '--inspector-width': `${collapsed.inspector ? 34 : panelSizes.inspector}px`, '--metrics-height': `${collapsed.metrics ? 36 : panelSizes.metrics}px`, '--mobile-rail-height': `${mobileHeights.rail}px`, '--mobile-editor-height': `${mobileHeights.editor}px`, '--mobile-inspector-height': `${mobileHeights.inspector}px`, '--mobile-metrics-height': `${mobileHeights.metrics}px` } as CSSProperties}>
        <ComponentRail onAdd={addNode} allowedKinds={addableProblemKinds} collapsed={collapsed.rail} onToggle={() => togglePanel('rail')} onResizeStart={event => beginMobileResize('rail', event)} />
        <section className={`editor-main ${collapsed.editor ? 'is-collapsed' : ''}`} aria-label="Architecture editor">
          <div className="editor-toolbar">
            <span className="editor-title">Architecture</span>
            <span className="editor-subtitle">{architecture.nodes.length} components · {architecture.edges.length} {architecture.edges.length === 1 ? 'route' : 'routes'}</span>
            {simulation.warning && <span className="editor-live-warning" title={simulation.warning}>Connect component to apply live</span>}
            <div className="toolbar-spacer" />
            <button type="button" className="toolbar-button" aria-label={collapsed.editor ? 'Expand architecture canvas' : 'Collapse architecture canvas'} title={collapsed.editor ? 'Expand canvas' : 'Collapse canvas'} onClick={() => togglePanel('editor')}>{collapsed.editor ? <ChevronDown size={15} /> : <ChevronUp size={15} />}</button>
            {mode === 'Dev' && <button type="button" className="toolbar-projects" aria-label="Projects" onClick={() => void openProjects()}><FolderOpen size={15} /><span>Projects</span></button>}
            <button type="button" className="toolbar-button" title="Undo" aria-label="Undo" disabled={!history.length} onClick={undo}><Undo2 size={17} /></button>
            <button type="button" className="toolbar-button" title="Redo" aria-label="Redo" disabled={!future.length} onClick={redo}><Redo2 size={17} /></button>
            <span className="toolbar-divider" />
            <input ref={fileInput} type="file" accept="application/json,.json" hidden onChange={event => void importGraph(event.target.files?.[0])} />
            <button type="button" className="toolbar-button" title="Import JSON" aria-label="Import JSON" onClick={() => fileInput.current?.click()}><ArrowUpFromLine size={17} /></button>
            <button type="button" className="toolbar-button" title="Export JSON" aria-label="Export JSON" onClick={exportGraph}><ArrowDownToLine size={17} /></button>
          </div>
          {collapsed.editor ? <div className="canvas-collapsed">Architecture canvas collapsed</div> : <Suspense fallback={<div className="canvas-wrap" />}><EditorCanvas architecture={architecture} selectedId={selectedId} onSelect={setSelectedId} onMove={moveNode} onMoveStart={() => { dragStartRef.current = architectureRef.current }} onMoveStop={moveStop} onConnect={connect} onDeleteNode={removeNode} onDeleteEdge={removeEdge} onChangeEdgeRoute={changeEdgeRoute} functionalRouting={!!scenario} snapshot={latestSnapshot} previousSnapshot={previousSnapshot} flowActive={simulation.live && (simulation.status === 'running' || simulation.status === 'paused')} flowPaused={simulation.status === 'paused'} /></Suspense>}
          {!collapsed.editor && <div role="separator" aria-label="Resize architecture canvas" aria-orientation="horizontal" className="mobile-resizer" onPointerDown={event => beginMobileResize('editor', event)} />}
        </section>
        <Inspector node={selected} metrics={selectedMetrics} collapsed={collapsed.inspector} onToggle={() => togglePanel('inspector')} onResizeStart={event => beginMobileResize('inspector', event)} continuousMode={mode !== 'Practice'} fixedPerformance={mode === 'Practice'} deletionLocked={selected?.kind === 'source' || mode === 'Practice' && selected?.kind === 'api' && problemUsage.counts.api === 1} maxProcessingSlotsPerNode={mode === 'Practice' ? challenge?.resourceBudget.maxProcessingSlotsPerNode : undefined} onRename={renameNode} onConfig={changeConfig} onDelete={() => selectedId && removeNode(selectedId)} rateRps={rateRps} durationSeconds={durationSeconds} readPercent={readPercent} seed={seed} profile={profile} onProfile={setProfile} arrivalModel={arrivalModel} onArrivalModel={setArrivalModel} scenario={scenario} onScenario={setScenario} keySpace={keySpace} onKeySpace={setKeySpace} workloadLocked={mode === 'Practice'} onWorkload={values => { if (values.rateRps !== undefined) setRateRps(values.rateRps); if (values.durationSeconds !== undefined) setDurationSeconds(values.durationSeconds); if (values.readPercent !== undefined) setReadPercent(values.readPercent); if (values.seed !== undefined) setSeed(values.seed) }} />
        <MetricsPanel status={displayStatus} live={simulation.live || displayStatus === 'stopped'} warning={simulation.warning} liveP95LatencyUs={simulation.liveP95LatencyUs} liveP99LatencyUs={simulation.liveP99LatencyUs} snapshots={displaySnapshots} result={fixedRunCurrent ? simulation.result : null} error={simulation.error} durationSeconds={durationSeconds} onPause={simulation.pause} onResume={simulation.resume} onCancel={simulation.cancel} onSaveRun={mode === 'Dev' && projectId ? () => void saveRun() : undefined} savingRun={runBusy} collapsed={collapsed.metrics} onToggle={() => togglePanel('metrics')} onResizeStart={event => beginMobileResize('metrics', event)} />
        {!collapsed.rail && <div role="separator" aria-label="Resize components" aria-orientation="vertical" className="panel-resizer rail-resizer" onPointerDown={event => beginResize('rail', event)} />}
        {!collapsed.inspector && <div role="separator" aria-label="Resize inspector" aria-orientation="vertical" className="panel-resizer inspector-resizer" onPointerDown={event => beginResize('inspector', event)} />}
        {!collapsed.metrics && <div role="separator" aria-label="Resize simulation metrics" aria-orientation="horizontal" className="panel-resizer metrics-resizer" onPointerDown={event => beginResize('metrics', event)} />}
      </main>
      </div></div> : <main className="portal-page"><div><h1>Page not found</h1><button type="button" className="secondary-button" onClick={() => openPage('/')}>Back home</button></div></main>}
      {notice && <div className="notice" role="status"><span>{notice}</span><button type="button" aria-label="Dismiss notice" onClick={() => setNotice(null)}>×</button></div>}
      {authOpen && <div className="dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setAuthOpen(false) }}><section className="dialog" role="dialog" aria-modal="true" aria-labelledby="auth-title">
        <button className="dialog-close" type="button" aria-label="Close sign-in" onClick={() => setAuthOpen(false)}>×</button>
        <h2 id="auth-title">{authMode === 'login' ? 'Sign in to playHLD' : 'Create your account'}</h2>
        <p>Save architecture designs to your private account.</p>
        <form onSubmit={event => void submitAuth(event)}>
          <label className="field"><span>Email</span><input type="email" autoComplete="email" required value={authEmail} onChange={event => setAuthEmail(event.target.value)} /></label>
          <label className="field"><span>Password</span><input type="password" autoComplete={authMode === 'login' ? 'current-password' : 'new-password'} minLength={authMode === 'register' ? 12 : undefined} required value={authPassword} onChange={event => setAuthPassword(event.target.value)} /></label>
          {authError && <p role="alert" className="error-message">{authError}</p>}
          <button className="primary-button dialog-submit" type="submit" disabled={authBusy}>{authBusy ? 'Working…' : authMode === 'login' ? 'Sign in' : 'Create account'}</button>
        </form>
        <button className="dialog-switch" type="button" onClick={() => { setAuthMode(authMode === 'login' ? 'register' : 'login');setAuthError(null) }}>{authMode === 'login' ? 'Create an account' : 'Already have an account? Sign in'}</button>
      </section></div>}
      {designsOpen && <div className="dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setDesignsOpen(false) }}><section className="dialog" role="dialog" aria-modal="true" aria-labelledby="designs-title">
        <button className="dialog-close" type="button" aria-label="Close designs" onClick={() => setDesignsOpen(false)}>×</button>
        <h2 id="designs-title">My designs</h2>
        <label className="field"><span>Current design name</span><input value={designName} maxLength={120} onChange={event => setDesignName(event.target.value)} /></label>
        <div className="saved-design-list">{savedDesigns.length ? savedDesigns.map(design => <button key={design.id} type="button" onClick={() => void loadDesign(design.id)}><strong>{design.name}</strong><span>Version {design.version} · {new Date(design.updatedAt).toLocaleString()}</span></button>) : <p>No server-saved designs yet. Save this draft to create one.</p>}</div>
      </section></div>}
      {projectsOpen && <ProjectDialog currentProjectId={projectId} projects={projects} workspaces={workspaces} versions={versions} runs={runs} name={newProjectName} busy={projectBusy} onName={setNewProjectName} onClose={() => setProjectsOpen(false)} onCreate={() => void createProject()} onLoad={id => void loadProject(id)} onSaveVersion={() => void saveVersion()} onLoadVersion={id => void loadVersion(id)} onOpenDesigns={() => { setProjectsOpen(false); void openDesigns() }} />}
    </div>
  </>
}

