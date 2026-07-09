import type {
  Step1OptionsResponse,
  GenerateRequest,
  GenerateResponse,
  ExportRequest,
} from '../types/genesis'

const API_BASE = '/api'

const CACHE_PREFIX = 'genesis-api:'
const RANCHER_VERSIONS_TTL_MS = 15 * 60 * 1000
const STEP1_OPTIONS_TTL_MS = 10 * 60 * 1000

type CacheEnvelope<T> = { at: number; data: T }

function cacheGet<T>(key: string, ttlMs: number): T | null {
  try {
    const raw = sessionStorage.getItem(CACHE_PREFIX + key)
    if (!raw) return null
    const parsed = JSON.parse(raw) as CacheEnvelope<T>
    if (Date.now() - parsed.at > ttlMs) return null
    return parsed.data
  } catch {
    return null
  }
}

function cacheSet<T>(key: string, data: T) {
  try {
    sessionStorage.setItem(CACHE_PREFIX + key, JSON.stringify({ at: Date.now(), data }))
  } catch {
    /* quota or private mode */
  }
}

const inflight = new Map<string, Promise<unknown>>()

/** Synchronous sessionStorage read for instant UI hydration on repeat visits. */
export function peekSessionCache<T>(key: string, ttlMs: number): T | null {
  return cacheGet<T>(key, ttlMs)
}

export function peekRancherVersionsCache(includeRC = false): RancherVersionInfo[] | null {
  return peekSessionCache<RancherVersionInfo[]>(`rancher-versions:rc=${includeRC}`, RANCHER_VERSIONS_TTL_MS)
}

export function peekStep1OptionsCache(
  rancherVersion: string,
  includeRC = false,
  includeGitHubVersions = false
): Step1OptionsResponse | null {
  return peekSessionCache<Step1OptionsResponse>(
    `step1:${rancherVersion}:rc=${includeRC}:gh=${includeGitHubVersions}`,
    STEP1_OPTIONS_TTL_MS
  )
}

async function fetchCached<T>(key: string, ttlMs: number, fetcher: () => Promise<T>): Promise<T> {
  const hit = cacheGet<T>(key, ttlMs)
  if (hit !== null) return hit
  const pending = inflight.get(key) as Promise<T> | undefined
  if (pending) return pending
  const p = fetcher().then((data) => {
    cacheSet(key, data)
    inflight.delete(key)
    return data
  }).catch((err) => {
    inflight.delete(key)
    throw err
  })
  inflight.set(key, p)
  return p
}

export interface RancherVersionInfo {
  version: string
  date: string
}

export async function fetchRancherVersions(includeRC = false): Promise<RancherVersionInfo[]> {
  const cacheKey = `rancher-versions:rc=${includeRC}`
  return fetchCached(cacheKey, RANCHER_VERSIONS_TTL_MS, async () => {
    const rc = includeRC ? '?includeRC=true' : ''
    const r = await fetch(`${API_BASE}/rancher-versions${rc}`)
    if (!r.ok) {
      const err = await r.json().catch(() => ({ error: r.statusText }))
      throw new Error((err as { error?: string }).error || r.statusText)
    }
    const data = await r.json() as { versions: RancherVersionInfo[] | string[] }
    if (!data.versions?.length) return []
    if (typeof data.versions[0] === 'string') {
      return (data.versions as string[]).map(v => ({ version: v, date: '' }))
    }
    return data.versions as RancherVersionInfo[]
  })
}

export async function fetchStep1Options(
  rancherVersion: string,
  includeRC = false,
  includeGitHubVersions = false
): Promise<Step1OptionsResponse> {
  const cacheKey = `step1:${rancherVersion}:rc=${includeRC}:gh=${includeGitHubVersions}`
  return fetchCached(cacheKey, STEP1_OPTIONS_TTL_MS, async () => {
    const v = encodeURIComponent(rancherVersion)
    const rc = includeRC ? '&includeRC=true' : ''
    const gh = includeGitHubVersions ? '&includeGitHubVersions=true' : ''
    const r = await fetch(`${API_BASE}/step1-options?rancher=${v}${rc}${gh}`)
    if (!r.ok) {
      const err = await r.json().catch(() => ({ error: r.statusText }))
      throw new Error((err as { error?: string }).error || r.statusText)
    }
    return r.json()
  })
}

