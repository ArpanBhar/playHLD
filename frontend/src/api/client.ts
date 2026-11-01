import type { Architecture, TrafficSegment } from '../simulation/types'

export interface Account { id: string; email: string }
export interface SavedDesign { id: string; name: string; architecture: Architecture; version: number; updatedAt: string }
export interface DesignSummary { id: string; name: string; version: number; updatedAt: string }
export interface WorkspaceSummary { id: string; name: string; role: string }
export interface ProjectSummary { id: string; workspaceId: string; name: string; description: string; updatedAt: string; role: string }
export interface ProjectDetails { id: string; workspaceId: string; name: string; description: string; branch: { id: string; revision: number; architecture: Architecture } }
export interface VersionSummary { id: string; revision: number; graphHash: string; createdAt: string }
export interface RunSummary { id: string; versionId: string; engineVersion: string; configHash: string; provenance: string; createdAt: string; workload: { schemaVersion: number; seed: string; durationUs: number; arrivalRateRps: number; readFraction: number; segments?: TrafficSegment[]; arrivalModel?: 'fixed' | 'poisson' }; summary: { generated: number; succeeded: number; failed: number; unfinished: number; p95LatencyUs: number | null } }

function csrfToken(): string {
  return document.cookie.split('; ').find(item => item.startsWith('sf_csrf='))?.slice('sf_csrf='.length) || ''
}

export async function apiRequest<T>(path: string, method = 'GET', body?: unknown): Promise<T> {
  const headers: Record<string, string> = {}
  if (body !== undefined) headers['Content-Type'] = 'application/json'
  if (!['GET', 'HEAD'].includes(method) && !['/api/auth/login', '/api/auth/register'].includes(path)) headers['X-CSRF-Token'] = csrfToken()
  const response = await fetch(path, { method, headers, credentials: 'same-origin', body: body === undefined ? undefined : JSON.stringify(body) })
  if (!response.ok) {
    let message = `Request failed (${response.status})`
    try { const payload = await response.json(); message = payload.error?.message || message } catch { /* keep HTTP status */ }
    throw new Error(message)
  }
  return response.json() as Promise<T>
}

