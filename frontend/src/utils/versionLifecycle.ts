/** Kubernetes minor lifecycle (active support ≈ EOM, maintenance end = EOL). Source: kubernetes.io/releases */
const K8S_MINOR_LIFECYCLE: Record<string, { eom: string; eol: string }> = {
  '1.36': { eom: '2027-04-28', eol: '2027-06-28' },
  '1.35': { eom: '2026-12-28', eol: '2027-02-28' },
  '1.34': { eom: '2026-08-27', eol: '2026-10-27' },
  '1.33': { eom: '2026-04-28', eol: '2026-06-28' },
  '1.32': { eom: '2025-12-28', eol: '2026-02-28' },
  '1.31': { eom: '2025-09-11', eol: '2025-11-11' },
  '1.30': { eom: '2025-05-15', eol: '2025-07-15' },
  '1.29': { eom: '2024-12-28', eol: '2025-02-28' },
  '1.28': { eom: '2024-08-28', eol: '2024-10-22' },
  '1.27': { eom: '2024-04-28', eol: '2024-07-16' },
  '1.26': { eom: '2023-12-28', eol: '2024-02-28' },
  '1.25': { eom: '2023-08-28', eol: '2023-10-28' },
}

/** SUSE Rancher Manager minor lifecycle (general support ≈ EOM, end of maintenance = EOL). */
const RANCHER_MINOR_LIFECYCLE: Record<string, { eom: string; eol: string }> = {
  '2.14': { eom: '2027-06-30', eol: '2027-12-31' },
  '2.13': { eom: '2027-03-31', eol: '2027-09-30' },
  '2.12': { eom: '2026-12-31', eol: '2027-06-30' },
  '2.11': { eom: '2026-06-30', eol: '2026-12-31' },
  '2.10': { eom: '2025-06-30', eol: '2025-12-31' },
  '2.9': { eom: '2025-03-31', eol: '2025-09-30' },
  '2.8': { eom: '2024-12-31', eol: '2025-06-30' },
  '2.7': { eom: '2024-06-30', eol: '2024-12-31' },
}

export type VersionLifecycleStatus = 'supported' | 'maintenance' | 'eol' | 'unknown'

export interface VersionAnnotation {
  version: string
  releaseDate?: string
  kubernetesMinor?: string
  status: VersionLifecycleStatus
  eom?: string
  eol?: string
  isLatestPatch: boolean
  isCurrentMinor: boolean
}

export function extractKubernetesCore(version: string): string | null {
  const m = version.match(/^v?(\d+\.\d+\.\d+)/i)
  return m?.[1] ?? null
}

export function kubernetesMinorKey(core: string): string {
  const [major, minor] = core.split('.')
  return `${major}.${minor}`
}

export function extractRancherMinor(version: string): string | null {
  const m = version.match(/^v?(\d+\.\d+)/i)
  return m?.[1] ?? null
}

function parseIsoDate(iso: string): Date {
  const parts = iso.split('-').map(Number)
  const y = parts[0] ?? 0
  const m = parts[1] ?? 1
  const d = parts[2] ?? 1
  return new Date(y, m - 1, d)
}

