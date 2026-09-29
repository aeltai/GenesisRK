/** Upstream docs and GitHub release links for Rancher stack components. */

import { brandIcon } from './brandIcons'

export interface CNIDefinition {
  id: string
  label: string
  hint?: string
  description: string
  color: string
  iconKey?: keyof typeof import('./brandIcons').BRAND_ICONS
  logoUrl?: string
  upstreamRepo?: string
  docsUrl: string
  rancherDocUrl?: string
}

export const CNI_CATALOG: Record<string, CNIDefinition> = {
  cni_calico: {
    id: 'cni_calico',
    label: 'Calico',
    description: 'Policy-driven L3 networking with optional network policies. Supported on K3s and RKE2.',
    color: '#f57c00',
    iconKey: 'calico',
    upstreamRepo: 'projectcalico/calico',
    docsUrl: 'https://docs.tigera.io/calico/latest/about/',
    rancherDocUrl: 'https://docs.rke2.io/networking/basic_network_options#calico-cni-plugin',
  },
  cni_canal: {
    id: 'cni_canal',
    label: 'Canal',
    hint: 'RKE2 default',
    description: 'Flannel overlay + Calico policy engine. Default CNI on RKE2.',
    color: '#5c6bc0',
    iconKey: 'canal',
    upstreamRepo: 'projectcalico/calico',
    docsUrl: 'https://docs.rke2.io/networking/basic_network_options#canal-cni-plugin',
    rancherDocUrl: 'https://docs.rke2.io/networking/basic_network_options#canal-cni-plugin',
  },
  cni_cilium: {
    id: 'cni_cilium',
    label: 'Cilium',
    description: 'eBPF-based networking, observability, and security. Supported on K3s and RKE2.',
    color: '#6366f1',
    iconKey: 'cilium',
    upstreamRepo: 'cilium/cilium',
    docsUrl: 'https://docs.cilium.io/en/stable/',
    rancherDocUrl: 'https://docs.rke2.io/networking/basic_network_options#cilium-cni-plugin',
  },
  cni_flannel: {
    id: 'cni_flannel',
    label: 'Flannel',
    hint: 'K3s default',
    description: 'Simple overlay network. Default CNI on K3s; also available on RKE2.',
    color: '#0ea5e9',
    iconKey: 'flannel',
    upstreamRepo: 'flannel-io/flannel',
    docsUrl: 'https://github.com/flannel-io/flannel#flannel',
    rancherDocUrl: 'https://docs.k3s.io/networking/basic-network-options',
  },
  cni: {
    id: 'cni',
    label: 'All CNI',
    description: 'Include images for every supported CNI (Calico, Canal, Cilium, Flannel).',
    color: '#64748b',
    iconKey: 'allCni',
    docsUrl: 'https://ranchermanager.docs.rancher.com/how-to-guides/new-user-guides/manage-clusters/manage-cluster-cnames',
  },
  '': {
    id: '',
    label: 'None',
    description: 'Skip CNI-specific images (distro core only).',
    color: '#475569',
    iconKey: 'none',
    docsUrl: 'https://ranchermanager.docs.rancher.com/',
  },
}

export interface LoadBalancerOption {
  id: 'lbK3sKlipper' | 'lbK3sTraefik' | 'lbRKE2Nginx' | 'lbRKE2Traefik'
  label: string
  subtitle: string
  distro: 'k3s' | 'rke2'
  // Role is informational (ingress vs ServiceLB). The UI allows multi-select so
  // air-gap lists can include every chosen option's images (e.g. both RKE2
  // NGINX and Traefik), even though a live cluster typically runs only one ingress.
  role: 'ingress' | 'lb-service'
  iconKey: keyof typeof import('./brandIcons').BRAND_ICONS
  docsUrl: string
  releaseUrl?: string
}