export async function generate(req: GenerateRequest): Promise<GenerateResponse> {
  const distros = req.distros.filter((d) => d !== 'rke')
  const payload: Record<string, unknown> = {
    ...req,
    distros,
    k3sVersions: distros.includes('k3s') ? req.k3sVersions.join(',') : '',
    rke2Versions: distros.includes('rke2') ? req.rke2Versions.join(',') : '',
    rkeVersions: '',
  }
  if (req.rancherVersions?.length) {
    payload.rancherVersions = req.rancherVersions
    payload.rancherVersion = req.rancherVersions[0] || req.rancherVersion
  }
  const r = await fetch(`${API_BASE}/generate`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

/** Fetch step1 options for multiple Rancher versions and merge capabilities (union of K3s/RKE2/RKE versions). */
export async function fetchStep1OptionsMerged(
  rancherVersions: string[],
  includeRC: boolean,
  includeGitHubVersions: boolean
): Promise<Step1OptionsResponse> {
  if (rancherVersions.length === 0) {
    return { hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } }
  }
  if (rancherVersions.length === 1) {
    const v = rancherVersions[0]
    return v ? fetchStep1Options(v, includeRC, includeGitHubVersions) : Promise.resolve({ hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } })
  }
  const results = await Promise.all(
    rancherVersions.map((v) => fetchStep1Options(v, includeRC, includeGitHubVersions))
  )
  const first = results[0]
  const merged: Step1OptionsResponse = {
    hasRKE1: results.some((r) => r.hasRKE1),
    capabilities: {},
    details: first ? first.details : { kdmUrl: '', imageListSource: '' },
  }
  const distros = ['k3s', 'rke2'] as const
  for (const d of distros) {
    const allVersions = new Set<string>()
    const sources: Record<string, string> = {}
    for (const r of results) {
      const cap = r.capabilities?.[d]
      if (!cap) continue
      for (const v of cap.versions) {
        allVersions.add(v)
        sources[v] = cap.sources?.[v] ?? 'kdm'
      }
    }
    if (allVersions.size) {
      merged.capabilities[d] = {
        versions: [...allVersions].sort(),
        sources,
      }
    }
  }
  return merged
}

export async function fetchLogs(): Promise<string[]> {
  const r = await fetch(`${API_BASE}/logs`)
  if (!r.ok) return []
  const data = (await r.json()) as { lines?: string[] }
  return data.lines ?? []
}

export type AvailabilityResult = Record<string, { status: string; detail: string; sizeBytes?: number }>

export async function checkAvailability(images: string[]): Promise<AvailabilityResult> {
  const r = await fetch(`${API_BASE}/check-availability`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { results: AvailabilityResult }
  return data.results
}

export interface ReleaseInfo {
  tag: string
  name: string
  publishedAt: string
  url: string
  prerelease: boolean
  charts: { name: string; version: string }[]
  changelog: string[]
  body?: string
}

export async function fetchReleaseNotes(repo: string, tag: string): Promise<ReleaseInfo> {
  const r = await fetch(`${API_BASE}/release-notes?repo=${encodeURIComponent(repo)}&tag=${encodeURIComponent(tag)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function exportImageList(req: ExportRequest): Promise<Blob> {
  const r = await fetch(`${API_BASE}/export`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.blob()
}

export interface ScanStatusResponse {
  status: 'running' | 'completed' | 'failed'
  error?: string
  summary?: { critical: number; high: number; medium: number; low: number }
}

export async function startScan(images: string[]): Promise<{ scanJobId: string }> {
  const r = await fetch(`${API_BASE}/scan`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function getScanStatus(scanJobId: string): Promise<ScanStatusResponse> {
  const r = await fetch(`${API_BASE}/scan/status/${encodeURIComponent(scanJobId)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.json()
}

export async function downloadScanReport(scanJobId: string): Promise<Blob> {
  const r = await fetch(`${API_BASE}/scan/report/${encodeURIComponent(scanJobId)}`)
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  return r.blob()
}