function compareCoreSemver(a: string, b: string): number {
  const pa = a.split('.').map(Number)
  const pb = b.split('.').map(Number)
  for (let i = 0; i < 3; i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}

function compareVersionTags(a: string, b: string): number {
  const ca = extractKubernetesCore(a)
  const cb = extractKubernetesCore(b)
  if (ca && cb) return compareCoreSemver(ca, cb)
  const ra = a.replace(/^v/i, '').split('+')[0] ?? a
  const rb = b.replace(/^v/i, '').split('+')[0] ?? b
  const pa = ra.split('.').map(Number)
  const pb = rb.split('.').map(Number)
  for (let i = 0; i < Math.max(pa.length, pb.length); i++) {
    const diff = (pa[i] ?? 0) - (pb[i] ?? 0)
    if (diff !== 0) return diff
  }
  return 0
}

function k8sLifecycleStatus(minor: string, now = new Date()): {
  status: VersionLifecycleStatus
  eom?: string
  eol?: string
} {
  const row = K8S_MINOR_LIFECYCLE[minor]
  if (!row) return { status: 'unknown' }
  const eom = parseIsoDate(row.eom)
  const eol = parseIsoDate(row.eol)
  if (now > eol) return { status: 'eol', eom: row.eom, eol: row.eol }
  if (now > eom) return { status: 'maintenance', eom: row.eom, eol: row.eol }
  return { status: 'supported', eom: row.eom, eol: row.eol }
}

function rancherLifecycleStatus(minor: string, now = new Date()): {
  status: VersionLifecycleStatus
  eom?: string
  eol?: string
} {
  const row = RANCHER_MINOR_LIFECYCLE[minor]
  if (!row) return { status: 'unknown' }
  const eom = parseIsoDate(row.eom)
  const eol = parseIsoDate(row.eol)
  if (now > eol) return { status: 'eol', eom: row.eom, eol: row.eol }
  if (now > eom) return { status: 'maintenance', eom: row.eom, eol: row.eol }
  return { status: 'supported', eom: row.eom, eol: row.eol }
}

function latestPatchByMinor(versions: string[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const v of versions) {
    const core = extractKubernetesCore(v)
    if (!core) continue
    const minor = kubernetesMinorKey(core)
    const prev = map.get(minor)
    if (!prev || compareVersionTags(v, prev) > 0) map.set(minor, v)
  }
  return map
}

function currentMinorKey(versions: string[]): string | null {
  let best: string | null = null
  for (const v of versions) {
    const core = extractKubernetesCore(v)
    if (!core) continue
    const minor = kubernetesMinorKey(core)
    if (!best || compareCoreSemver(core, best) > 0) best = minor
  }
  return best
}

function rancherLatestPatchByMinor(versions: string[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const v of versions) {
    const minor = extractRancherMinor(v)
    if (!minor) continue
    const prev = map.get(minor)
    if (!prev || compareVersionTags(v, prev) > 0) map.set(minor, v)
  }
  return map
}

function rancherCurrentMinor(versions: string[]): string | null {
  let best: string | null = null
  for (const v of versions) {
    const minor = extractRancherMinor(v)
    if (!minor) continue
    if (!best || compareCoreSemver(`${minor}.0`, `${best}.0`) > 0) best = minor
  }
  return best
}

/** Annotate K3s/RKE2 tags (kubernetes-based lifecycle). */
export function annotateDistroVersions(versions: string[]): VersionAnnotation[] {
  const latestByMinor = latestPatchByMinor(versions)
  const currentMinor = currentMinorKey(versions)
  return versions.map((version) => {
    const core = extractKubernetesCore(version)
    const kubernetesMinor = core ? kubernetesMinorKey(core) : undefined
    const lifecycle = kubernetesMinor ? k8sLifecycleStatus(kubernetesMinor) : { status: 'unknown' as const }
    const isLatestPatch = kubernetesMinor ? latestByMinor.get(kubernetesMinor) === version : false
    const isCurrentMinor = !!(kubernetesMinor && currentMinor === kubernetesMinor && isLatestPatch)
    return {
      version,
      kubernetesMinor,
      status: lifecycle.status,
      eom: lifecycle.eom,
      eol: lifecycle.eol,
      isLatestPatch,
      isCurrentMinor,
    }
  })
}

/** Annotate Rancher Manager releases (latest patch / current minor; SUSE lifecycle dates). */
export function annotateRancherVersions(
  releases: { version: string; date?: string }[]
): VersionAnnotation[] {
  const versions = releases.map((r) => r.version)
  const latestByMinor = rancherLatestPatchByMinor(versions)
  const currentMinor = rancherCurrentMinor(versions)
  return releases.map(({ version, date }) => {
    const minor = extractRancherMinor(version)
    const isLatestPatch = minor ? latestByMinor.get(minor) === version : false
    const isCurrentMinor = !!(minor && currentMinor === minor && isLatestPatch)
    const lifecycle = minor ? rancherLifecycleStatus(minor) : { status: 'unknown' as const }
    return {
      version,
      releaseDate: date,
      status: lifecycle.status,
      eom: lifecycle.eom,
      eol: lifecycle.eol,
      isLatestPatch,
      isCurrentMinor,
    }
  })
}

/** Pick the newest KDM-supported patch (current minor line, official catalog). */
export function pickLatestOfficialDistroVersion(
  versions: string[],
  sources?: Record<string, string>
): string | null {
  if (!versions.length) return null
  const kdmOnly = sources
    ? versions.filter((v) => {
        const src = sources[v]?.toLowerCase()
        return src === 'kdm' || src === 'both'
      })
    : versions
  const pool = kdmOnly.length
    ? kdmOnly
    : versions.filter((v) => {
        const src = sources?.[v]?.toLowerCase()
        return src !== 'github' && !/[-+]rc/i.test(v)
      })
  const candidates = pool.length ? pool : versions
  const annotated = annotateDistroVersions(candidates)
  const current = annotated.find((a) => a.isCurrentMinor)
  if (current) return current.version
  const sorted = [...annotated].sort((a, b) => compareVersionTags(b.version, a.version))
  return sorted[0]?.version ?? null
}

export function lifecycleStatusLabel(status: VersionLifecycleStatus): string {
  switch (status) {
    case 'supported':
      return 'Supported'
    case 'maintenance':
      return 'EOM'
    case 'eol':
      return 'EOL'
    default:
      return ''
  }
}

export function lifecycleStatusTitle(ann: VersionAnnotation): string {
  const parts: string[] = []
  if (ann.isCurrentMinor) parts.push('Newest minor line in this list')
  else if (ann.isLatestPatch) parts.push('Latest patch for this minor release')
  if (ann.kubernetesMinor && ann.eom && ann.eol) {
    parts.push(`Active support ends ${ann.eom} (EOM)`)
    parts.push(`End of life ${ann.eol} (EOL)`)
  } else if (!ann.kubernetesMinor && ann.eom && ann.eol) {
    parts.push(`General support ends ${ann.eom} (EOM)`)
    parts.push(`End of maintenance ${ann.eol} (EOL)`)
  }
  if (ann.releaseDate) parts.push(`Released ${ann.releaseDate}`)
  return parts.join(' · ')
}