export const LOAD_BALANCER_OPTIONS: LoadBalancerOption[] = [
  {
    id: 'lbK3sKlipper',
    label: 'Klipper LB',
    subtitle: 'K3s built-in ServiceLB',
    distro: 'k3s',
    role: 'lb-service',
    iconKey: 'k3s',
    docsUrl: 'https://docs.k3s.io/networking/networking-services#klipper-lb',
  },
  {
    id: 'lbK3sTraefik',
    label: 'Traefik',
    subtitle: 'K3s ingress controller',
    distro: 'k3s',
    role: 'ingress',
    iconKey: 'traefik',
    docsUrl: 'https://doc.traefik.io/traefik/',
    releaseUrl: 'https://github.com/traefik/traefik/releases',
  },
  {
    id: 'lbRKE2Nginx',
    label: 'NGINX Ingress',
    subtitle: 'RKE2 default ingress',
    distro: 'rke2',
    role: 'ingress',
    iconKey: 'nginx',
    docsUrl: 'https://kubernetes.github.io/ingress-nginx/',
    releaseUrl: 'https://github.com/kubernetes/ingress-nginx/releases',
  },
  {
    id: 'lbRKE2Traefik',
    label: 'Traefik',
    subtitle: 'RKE2 ingress controller',
    distro: 'rke2',
    role: 'ingress',
    iconKey: 'traefik',
    docsUrl: 'https://doc.traefik.io/traefik/',
    releaseUrl: 'https://github.com/traefik/traefik/releases',
  },
]

export function cniIconUrl(def: Pick<CNIDefinition, 'iconKey' | 'logoUrl'>): string {
  if (def.iconKey) return brandIcon(def.iconKey)
  return def.logoUrl ?? brandIcon('kubernetes')
}

export interface ComponentLink {
  label: string
  href: string
  hint?: string
}

export function githubRelease(repo: string, tag?: string): string {
  const base = `https://github.com/${repo}/releases`
  if (!tag) return base
  return `${base}/tag/${encodeURIComponent(tag)}`
}

export function githubLatest(repo: string): string {
  return `https://github.com/${repo}/releases/latest`
}

export function rancherRelease(version: string): string {
  return githubRelease('rancher/rancher', version)
}

export function k3sRelease(version: string): string {
  return githubRelease('k3s-io/k3s', version)
}

export function rke2Release(version: string): string {
  return githubRelease('rancher/rke2', version)
}

export function cniRelease(cniId: string): string | undefined {
  const def = CNI_CATALOG[cniId]
  if (!def?.upstreamRepo) return undefined
  return githubLatest(def.upstreamRepo)
}

export function cniDocs(cniId: string): string {
  return CNI_CATALOG[cniId]?.docsUrl ?? 'https://ranchermanager.docs.rancher.com/'
}

export const STACK_COMPONENTS = {
  coredns: {
    label: 'CoreDNS',
    repo: 'coredns/coredns',
    docs: 'https://coredns.io/manual/toc/',
  },
  fleet: {
    label: 'Fleet',
    repo: 'rancher/fleet',
    docs: 'https://fleet.rancher.io/',
  },
  metricsServer: {
    label: 'Metrics Server',
    repo: 'kubernetes-sigs/metrics-server',
    docs: 'https://github.com/kubernetes-sigs/metrics-server#kubernetes-metrics-server',
  },
  ingressNginx: {
    label: 'Ingress NGINX',
    repo: 'kubernetes/ingress-nginx',
    docs: 'https://kubernetes.github.io/ingress-nginx/',
  },
  traefik: {
    label: 'Traefik',
    repo: 'traefik/traefik',
    docs: 'https://doc.traefik.io/traefik/',
  },
} as const satisfies Record<string, { label: string; repo: string; docs: string }>

export function loadBalancerLinks(opts: {
  lbK3sKlipper: boolean
  lbK3sTraefik: boolean
  lbRKE2Nginx: boolean
  lbRKE2Traefik: boolean
}): ComponentLink[] {
  const out: ComponentLink[] = []
  if (opts.lbK3sKlipper) {
    out.push({ label: 'K3s Klipper LB', href: 'https://docs.k3s.io/networking/networking-services#klipper-lb', hint: 'Built-in ServiceLB' })
  }
  if (opts.lbK3sTraefik) {
    out.push({ label: 'K3s Traefik', href: githubLatest('traefik/traefik'), hint: 'Ingress' })
  }
  if (opts.lbRKE2Nginx) {
    out.push({ label: 'RKE2 NGINX Ingress', href: githubLatest('kubernetes/ingress-nginx'), hint: 'Default RKE2 ingress' })
  }
  if (opts.lbRKE2Traefik) {
    out.push({ label: 'RKE2 Traefik', href: githubLatest('traefik/traefik'), hint: 'Ingress' })
  }
  return out
}

