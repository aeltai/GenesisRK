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
  /** Older patch superseded within the same Kubernetes minor (KDM only). */
  isDeprecatedPatch: boolean
  /** Image lists on prime.ribs.rancher.io (Rancher Prime Registry). */
  primeAvailable?: boolean
  /** Image lists on GitHub releases (Community). */
  communityAvailable?: boolean
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

/** Rancher Manager pre-release tag (RC/alpha/beta). */
export function isRancherPreRelease(version: string): boolean {
  return /-(rc|alpha|beta)/i.test(version)
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
    const isDeprecatedPatch = !!(kubernetesMinor && !isLatestPatch)
    return {
      version,
      kubernetesMinor,
      status: lifecycle.status,
      eom: lifecycle.eom,
      eol: lifecycle.eol,
      isLatestPatch,
      isCurrentMinor,
      isDeprecatedPatch,
    }
  })
}

/** Annotate Rancher Manager releases (latest patch / current minor; SUSE lifecycle dates). */
export function annotateRancherVersions(
  releases: {
    version: string
    date?: string
    primeAvailable?: boolean
    communityAvailable?: boolean
  }[]
): VersionAnnotation[] {
  const stableReleases = releases.filter((r) => !isRancherPreRelease(r.version))
  const badgePool = stableReleases.length ? stableReleases : releases
  const badgeVersions = badgePool.map((r) => r.version)
  const latestByMinor = rancherLatestPatchByMinor(badgeVersions)
  const currentMinor = rancherCurrentMinor(badgeVersions)
  return releases.map(({ version, date, primeAvailable, communityAvailable }) => {
    const minor = extractRancherMinor(version)
    const pre = isRancherPreRelease(version)
    const isLatestPatch = !pre && minor ? latestByMinor.get(minor) === version : false
    const isCurrentMinor = !pre && !!(minor && currentMinor === minor && isLatestPatch)
    const isDeprecatedPatch = !pre && !!(minor && !isLatestPatch)
    const lifecycle = minor ? rancherLifecycleStatus(minor) : { status: 'unknown' as const }
    return {
      version,
      releaseDate: date,
      status: lifecycle.status,
      eom: lifecycle.eom,
      eol: lifecycle.eol,
      isLatestPatch,
      isCurrentMinor,
      isDeprecatedPatch,
      primeAvailable,
      communityAvailable: communityAvailable ?? true,
    }
  })
}

/** Newest stable Rancher release (current minor’s latest patch when identifiable). */
export function pickLatestStableRancherVersion(versions: string[]): string | null {
  const stable = versions.filter((v) => !isRancherPreRelease(v))
  if (!stable.length) return versions[0] ?? null
  const annotated = annotateRancherVersions(stable.map((version) => ({ version })))
  const current = annotated.find((a) => a.isCurrentMinor)
  if (current) return current.version
  return stable[0] ?? null
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
  else if (ann.isDeprecatedPatch) parts.push('Superseded patch — a newer patch exists for this minor')
  if (ann.kubernetesMinor && ann.eom && ann.eol) {
    parts.push(`Active support ends ${ann.eom} (EOM)`)
    parts.push(`End of life ${ann.eol} (EOL)`)
  } else if (!ann.kubernetesMinor && ann.eom && ann.eol) {
    parts.push(`General support ends ${ann.eom} (EOM)`)
    parts.push(`End of maintenance ${ann.eol} (EOL)`)
  }
  if (ann.releaseDate) parts.push(`Released ${ann.releaseDate}`)
  if (ann.primeAvailable && ann.communityAvailable !== false) {
    parts.push('Image lists: Community (GitHub) + Rancher Prime (prime.ribs.rancher.io)')
  } else if (ann.primeAvailable) {
    parts.push('Image lists: Rancher Prime (prime.ribs.rancher.io) only')
  } else if (ann.communityAvailable !== false) {
    parts.push('Image lists: Community (GitHub) only — no Prime mirror for this patch')
  }
  return parts.join(' · ')
}

/**
 * Best-effort lifecycle status for packaged Rancher stack components (charts/images).
 * Sourced from SUSE Product Lifecycle, endoflife.date, and upstream project repos.
 * Only entries with a known maintenance signal are listed; absence means unknown
 * (no badge is rendered). Dates are omitted where upstream does not publish fixed
 * EOM/EOL dates — only the status is asserted there.
 */
export interface ComponentLifecycleAnnotation {
  component: string
  status: VersionLifecycleStatus
  eom?: string
  eol?: string
  source: string
  note?: string
}

