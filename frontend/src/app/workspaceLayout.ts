export interface WorkspaceLayout {
  panelSizes: { rail: number; inspector: number; metrics: number }
  descriptionWidth: number
  mobileHeights: { description: number; rail: number; editor: number; inspector: number; metrics: number }
  collapsed: { rail: boolean; inspector: boolean; metrics: boolean; editor: boolean }
  descriptionExpanded: boolean
}

const storageKey = 'systemforge:workspace-layout:v1'

export const defaultWorkspaceLayout: WorkspaceLayout = {
  panelSizes: { rail: 160, inspector: 220, metrics: 244 },
  descriptionWidth: 42,
  mobileHeights: { description: 225, rail: 70, editor: 400, inspector: 320, metrics: 310 },
  collapsed: { rail: false, inspector: false, metrics: false, editor: false },
  descriptionExpanded: false,
}

function dimension(value: unknown, fallback: number, minimum: number, maximum: number): number {
  return typeof value === 'number' && Number.isFinite(value)
    ? Math.max(minimum, Math.min(maximum, value))
    : fallback
}

function flag(value: unknown, fallback: boolean): boolean {
  return typeof value === 'boolean' ? value : fallback
}

export function loadWorkspaceLayout(): WorkspaceLayout {
  try {
    const saved = JSON.parse(localStorage.getItem(storageKey) ?? 'null') as Partial<WorkspaceLayout> | null
    const defaults = defaultWorkspaceLayout
    return {
      panelSizes: {
        rail: dimension(saved?.panelSizes?.rail, defaults.panelSizes.rail, 132, 900),
        inspector: dimension(saved?.panelSizes?.inspector, defaults.panelSizes.inspector, 190, 900),
        metrics: dimension(saved?.panelSizes?.metrics, defaults.panelSizes.metrics, 160, 800),
      },
      descriptionWidth: dimension(saved?.descriptionWidth, defaults.descriptionWidth, 20, 70),
      mobileHeights: {
        description: dimension(saved?.mobileHeights?.description, defaults.mobileHeights.description, 130, 700),
        rail: dimension(saved?.mobileHeights?.rail, defaults.mobileHeights.rail, 50, 240),
        editor: dimension(saved?.mobileHeights?.editor, defaults.mobileHeights.editor, 250, 900),
        inspector: dimension(saved?.mobileHeights?.inspector, defaults.mobileHeights.inspector, 110, 700),
        metrics: dimension(saved?.mobileHeights?.metrics, defaults.mobileHeights.metrics, 110, 700),
      },
      collapsed: {
        rail: flag(saved?.collapsed?.rail, defaults.collapsed.rail),
        inspector: flag(saved?.collapsed?.inspector, defaults.collapsed.inspector),
        metrics: flag(saved?.collapsed?.metrics, defaults.collapsed.metrics),
        editor: flag(saved?.collapsed?.editor, defaults.collapsed.editor),
      },
      descriptionExpanded: flag(saved?.descriptionExpanded, defaults.descriptionExpanded),
    }
  } catch {
    return defaultWorkspaceLayout
  }
}

export function saveWorkspaceLayout(layout: WorkspaceLayout): void {
  try { localStorage.setItem(storageKey, JSON.stringify(layout)) }
  catch { /* The editor still works when browser storage is unavailable. */ }
}