export function selectedCniDefinition(cniId: string): CNIDefinition {
  return CNI_CATALOG[cniId] ?? CNI_CATALOG['']!
}

export function formatVersionList(versions: string[], max = 3): string {
  if (!versions?.length || versions.includes('all')) return 'All'
  if (versions.length <= max) return versions.join(', ')
  return `${versions.slice(0, max).join(', ')} +${versions.length - max}`
}

/** Upstream open-source project for container images (GitHub + optional docs). */
export interface UpstreamProject {
  repo: string
  docs?: string
  label: string
}

/**
 * Image/chart name fragment → upstream OSS project.
 * Keys are matched against image short name, repo path segments, and chart name (longest key wins).
 */
export const UPSTREAM_PROJECTS: Record<string, UpstreamProject> = {
  'hardened-calico': { repo: 'projectcalico/calico', docs: 'https://docs.tigera.io/calico/latest/about/', label: 'Calico' },
  'calico-node': { repo: 'projectcalico/calico', docs: 'https://docs.tigera.io/calico/latest/about/', label: 'Calico' },
  'calico-kube-controllers': { repo: 'projectcalico/calico', label: 'Calico' },
  calico: { repo: 'projectcalico/calico', docs: 'https://docs.tigera.io/calico/latest/about/', label: 'Calico' },
  'hardened-canal': { repo: 'projectcalico/calico', docs: 'https://docs.rke2.io/networking/basic_network_options#canal-cni-plugin', label: 'Canal' },
  canal: { repo: 'projectcalico/calico', docs: 'https://docs.rke2.io/networking/basic_network_options#canal-cni-plugin', label: 'Canal' },
  'hardened-cilium': { repo: 'cilium/cilium', docs: 'https://docs.cilium.io/en/stable/', label: 'Cilium' },
  cilium: { repo: 'cilium/cilium', docs: 'https://docs.cilium.io/en/stable/', label: 'Cilium' },
  'hardened-flannel': { repo: 'flannel-io/flannel', docs: 'https://github.com/flannel-io/flannel#flannel', label: 'Flannel' },
  flannel: { repo: 'flannel-io/flannel', docs: 'https://github.com/flannel-io/flannel#flannel', label: 'Flannel' },
  coredns: { repo: 'coredns/coredns', docs: STACK_COMPONENTS.coredns.docs, label: 'CoreDNS' },
  'rke2-coredns': { repo: 'coredns/coredns', docs: STACK_COMPONENTS.coredns.docs, label: 'CoreDNS' },
  traefik: { repo: 'traefik/traefik', docs: STACK_COMPONENTS.traefik.docs, label: 'Traefik' },
  'ingress-nginx': { repo: 'kubernetes/ingress-nginx', docs: STACK_COMPONENTS.ingressNginx.docs, label: 'Ingress NGINX' },
  'nginx-ingress-controller': { repo: 'kubernetes/ingress-nginx', docs: STACK_COMPONENTS.ingressNginx.docs, label: 'Ingress NGINX' },
  'metrics-server': { repo: 'kubernetes-sigs/metrics-server', docs: STACK_COMPONENTS.metricsServer.docs, label: 'Metrics Server' },
  'kube-state-metrics': { repo: 'kubernetes/kube-state-metrics', label: 'kube-state-metrics' },
  'node-exporter': { repo: 'prometheus/node_exporter', label: 'node-exporter' },
  prometheus: { repo: 'prometheus/prometheus', label: 'Prometheus' },
  alertmanager: { repo: 'prometheus/alertmanager', label: 'Alertmanager' },
  grafana: { repo: 'grafana/grafana', label: 'Grafana' },
  thanos: { repo: 'thanos-io/thanos', label: 'Thanos' },
  'prometheus-operator': { repo: 'prometheus-operator/prometheus-operator', label: 'Prometheus Operator' },
  'config-reloader': { repo: 'prometheus-operator/prometheus-operator', label: 'Prometheus Operator' },
  'fluent-bit': { repo: 'fluent/fluent-bit', label: 'Fluent Bit' },
  fluentbit: { repo: 'fluent/fluent-bit', label: 'Fluent Bit' },
  fluentd: { repo: 'fluent/fluentd', label: 'Fluentd' },
  velero: { repo: 'vmware-tanzu/velero', label: 'Velero' },
  longhorn: { repo: 'longhorn/longhorn', label: 'Longhorn' },
  'longhorn-manager': { repo: 'longhorn/longhorn', label: 'Longhorn' },
  'longhorn-engine': { repo: 'longhorn/longhorn', label: 'Longhorn' },
  'longhorn-instance-manager': { repo: 'longhorn/longhorn', label: 'Longhorn' },
  neuvector: { repo: 'neuvector/neuvector', label: 'NeuVector' },
  gatekeeper: { repo: 'open-policy-agent/gatekeeper', label: 'Gatekeeper' },
  fleet: { repo: 'rancher/fleet', docs: STACK_COMPONENTS.fleet.docs, label: 'Fleet' },
  'fleet-agent': { repo: 'rancher/fleet', docs: STACK_COMPONENTS.fleet.docs, label: 'Fleet' },
  'local-path-provisioner': { repo: 'rancher/local-path-provisioner', label: 'Local Path Provisioner' },
  'snapshot-controller': { repo: 'kubernetes-csi/external-snapshotter', label: 'Snapshot Controller' },
  'system-upgrade-controller': { repo: 'rancher/system-upgrade-controller', label: 'System Upgrade Controller' },
  rancher: { repo: 'rancher/rancher', label: 'Rancher' },
  'rancher-agent': { repo: 'rancher/rancher', label: 'Rancher' },
  pause: { repo: 'kubernetes/kubernetes', label: 'Kubernetes pause' },
  'kube-proxy': { repo: 'kubernetes/kubernetes', label: 'Kubernetes' },
  'kube-apiserver': { repo: 'kubernetes/kubernetes', label: 'Kubernetes' },
  'kube-controller-manager': { repo: 'kubernetes/kubernetes', label: 'Kubernetes' },
  'kube-scheduler': { repo: 'kubernetes/kubernetes', label: 'Kubernetes' },
  etcd: { repo: 'etcd-io/etcd', label: 'etcd' },
  istio: { repo: 'istio/istio', label: 'Istio' },
  pilot: { repo: 'istio/istio', label: 'Istio' },
  proxyv2: { repo: 'istio/istio', label: 'Istio' },
}