// Keys are matched case-insensitively against chart/image names.
const COMPONENT_LIFECYCLE: Record<string, ComponentLifecycleAnnotation> = {
  coredns: { component: 'CoreDNS', status: 'supported', source: 'coredns/coredns (CNCF graduated)', note: 'Actively maintained; bundled as rke2-coredns / k3s CoreDNS.' },
  fleet: { component: 'Fleet', status: 'supported', source: 'rancher/fleet (SUSE Rancher)', note: 'Actively maintained GitOps controller.' },
  'fleet-agent': { component: 'Fleet Agent', status: 'supported', source: 'rancher/fleet (SUSE Rancher)' },
  'fleet-controller': { component: 'Fleet Controller', status: 'supported', source: 'rancher/fleet (SUSE Rancher)' },
  'ingress-nginx': { component: 'Ingress NGINX', status: 'supported', source: 'kubernetes/ingress-nginx', note: 'Current Kubernetes ingress controller.' },
  // Legacy nginx-ingress chart is deprecated in favor of ingress-nginx.
  'nginx-ingress': { component: 'nginx-ingress (legacy)', status: 'maintenance', source: 'kubernetes/ingress-nginx', note: 'Legacy chart name; prefer ingress-nginx.' },
  'metrics-server': { component: 'Metrics Server', status: 'supported', source: 'kubernetes-sigs/metrics-server' },
  'snapshot-controller': { component: 'Snapshot Controller', status: 'supported', source: 'kubernetes-csi/external-snapshotter' },
  'rancher-snapshot-controller': { component: 'Snapshot Controller', status: 'supported', source: 'kubernetes-csi/external-snapshotter' },
  canal: { component: 'Canal', status: 'supported', source: 'projectcalico/calico', note: 'Calico + Flannel; RKE2 default CNI.' },
  'hardened-canal': { component: 'Canal', status: 'supported', source: 'projectcalico/calico (SUSE hardened)' },
  calico: { component: 'Calico', status: 'supported', source: 'projectcalico/calico / Tigera' },
  'hardened-calico': { component: 'Calico', status: 'supported', source: 'projectcalico/calico (SUSE hardened)' },
  cilium: { component: 'Cilium', status: 'supported', source: 'cilium/cilium (CNCF graduated)' },
  flannel: { component: 'Flannel', status: 'supported', source: 'flannel-io/flannel', note: 'Simple overlay; K3s default CNI.' },
  'hardened-flannel': { component: 'Flannel', status: 'supported', source: 'flannel-io/flannel (SUSE hardened)' },
  'local-path-provisioner': { component: 'Local Path Provisioner', status: 'supported', source: 'rancher/local-path-provisioner' },
  longhorn: { component: 'Longhorn', status: 'supported', source: 'longhorn/longhorn (SUSE Rancher)' },
  neuvector: { component: 'NeuVector', status: 'supported', source: 'SUSE NeuVector / neuvector/neuvector' },
  'rancher-gatekeeper': { component: 'Gatekeeper', status: 'supported', source: 'open-policy-agent/gatekeeper' },
  gatekeeper: { component: 'Gatekeeper', status: 'supported', source: 'open-policy-agent/gatekeeper' },
  monitoring: { component: 'Monitoring', status: 'supported', source: 'rancher/charts (Prometheus stack)', note: 'Prometheus, Grafana, Alertmanager — SUSE Rancher marketplace chart.' },
  logging: { component: 'Logging', status: 'supported', source: 'rancher/charts (Fluent Bit / Fluentd)' },
  'backup-restore': { component: 'Backup & Restore', status: 'supported', source: 'rancher/charts (Velero)' },
  prometheus: { component: 'Prometheus', status: 'supported', source: 'prometheus/prometheus (CNCF graduated)' },
  grafana: { component: 'Grafana', status: 'supported', source: 'grafana/grafana' },
  alertmanager: { component: 'Alertmanager', status: 'supported', source: 'prometheus/alertmanager' },
  'kube-state-metrics': { component: 'kube-state-metrics', status: 'supported', source: 'kubernetes/kube-state-metrics' },
  'node-exporter': { component: 'node-exporter', status: 'supported', source: 'prometheus/node_exporter' },
  thanos: { component: 'Thanos', status: 'supported', source: 'thanos-io/thanos' },
  pushprox: { component: 'PushProx', status: 'supported', source: 'rancher/charts' },
  'prometheus-operator': { component: 'Prometheus Operator', status: 'supported', source: 'prometheus-operator/prometheus-operator' },
  'config-reloader': { component: 'config-reloader', status: 'supported', source: 'prometheus-operator/prometheus-operator' },
  'rancher-monitoring': { component: 'rancher-monitoring', status: 'supported', source: 'rancher/charts (Prometheus stack)' },
  'rancher-monitoring-crd': { component: 'rancher-monitoring CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-project-monitoring': { component: 'rancher-project-monitoring', status: 'supported', source: 'rancher/charts' },
  'prometheus-federator': { component: 'Prometheus Federator', status: 'supported', source: 'rancher/charts' },
  'rancher-alerting-drivers': { component: 'Alerting Drivers', status: 'supported', source: 'rancher/charts' },
  'suse-observability-agent': { component: 'SUSE Observability Agent', status: 'supported', source: 'SUSE Observability' },
  'rancher-logging': { component: 'rancher-logging', status: 'supported', source: 'rancher/charts (Fluent Bit / Fluentd)' },
  'rancher-backup': { component: 'rancher-backup', status: 'supported', source: 'rancher/charts' },
  fluentbit: { component: 'Fluent Bit', status: 'supported', source: 'fluent/fluent-bit' },
  'fluent-bit': { component: 'Fluent Bit', status: 'supported', source: 'fluent/fluent-bit' },
  fluentd: { component: 'Fluentd', status: 'supported', source: 'fluent/fluentd' },
  velero: { component: 'Velero', status: 'supported', source: 'vmware-tanzu/velero' },
  // Add-on categories (subgroups under AddOns)
  storage: { component: 'Storage', status: 'supported', source: 'rancher/charts (Longhorn, Harvester, CSI)', note: 'Persistent storage operators and CSI drivers.' },
  security: { component: 'Security', status: 'supported', source: 'rancher/charts (NeuVector, Gatekeeper)', note: 'Runtime security and policy enforcement.' },
  cis: { component: 'CIS Benchmark', status: 'supported', source: 'rancher/charts (compliance scanning)', note: 'CIS benchmark and compliance reporting.' },
  provisioning: { component: 'Provisioning', status: 'supported', source: 'rancher/charts (EKS/GKE/AKS/vSphere operators)', note: 'Cloud provider cluster provisioning operators.' },
  networking: { component: 'Networking', status: 'supported', source: 'rancher/charts (Istio, SR-IOV)', note: 'Service mesh and advanced networking add-ons.' },
  'cluster-api': { component: 'Cluster API', status: 'supported', source: 'rancher/charts (CAPI providers)', note: 'Cluster API providers and Rancher Turtles.' },
  'os-management': { component: 'OS Management', status: 'supported', source: 'rancher/charts (Elemental)', note: 'Edge and OS lifecycle management.' },
  support: { component: 'Support & Diagnostics', status: 'supported', source: 'rancher/charts', note: 'Supportability and diagnostic tooling.' },
  other: { component: 'Marketplace Chart', status: 'supported', source: 'rancher/charts', note: 'Additional Rancher marketplace chart.' },
  core: { component: 'Rancher Core', status: 'supported', source: 'rancher/charts (auto-deployed)', note: 'System chart auto-deployed by Rancher.' },
  system: { component: 'Rancher System', status: 'supported', source: 'rancher/charts (auto-deployed)', note: 'Charts auto-deployed by Rancher.' },
  // Partner Charts & UI Plugins (opt-in repos)
  partnercharts: { component: 'Partner Charts', status: 'supported', source: 'rancher/partner-charts', note: 'Third-party charts validated for the Rancher marketplace.' },
  sourcepartnercharts: { component: 'Partner Charts', status: 'supported', source: 'rancher/partner-charts', note: 'Third-party charts validated for the Rancher marketplace.' },
  uiplugins: { component: 'UI Plugins', status: 'supported', source: 'rancher/ui-plugin-charts', note: 'Rancher dashboard UI extension charts.' },
  sourceuiplugincharts: { component: 'UI Plugins', status: 'supported', source: 'rancher/ui-plugin-charts', note: 'Rancher dashboard UI extension charts.' },
  addons: { component: 'AddOns', status: 'supported', source: 'rancher/charts', note: 'Optional Rancher marketplace charts.' },
  // Named Rancher charts (aligned with pkg/rancher/listgenerator/components.go)
  'rancher-webhook': { component: 'Rancher Webhook', status: 'supported', source: 'rancher/charts' },
  'rancher-provisioning-capi': { component: 'Rancher Provisioning CAPI', status: 'supported', source: 'rancher/charts' },
  'rancher-turtles': { component: 'Rancher Turtles', status: 'supported', source: 'rancher/turtles' },
  'cert-manager': { component: 'cert-manager', status: 'supported', source: 'quay.io/jetstack (prerequisite for Rancher Helm)', note: 'Install before the Rancher Helm chart.' },
  'system-upgrade-controller': { component: 'System Upgrade Controller', status: 'supported', source: 'rancher/system-upgrade-controller' },
  'remotedialer-proxy': { component: 'Remotedialer Proxy', status: 'supported', source: 'rancher/remotedialer' },
  'rancher-logging-crd': { component: 'rancher-logging CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-backup-crd': { component: 'rancher-backup CRDs', status: 'supported', source: 'rancher/charts' },
  'longhorn-crd': { component: 'Longhorn CRDs', status: 'supported', source: 'longhorn/longhorn' },
  'harvester-cloud-provider': { component: 'Harvester Cloud Provider', status: 'supported', source: 'harvester/harvester' },
  'harvester-csi-driver': { component: 'Harvester CSI Driver', status: 'supported', source: 'harvester/harvester' },
  'rancher-vsphere-csi': { component: 'vSphere CSI', status: 'supported', source: 'rancher/charts' },
  'neuvector-crd': { component: 'NeuVector CRDs', status: 'supported', source: 'neuvector/neuvector' },
  'neuvector-monitor': { component: 'NeuVector Monitor', status: 'supported', source: 'neuvector/neuvector' },
  'neuvector-controller': { component: 'NeuVector Controller', status: 'supported', source: 'neuvector/neuvector' },
  'neuvector-enforcer': { component: 'NeuVector Enforcer', status: 'supported', source: 'neuvector/neuvector' },
  'neuvector-manager': { component: 'NeuVector Manager', status: 'supported', source: 'neuvector/neuvector' },
  'rancher-gatekeeper-crd': { component: 'Gatekeeper CRDs', status: 'supported', source: 'open-policy-agent/gatekeeper' },
  'scc-operator': { component: 'SCC Operator', status: 'supported', source: 'rancher/charts' },
  'rancher-cis-benchmark': { component: 'CIS Benchmark', status: 'supported', source: 'rancher/charts' },
  'rancher-cis-benchmark-crd': { component: 'CIS Benchmark CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-compliance': { component: 'Rancher Compliance', status: 'supported', source: 'rancher/charts' },
  'rancher-compliance-crd': { component: 'Rancher Compliance CRDs', status: 'supported', source: 'rancher/charts' },
  'compliance-operator': { component: 'Compliance Operator', status: 'supported', source: 'rancher/charts' },
  'aks-operator': { component: 'AKS Operator', status: 'supported', source: 'rancher/aks-operator' },
  'eks-operator': { component: 'EKS Operator', status: 'supported', source: 'rancher/eks-operator' },
  'gke-operator': { component: 'GKE Operator', status: 'supported', source: 'rancher/gke-operator' },
  'ali-operator': { component: 'Aliyun Operator', status: 'supported', source: 'rancher/ali-operator' },
  'rancher-aks-operator': { component: 'Rancher AKS Operator', status: 'supported', source: 'rancher/charts' },
  'rancher-aks-operator-crd': { component: 'Rancher AKS Operator CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-eks-operator': { component: 'Rancher EKS Operator', status: 'supported', source: 'rancher/charts' },
  'rancher-eks-operator-crd': { component: 'Rancher EKS Operator CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-gke-operator': { component: 'Rancher GKE Operator', status: 'supported', source: 'rancher/charts' },
  'rancher-gke-operator-crd': { component: 'Rancher GKE Operator CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-ali-operator': { component: 'Rancher Aliyun Operator', status: 'supported', source: 'rancher/charts' },
  'rancher-vsphere-cpi': { component: 'vSphere CPI', status: 'supported', source: 'rancher/charts' },
  'rancher-cluster-api': { component: 'Rancher Cluster API', status: 'supported', source: 'rancher/charts' },
  'rancher-cluster-api-eks': { component: 'Rancher Cluster API EKS', status: 'supported', source: 'rancher/charts' },
  'cluster-api-controller': { component: 'Cluster API Controller', status: 'supported', source: 'kubernetes-sigs/cluster-api' },
  'cluster-api-aws-controller': { component: 'Cluster API AWS', status: 'supported', source: 'kubernetes-sigs/cluster-api-provider-aws' },
  'cluster-api-azure-controller': { component: 'Cluster API Azure', status: 'supported', source: 'kubernetes-sigs/cluster-api-provider-azure' },
  'cluster-api-gcp-controller': { component: 'Cluster API GCP', status: 'supported', source: 'kubernetes-sigs/cluster-api-provider-gcp' },
  'cluster-api-vsphere-controller': { component: 'Cluster API vSphere', status: 'supported', source: 'kubernetes-sigs/cluster-api-provider-vsphere' },
  'rancher-istio': { component: 'Rancher Istio', status: 'supported', source: 'rancher/charts (Istio service mesh)' },
  istio: { component: 'Istio', status: 'supported', source: 'istio/istio (CNCF)' },
  sriov: { component: 'SR-IOV', status: 'supported', source: 'rancher/charts' },
  'sriov-crd': { component: 'SR-IOV CRDs', status: 'supported', source: 'rancher/charts' },
  elemental: { component: 'Elemental', status: 'supported', source: 'rancher/elemental-operator' },
  'elemental-crd': { component: 'Elemental CRDs', status: 'supported', source: 'rancher/elemental-operator' },
  'elemental-operator': { component: 'Elemental Operator', status: 'supported', source: 'rancher/elemental-operator' },
  'rancher-supportability-review': { component: 'Supportability Review', status: 'supported', source: 'rancher/charts' },
  'rancher-supportability-review-crd': { component: 'Supportability Review CRDs', status: 'supported', source: 'rancher/charts' },
  'rancher-pushprox': { component: 'PushProx', status: 'supported', source: 'rancher/charts' },
  'fleet-crd': { component: 'Fleet CRDs', status: 'supported', source: 'rancher/fleet' },
  opa: { component: 'Open Policy Agent', status: 'supported', source: 'open-policy-agent/opa' },
  harvester: { component: 'Harvester', status: 'supported', source: 'harvester/harvester (SUSE)' },
  istiooperator: { component: 'Istio Operator', status: 'supported', source: 'istio/istio' },
  traefik: { component: 'Traefik', status: 'supported', source: 'traefik/traefik' },
  klipper: { component: 'Klipper', status: 'supported', source: 'k3s-io/klipper', note: 'K3s ServiceLB / helm controller.' },
  'klipper-helm': { component: 'Klipper Helm', status: 'supported', source: 'k3s-io/klipper' },
  'klipper-lb': { component: 'Klipper LB', status: 'supported', source: 'k3s-io/klipper' },
}

function normalizeComponentKey(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9-]/g, '')
}

/** Catalog repo for opt-in chart roots (partner charts, UI plugins, marketplace add-ons). */
export type ChartCatalog = 'rancher-charts' | 'partner-charts' | 'ui-plugins'

const CATALOG_LIFECYCLE: Record<ChartCatalog, ComponentLifecycleAnnotation> = {
  'rancher-charts': {
    component: 'Rancher Chart',
    status: 'supported',
    source: 'rancher/charts',
    note: 'SUSE Rancher marketplace chart.',
  },
  'partner-charts': {
    component: 'Partner Chart',
    status: 'supported',
    source: 'rancher/partner-charts',
    note: 'Third-party chart validated for the Rancher marketplace.',
  },
  'ui-plugins': {
    component: 'UI Plugin',
    status: 'supported',
    source: 'rancher/ui-plugin-charts',
    note: 'Rancher dashboard UI extension.',
  },
}

/** Annotate a chart by name, optional category, and catalog repo fallback. */
export function annotateChart(
  chartName: string,
  opts?: { category?: string; catalog?: ChartCatalog },
): ComponentLifecycleAnnotation | undefined {
  const byName = annotateComponent(chartName)
  if (byName) return byName
  if (opts?.category) {
    const byCategory = annotateComponent(opts.category)
    if (byCategory) return byCategory
  }
  if (opts?.catalog) return CATALOG_LIFECYCLE[opts.catalog]
  return undefined
}

/** Annotate a chart/image by component name. Returns undefined when no data exists. */
export function annotateComponent(name: string): ComponentLifecycleAnnotation | undefined {
  if (!name) return undefined
  const key = normalizeComponentKey(name)
  if (!key) return undefined
  // Direct match first, then longest-key substring match (e.g. rancher-monitoring-crd before monitoring).
  if (COMPONENT_LIFECYCLE[key]) return COMPONENT_LIFECYCLE[key]
  const keys = Object.keys(COMPONENT_LIFECYCLE).sort((a, b) => b.length - a.length)
  for (const k of keys) {
    if (key.includes(k)) return COMPONENT_LIFECYCLE[k]
  }
  return undefined
}

export function componentLifecycleTitle(ann: ComponentLifecycleAnnotation): string {
  const parts: string[] = [`${ann.component}: ${lifecycleStatusLabel(ann.status) || 'Supported'}`]
  if (ann.note) parts.push(ann.note)
  if (ann.eom) parts.push(`EOM ${ann.eom}`)
  if (ann.eol) parts.push(`EOL ${ann.eol}`)
  parts.push(`Source: ${ann.source}`)
  return parts.join(' · ')
}
