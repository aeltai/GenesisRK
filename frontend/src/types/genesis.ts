// API types matching the Go genesis serve API

export interface ClusterVersionInfo {
  versions: string[]
  sources?: Record<string, string> // version -> "kdm" | "github" | "both"
}

export interface Step1Details {
  kdmUrl: string
  imageListSource: string
}

export interface Step1OptionsResponse {
  hasRKE1: boolean
  capabilities: Record<string, ClusterVersionInfo>
  details: Step1Details
}

export interface TreeNode {
  id: string
  label: string
  kind: string
  count: number
  description?: string
  version?: string
  category?: string
  iconUrl?: string
  children?: TreeNode[]
}

export interface GenerateRequest {
  rancherVersion: string
  rancherVersions?: string[]
  isRPMGC: boolean
  includeCommunityImageLists: boolean
  includeAppCollectionCharts: boolean
  includePartnerCharts: boolean
  includeUIPluginCharts: boolean
  includeCertManager: boolean
  /** SUSE Observability Agent images from charts.rancher.com prime/suse-observability. */
  includeSuseObservability?: boolean
  /** SUSE Observability self-hosted platform (server) images — elasticsearch, kafka, hbase, etc. */
  includeSuseObservabilityServer?: boolean
  appCollectionAPIUser: string
  appCollectionAPIPassword: string
  distros: string[]
  cni: string
  /** Multiple CNIs (used when both distros are selected). Sent to backend as CNIs. */
  cnis?: string[]
  /** Target architecture for RKE2 image lists and registry checks (amd64 or arm64). */
  arch?: string
  loadBalancer: boolean
  lbK3sKlipper: boolean
  lbK3sTraefik: boolean
  lbRKE2Nginx: boolean
  lbRKE2Traefik: boolean
  includeWindows: boolean
  k3sVersions: string[]
  rke2Versions: string[]
  /** Include every KDM patch per Kubernetes minor (not only the latest). */
  includeDeprecatedPatches?: boolean
  /** Optional destination registry for mirror/save/load and Hauler; used in Next steps commands. */
  destinationRegistry?: string
}

export interface GenerateResponse {
  jobId: string
  roots: TreeNode[]
  basicCharts: TreeNode[]
  basicImageComponent: Record<string, string>
  pastSelection: string
}

export interface ExportRequest {
  jobId: string
  selectedComponentIDs: string[]
  chartNames: string[]
  selectedImageRefs: string[]
}
