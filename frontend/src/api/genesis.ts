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
  includeGitHubVersions = false,
  includeDeprecatedPatches = false
): Step1OptionsResponse | null {
  return peekSessionCache<Step1OptionsResponse>(
    `step1:${rancherVersion}:rc=${includeRC}:gh=${includeGitHubVersions}:dep=${includeDeprecatedPatches}`,
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
  /** Listed from rancher/rancher GitHub releases (community image lists). */
  communityAvailable?: boolean
  /** rancher-images.txt exists on prime.ribs.rancher.io. */
  primeAvailable?: boolean
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
  includeGitHubVersions = false,
  includeDeprecatedPatches = false
): Promise<Step1OptionsResponse> {
  const cacheKey = `step1:${rancherVersion}:rc=${includeRC}:gh=${includeGitHubVersions}:dep=${includeDeprecatedPatches}`
  return fetchCached(cacheKey, STEP1_OPTIONS_TTL_MS, async () => {
    const v = encodeURIComponent(rancherVersion)
    const rc = includeRC ? '&includeRC=true' : ''
    const gh = includeGitHubVersions ? '&includeGitHubVersions=true' : ''
    const dep = includeDeprecatedPatches ? '&includeDeprecatedPatches=true' : ''
    const r = await fetch(`${API_BASE}/step1-options?rancher=${v}${rc}${gh}${dep}`)
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
  // Multi-CNI: when cni is a comma-joined list, send the array as cnis (backend
  // prefers CNIs over CNI). Keep cni for backward compatibility.
  if ((!req.cnis || req.cnis.length === 0) && req.cni && req.cni.includes(',')) {
    payload.cnis = req.cni.split(',').map((c) => c.trim()).filter(Boolean)
  } else if (req.cnis && req.cnis.length > 0) {
    payload.cnis = req.cnis
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
  includeGitHubVersions: boolean,
  includeDeprecatedPatches: boolean
): Promise<Step1OptionsResponse> {
  if (rancherVersions.length === 0) {
    return { hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } }
  }
  if (rancherVersions.length === 1) {
    const v = rancherVersions[0]
    return v ? fetchStep1Options(v, includeRC, includeGitHubVersions, includeDeprecatedPatches) : Promise.resolve({ hasRKE1: false, capabilities: {}, details: { kdmUrl: '', imageListSource: '' } })
  }
  const results = await Promise.all(
    rancherVersions.map((v) => fetchStep1Options(v, includeRC, includeGitHubVersions, includeDeprecatedPatches))
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

export interface ProgressResponse {
  active: boolean
  percent: number
  phase: string
  detail?: string
  current?: number
  total?: number
}

export async function fetchProgress(): Promise<ProgressResponse & { available?: boolean }> {
  const r = await fetch(`${API_BASE}/progress`)
  if (!r.ok) return { active: false, percent: 0, phase: 'Idle', available: false }
  const data = (await r.json()) as ProgressResponse
  return { ...data, available: true }
}

export type AvailabilityResult = Record<string, { status: string; detail: string; sizeBytes?: number }>

export async function checkAvailability(images: string[], arch = 'amd64'): Promise<AvailabilityResult> {
  const r = await fetch(`${API_BASE}/check-availability`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images, arch }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { results: AvailabilityResult }
  return data.results
}

export type ImageSizeResult = Record<string, { sizeBytes?: number; arch?: string; status: string; detail?: string }>

/** Fetch compressed linux/<arch> sizes for the given images, independent of availability. */
export async function fetchImageSizes(images: string[], arch = 'amd64'): Promise<ImageSizeResult> {
  const r = await fetch(`${API_BASE}/image-sizes`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ images, arch }),
  })
  if (!r.ok) {
    const err = await r.json().catch(() => ({ error: r.statusText }))
    throw new Error((err as { error?: string }).error || r.statusText)
  }
  const data = await r.json() as { results: ImageSizeResult }
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

export interface SuseObservabilityVersionsResponse {
  agent: string[]
  server: string[]
}

/** Chart versions from charts.rancher.com prime/suse-observability index. */
export async function fetchSuseObservabilityVersions(includePre = false): Promise<SuseObservabilityVersionsResponse> {
  const key = `suse-observability-versions:pre=${includePre}`
  const cached = cacheGet<SuseObservabilityVersionsResponse>(key, STEP1_OPTIONS_TTL_MS)
  if (cached) return cached
  const existing = inflight.get(key)
  if (existing) return existing as Promise<SuseObservabilityVersionsResponse>
  const p = (async () => {
    const r = await fetch(`${API_BASE}/suse-observability-versions?includePre=${includePre}`)
    if (!r.ok) {
      const err = await r.json().catch(() => ({ error: r.statusText }))
      throw new Error((err as { error?: string }).error || r.statusText)
    }
    const data = (await r.json()) as SuseObservabilityVersionsResponse
    cacheSet(key, data)
    return data
  })().finally(() => inflight.delete(key))
  inflight.set(key, p)
  return p
}

export interface ScanStatusResponse {
  status: 'running' | 'completed' | 'failed'
  error?: string
  summary?: { critical: number; high: number; medium: number; low: number }
  phase?: string
  percent?: number
  current?: number
  total?: number
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
