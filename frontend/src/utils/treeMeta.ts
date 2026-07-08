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
  Essentials: 'Required images for Rancher, your Kubernetes distro, selected CNI, and ingress/load balancer.',
  AddOns: 'Optional Rancher marketplace charts — monitoring, logging, backup, storage, security, and more.',
  Rancher: 'Core Rancher server, agent, webhooks, Fleet GitOps, and auto-deployed system charts.',
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
    const tag = node.version || imageTag(node.label)
    if (tag) parts.push(`Tag: ${tag}`)
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
