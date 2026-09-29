import type { TreeNode } from '../types/genesis'

/** Rancher tree node kind labels for UI badges */
export const KIND_LABELS: Record<string, string> = {
  preset: 'Preset',
  component: 'Group',
  chart_all: 'Charts',
  chart: 'Chart',
  image: 'Image',
}

/** Fallback descriptions when API does not provide node.description */
export const GROUP_DESCRIPTIONS: Record<string, string> = {
  Essentials: 'Required images for Rancher, cert-manager, your Kubernetes distro, selected CNI, and ingress/load balancer.',
  AddOns: 'Optional Rancher marketplace charts — monitoring, logging, backup, storage, security, and more.',
  Rancher: 'Core Rancher server, agent, webhooks, Fleet GitOps, and auto-deployed system charts.',
  'Cert Manager': 'jetstack cert-manager — required TLS prerequisite before installing Rancher with Helm.',
  CNI: 'Container network interface — pod networking (Calico, Canal, Flannel, Cilium).',
  K3s: 'Lightweight Kubernetes distribution images for selected K3s version(s).',
  RKE2: 'RKE2 node and system images for selected version(s).',
  RKE1: 'Legacy RKE1 cluster provisioning images.',
  'Load Balancer / Ingress': 'Ingress controller or load-balancer for exposing services.',
}

export const CATEGORY_DESCRIPTIONS: Record<string, string> = {
  monitoring: 'Prometheus, Grafana, Alertmanager — observability stack.',
  logging: 'Fluent Bit / Fluentd log collection and forwarding.',
  'backup-restore': 'Velero and backup-restore operators.',
  storage: 'Longhorn, Harvester, CSI drivers, persistent storage.',
  security: 'NeuVector, Gatekeeper, runtime security.',
  cis: 'CIS benchmark scanning and compliance.',
  provisioning: 'Cloud provider operators (EKS, GKE, AKS, vSphere, CAPI).',
  networking: 'Service mesh (Istio), SR-IOV, advanced networking.',
  'cluster-api': 'Cluster API providers and Rancher Turtles.',
  'os-management': 'Elemental and edge/OS lifecycle management.',
  support: 'Supportability and diagnostic tools.',
  fleet: 'Fleet GitOps — deploy from Git across clusters.',
  system: 'Charts auto-deployed by Rancher (webhook, provisioning-capi).',
  'cert-manager': 'TLS certificate controller — install before the Rancher Helm chart.',
  other: 'Additional marketplace charts.',
}

export interface ParsedChartLabel {
  name: string
  version: string
  category: string
}

/** Parse chart label like "rancher-monitoring 6.3.0 [monitoring]" */
export function parseChartLabel(label: string): ParsedChartLabel {
  let rest = label.trim()
  let category = ''
  const catMatch = rest.match(/\s*\[([^\]]+)\]\s*$/)
  if (catMatch && catMatch[1]) {
    category = catMatch[1]
    rest = rest.slice(0, catMatch.index ?? rest.length).trim()
  }
  rest = rest.replace(/\s*\[auto-deployed\]\s*$/, '').trim()
  const parts = rest.split(/\s+/)
  const last = parts[parts.length - 1] ?? ''
  if (parts.length >= 2 && /^v?\d/.test(last)) {
    return {
      name: parts.slice(0, -1).join(' '),
      version: last,
      category,
    }
  }
  return { name: rest, version: '', category }
}

export function chartDisplayName(node: TreeNode, fallbackId = ''): string {
  if (node.kind !== 'chart') return node.label
  const parsed = parseChartLabel(node.label)
  return parsed.name || fallbackId
}

export function chartVersion(node: TreeNode): string {
  return node.version || parseChartLabel(node.label).version
}

export function chartCategory(node: TreeNode): string {
  return node.category || parseChartLabel(node.label).category
}

export function imageTag(ref: string): string {
  const i = ref.lastIndexOf(':')
  if (i > 0 && !ref.slice(i + 1).includes('/')) return ref.slice(i + 1)
  return ''
}

/** Image ref without tag (handles registry hosts with ports). */
export function imageRepo(ref: string): string {
  const tag = imageTag(ref)
  if (!tag) return ref
  const i = ref.lastIndexOf(':')
  return i > 0 ? ref.slice(0, i) : ref
}

