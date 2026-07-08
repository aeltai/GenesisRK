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
    iconKey: 'k3s',
    docsUrl: 'https://docs.k3s.io/networking/networking-services#klipper-lb',
  },
  {
    id: 'lbK3sTraefik',
    label: 'Traefik',
    subtitle: 'K3s ingress controller',
    distro: 'k3s',
    iconKey: 'traefik',
    docsUrl: 'https://doc.traefik.io/traefik/',
    releaseUrl: 'https://github.com/traefik/traefik/releases',
  },
  {
    id: 'lbRKE2Nginx',
    label: 'NGINX Ingress',
    subtitle: 'RKE2 default ingress',
    distro: 'rke2',
    iconKey: 'nginx',
    docsUrl: 'https://kubernetes.github.io/ingress-nginx/',
    releaseUrl: 'https://github.com/kubernetes/ingress-nginx/releases',
  },
  {
    id: 'lbRKE2Traefik',
    label: 'Traefik',
    subtitle: 'RKE2 ingress controller',
    distro: 'rke2',
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