function normalizeUpstreamKey(name: string): string {
  return name.toLowerCase().replace(/[^a-z0-9-]/g, '')
}

/** Resolve upstream OSS project for an image ref (and optional chart name). */
export function resolveUpstreamProject(ref: string, chartName?: string, shortName?: string, repoPath?: string): UpstreamProject | undefined {
  const short = normalizeUpstreamKey(shortName ?? ref.split('/').pop()?.split(':')[0] ?? ref)
  const parts = (repoPath ?? ref).split(/[/:]/).map(normalizeUpstreamKey).filter(Boolean)
  const chart = chartName ? normalizeUpstreamKey(chartName) : ''
  const candidates = [short, chart, ...parts]

  for (const c of candidates) {
    if (c && UPSTREAM_PROJECTS[c]) return UPSTREAM_PROJECTS[c]
  }

  const keys = Object.keys(UPSTREAM_PROJECTS).sort((a, b) => b.length - a.length)
  for (const c of candidates) {
    if (!c) continue
    for (const k of keys) {
      if (c.includes(k)) return UPSTREAM_PROJECTS[k]
    }
  }
  return undefined
}

/** GitHub releases page for the upstream OSS project behind an image. */
export function imageUpstreamReleaseUrl(
  ref: string,
  chartName?: string,
  shortName?: string,
  repoPath?: string,
  tag?: string,
): string | undefined {
  const project = resolveUpstreamProject(ref, chartName, shortName, repoPath)
  if (!project) return undefined
  if (tag) {
    const t = tag.startsWith('v') ? tag : `v${tag}`
    return githubRelease(project.repo, t)
  }
  return githubLatest(project.repo)
}

/** Upstream documentation URL when known. */
export function imageUpstreamDocsUrl(
  ref: string,
  chartName?: string,
  shortName?: string,
  repoPath?: string,
): string | undefined {
  return resolveUpstreamProject(ref, chartName, shortName, repoPath)?.docs
}

/** Human label for upstream project link text. */
export function imageUpstreamLabel(
  ref: string,
  chartName?: string,
  shortName?: string,
  repoPath?: string,
): string | undefined {
  return resolveUpstreamProject(ref, chartName, shortName, repoPath)?.label
}