export interface ParsedImageRef {
  registry: string
  repoPath: string
  shortName: string
  tag: string
  fullRef: string
}

/** Split an image ref into registry, repository path, short name, and tag. */
export function parseImageRef(ref: string): ParsedImageRef {
  const trimmed = ref.trim()
  const tag = imageTag(trimmed)
  const withoutTag = imageRepo(trimmed)

  let registry = 'docker.io'
  let repoPath = withoutTag
  const slashIdx = withoutTag.indexOf('/')
  if (slashIdx > 0) {
    const first = withoutTag.slice(0, slashIdx)
    if (first.includes('.') || first.includes(':') || first === 'localhost') {
      registry = first
      repoPath = withoutTag.slice(slashIdx + 1)
    }
  }

  const parts = repoPath.split('/').filter(Boolean)
  const shortName = parts.length > 0 ? parts[parts.length - 1]! : repoPath || trimmed

  return { registry, repoPath, shortName, tag, fullRef: trimmed }
}

/** Last path segment of the repository (e.g. prometheus from quay.io/prometheus/prometheus). */
export function imageShortName(ref: string): string {
  return parseImageRef(ref).shortName
}

/** Repository path without registry host (e.g. rancher/rancher). */
export function imageRepoPath(ref: string): string {
  return parseImageRef(ref).repoPath
}

/** Registry host for an image ref (docker.io when implicit). */
export function imageRegistryHost(ref: string): string {
  return parseImageRef(ref).registry
}

/** Best-effort registry or Docker Hub page for an image ref. */
export function imageRegistryUrl(ref: string): string | undefined {
  const trimmed = ref.trim()
  if (!trimmed) return undefined

  let registry = 'docker.io'
  let path = trimmed
  const slashIdx = trimmed.indexOf('/')
  if (slashIdx > 0) {
    const first = trimmed.slice(0, slashIdx)
    if (first.includes('.') || first.includes(':') || first === 'localhost') {
      registry = first
      path = trimmed.slice(slashIdx + 1)
    }
  }

  const colonIdx = path.lastIndexOf(':')
  let repo = path
  if (colonIdx > 0 && !path.slice(colonIdx + 1).includes('/')) {
    repo = path.slice(0, colonIdx)
  }

  if (registry === 'docker.io' || registry === 'index.docker.io') {
    return `https://hub.docker.com/r/${repo}`
  }
  return `https://${registry}/${repo}`
}

export function formatBytes(n: number): string {
  if (!n || n <= 0) return ''
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let v = n
  let u = 0
  while (v >= 1024 && u < units.length - 1) {
    v /= 1024
    u++
  }
  return `${v < 10 && u > 0 ? v.toFixed(1) : Math.round(v)} ${units[u]}`
}

export function nodeTooltip(node: TreeNode): string {
  const parts: string[] = []
  if (node.description) {
    parts.push(node.description)
  } else if (node.kind === 'component') {
    const base = node.label.replace(/\s*\(.*/, '').trim()
    parts.push(GROUP_DESCRIPTIONS[base] || GROUP_DESCRIPTIONS[node.label] || '')
  } else if (node.kind === 'chart') {
    const cat = chartCategory(node)
    if (cat) parts.push(CATEGORY_DESCRIPTIONS[cat] || `Category: ${cat}`)
  }
  if (node.version) parts.push(`Version: ${node.version}`)
  if (node.category && node.kind === 'chart') parts.push(`Category: ${node.category}`)
  if (node.kind === 'image') {
    parts.unshift(`Ref: ${node.label}`)
    const tag = node.version || imageTag(node.label)
    if (tag) parts.push(`Tag: ${tag}`)
    const repo = imageRepoPath(node.label)
    if (repo) parts.push(`Repository: ${repo}`)
  }
  if (node.count > 0 && node.kind !== 'image') {
    parts.push(`${node.count} item${node.count === 1 ? '' : 's'}`)
  }
  return parts.filter(Boolean).join('\n')
}

export function isExpandableNode(node: TreeNode): boolean {
  return (
    (node.kind === 'preset' ||
      node.kind === 'component' ||
      node.kind === 'chart_all' ||
      node.kind === 'chart') &&
    !!node.children &&
    node.children.length > 0
  )
}
