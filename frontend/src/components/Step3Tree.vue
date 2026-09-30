<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted, nextTick } from 'vue'
import type { TreeNode } from '../types/genesis'
import { checkAvailability, fetchImageSizes, fetchReleaseNotes, startScan, getScanStatus, downloadScanReport, fetchLogs, type AvailabilityResult, type ImageSizeResult, type ReleaseInfo, type ScanStatusResponse } from '../api/genesis'
import LoadingProgressBar from './LoadingProgressBar.vue'
import ServerLogsPanel from './ServerLogsPanel.vue'
import { formatLogsForDisplay } from '../utils/serverLogs'
import {
  chartCategory,
  chartDisplayName,
  chartVersion,
  formatBytes,
  imageRegistryUrl,
  imageRegistryHost,
  imageRepoPath,
  imageShortName,
  imageTag,
  isExpandableNode,
  KIND_LABELS,
  nodeTooltip,
} from '../utils/treeMeta'
import {
  cniDocs,
  cniRelease,
  githubRelease,
  imageUpstreamDocsUrl,
  imageUpstreamLabel,
  imageUpstreamReleaseUrl,
  resolveUpstreamProject,
  STACK_COMPONENTS,
} from '../utils/componentLinks'
import {
  annotateComponent,
  annotateChart,
  type ChartCatalog,
  annotateDistroVersions,
  annotateRancherVersions,
  componentLifecycleTitle,
  lifecycleStatusLabel,
  lifecycleStatusTitle,
  type ComponentLifecycleAnnotation,
  type VersionAnnotation,
} from '../utils/versionLifecycle'
import {
  eolProductPageUrl,
  eolProductSlug,
  lifecycleBadgeLabel,
  mergeLifecycleAnnotations,
  prefetchEolProducts,
  upstreamLifecycleForImageRef,
} from '../utils/endoflife'
import {
  isRancherCowIcon,
} from '../utils/rancherBrandIcons'
import { resolveImageIcons, resolveTreeNodeIcons } from '../utils/treeIcons'

const props = defineProps<{
  roots: TreeNode[]
  basicCharts: TreeNode[]
  basicImageComponent: Record<string, string>
  pastSelection: string
  components: string
  distros: string[]
  cniForStandard: string
  rancherVersions: string[]
  rke2Versions: string[]
  k3sVersions: string[]
  availableRancherVersions?: string[]
  availableK3sVersions?: string[]
  availableRke2Versions?: string[]
  loadBalancers?: string[]
  includeWindows?: boolean
  destinationRegistry?: string
  isPrime?: boolean
  arch?: string
  includePartnerCharts?: boolean
  includeUIPluginCharts?: boolean
}>()

const emit = defineEmits<{
  exportList: [selectedComponentIDs: string[], chartNames: string[], selectedImageRefs: string[]]
  back: []
  'update:destinationRegistry': [value: string]
}>()

// Mobile: switch between Groups / Charts / Images so each view gets full width
const mobileTab = ref<'groups' | 'charts' | 'images'>('groups')

// Flatten tree with expand state
const expanded = reactive<Record<string, boolean>>({})
const selected = reactive<Record<string, boolean>>({})
const expandedImages = reactive<Record<string, boolean>>({})

function toggleImageExpand(ref: string) {
  expandedImages[ref] = !expandedImages[ref]
}

function selectAll(node: TreeNode) {
  selected[node.id] = true
  for (const c of node.children || []) selectAll(c)
}

function initSelection() {
  if (!props.roots?.length) return
  for (const r of props.roots) {
    expanded[r.id] = true
    if (r.id === 'basic') {
      selectAll(r)
      for (const c of r.children || []) {
        expanded[c.id] = true
      }
    }
    if (r.id === 'source_ui_plugin_charts' || r.id === 'source_partner_charts') {
      expanded[r.id] = true
      selectAll(r)
    }
  }
}
watch(() => props.roots, () => initSelection(), { immediate: true })

interface Row {
  depth: number
  node: TreeNode
}

const visibleRows = computed(() => {
  const out: Row[] = []
  if (!props.roots?.length) return out
  function walk(nodes: TreeNode[], depth: number) {
    for (const n of nodes) {
      out.push({ depth, node: n })
      const isExpandable = isExpandableNode(n)
      if (isExpandable && expanded[n.id]) {
        walk(n.children!, depth + 1)
      }
    }
  }
  walk(props.roots, 0)
  return out
})

function toggleExpand(id: string) {
  expanded[id] = !expanded[id]
}

// Hide chart logos whose URL fails to load (falls back to text-only row)
function hideBrokenIcon(e: Event) {
  const img = e.target as HTMLImageElement | null
  if (img) img.style.display = 'none'
}

function setNodeSelected(node: TreeNode, checked: boolean) {
  selected[node.id] = checked
  if (node.children?.length) {
    for (const c of node.children) setNodeSelected(c, checked)
  }
}

function onSelectChange(row: Row, e: Event) {
  const checked = (e.target as HTMLInputElement).checked
  setNodeSelected(row.node, checked)
}

// Build chart info: label + parent group + version/category for preview column
interface ChartInfo {
  label: string
  group: string
  version: string
  category: string
  description: string
}

const chartInfoMap = computed(() => {
  const m: Record<string, ChartInfo> = {}
  function walk(nodes: TreeNode[], parentGroup: string) {
    for (const n of nodes) {
      if (n.kind === 'chart') {
        m[n.id] = {
          label: chartDisplayName(n, n.id),
          group: parentGroup,
          version: chartVersion(n),
          category: chartCategory(n),
          description: n.description || nodeTooltip(n),
        }
      }
      if (n.children) {
        const g = n.kind === 'component' ? n.label.replace(/\s*\(.*/, '') : parentGroup
        walk(n.children, g)
      }
    }
  }
  walk(props.roots, '')
  return m
})

const previewCharts = computed(() => {
  const set = new Set<string>()
  function walk(nodes: TreeNode[]) {
    for (const n of nodes) {
      if (!selected[n.id]) continue
      if (n.kind === 'chart') set.add(n.id)
      if (n.children?.length) walk(n.children)
    }
  }
  walk(props.roots)
  return [...set].sort()
})

const previewImages = computed(() => {
  const set = new Set<string>()
  function walk(nodes: TreeNode[]) {
    for (const n of nodes) {
      if (!selected[n.id]) continue
      if (n.kind === 'image') set.add(n.label)
      if (n.children?.length) walk(n.children)
    }
  }
  walk(props.roots)
  return [...set].sort()
})

const imageSourceGroup = computed(() => {
  const m: Record<string, string> = {}
  for (const img of previewImages.value) {
    if (props.basicImageComponent[img]) m[img] = props.basicImageComponent[img]
    else m[img] = 'addons'
  }
  return m
})

const imageChartMap = computed(() => {
  const m: Record<string, string> = {}
  function walk(nodes: TreeNode[], parentChart: string) {
    for (const n of nodes) {
      if (n.kind === 'image') {
        m[n.label] = parentChart
      }
      const chart = n.kind === 'chart' ? (chartInfoMap.value[n.id]?.label || n.label) : parentChart
      if (n.children) walk(n.children, chart)
    }
  }
  walk(props.roots, '')
  return m
})

/** Chart id → metadata for lifecycle badges (category + catalog repo). */
const chartMetaById = computed(() => {
  const m: Record<string, { label: string; category?: string; catalog?: ChartCatalog }> = {}
  function walk(nodes: TreeNode[], catalog?: ChartCatalog) {
    for (const n of nodes) {
      let c = catalog
      if (n.id === 'source_partner_charts') c = 'partner-charts'
      else if (n.id === 'source_ui_plugin_charts') c = 'ui-plugins'
      else if (n.id === 'addons' || n.id.startsWith('addon_')) c = 'rancher-charts'
      if (n.kind === 'chart') {
        m[n.id] = {
          label: chartDisplayName(n, n.id),
          category: n.category || chartCategory(n) || undefined,
          catalog: c,
        }
      }
      if (n.children) walk(n.children, c)
    }
  }
  walk(props.roots, undefined)
  return m
})

const imageChartIdMap = computed(() => {
  const m: Record<string, string> = {}
  function walk(nodes: TreeNode[], parentChartId = '') {
    for (const n of nodes) {
      if (n.kind === 'image') m[n.label] = parentChartId
      const chartId = n.kind === 'chart' ? n.id : parentChartId
      if (n.children) walk(n.children, chartId)
    }
  }
  walk(props.roots, '')
  return m
})

/** Parent chart logo URL for each image ref (from Helm chart metadata). */
const imageChartIconMap = computed(() => {
  const m: Record<string, string> = {}
  function walk(nodes: TreeNode[], parentIcon = '') {
    for (const n of nodes) {
      if (n.kind === 'image' && parentIcon) m[n.label] = parentIcon
      const icon = n.kind === 'chart' && n.iconUrl ? n.iconUrl : parentIcon
      if (n.children) walk(n.children, icon)
    }
  }
  walk(props.roots, '')
  return m
})

function resolveRowIcons(node: TreeNode) {
  if (node.kind === 'image') {
    return resolveImageRowIcons(node.label)
  }
  const chartId = node.kind === 'chart' ? node.id : undefined
  return resolveTreeNodeIcons(node, {
    isPrime: props.isPrime,
    chartGroup: chartId ? chartInfoMap.value[chartId]?.group : undefined,
    chartCatalog: chartId ? chartMetaById.value[chartId]?.catalog : undefined,
    chartId,
    chartIconUrl: node.iconUrl,
  })
}

function resolveImageRowIcons(img: string) {
  const chartId = imageChartIdMap.value[img]
  return resolveImageIcons(img, {
    isPrime: props.isPrime,
    sourceGroup: imageSourceGroup.value[img],
    chartId,
    chartName: imageChartMap.value[img],
    chartIconUrl: imageChartIconMap.value[img],
    chartCatalog: chartId ? chartMetaById.value[chartId]?.catalog : undefined,
  })
}

function hideBrokenSecondaryIcon(e: Event) {
  const img = e.target as HTMLImageElement | null
  if (img?.parentElement) img.parentElement.style.display = 'none'
}

function doExport() {
  const selectedImageRefs = previewImages.value
  const componentIDs: string[] = []
  const chartNames = previewCharts.value

  for (const r of visibleRows.value) {
    if (!selected[r.node.id]) continue
    if (r.node.id === 'basic') {
      const comps = props.components.split(',').map((c) => c.trim()).filter(Boolean)
      if (props.cniForStandard) componentIDs.push(props.cniForStandard)
      componentIDs.push('fleet')
      for (const c of comps) {
        if (c === 'k3s') componentIDs.push('k3s')
        else if (c === 'rke2') componentIDs.push('rke2')
        else if (c === 'rke') componentIDs.push('rke1')
      }
    } else if (r.node.id === 'addons') {
      // chartNames already collected from tree
    } else if (r.node.id === 'app_collection') {
      componentIDs.push('app_collection')
      componentIDs.push('app_collection_containers')
    }
  }

  emit('exportList', componentIDs, chartNames, selectedImageRefs)
}

const availResults = ref<AvailabilityResult>({})
const availLoading = ref(false)
const availChecked = ref(false)
const availError = ref('')

const sizeResults = ref<ImageSizeResult>({})
const sizesLoading = ref(false)
const sizesChecked = ref(false)
const sizesError = ref('')

const targetArch = computed(() => props.arch?.trim() || 'amd64')

async function doCheckAvailability() {
  const imgs = previewImages.value
  if (imgs.length === 0) return
  availLoading.value = true
  availChecked.value = false
  availError.value = ''
  availResults.value = {}
  try {
    availResults.value = await checkAvailability(imgs, targetArch.value)
    availChecked.value = true
  } catch (e) {
    availError.value = e instanceof Error ? e.message : String(e)
  } finally {
    availLoading.value = false
  }
}

async function doFetchImageSizes() {
  const imgs = previewImages.value
  if (imgs.length === 0) return
  sizesLoading.value = true
  sizesChecked.value = false
  sizesError.value = ''
  sizeResults.value = {}
  try {
    sizeResults.value = await fetchImageSizes(imgs, targetArch.value)
    sizesChecked.value = true
  } catch (e) {
    sizesError.value = e instanceof Error ? e.message : String(e)
  } finally {
    sizesLoading.value = false
  }
}

// Scan (Trivy) – scan selected images for vulnerabilities
const scanLoading = ref(false)
const scanJobId = ref<string | null>(null)
const scanStatus = ref<ScanStatusResponse | null>(null)
const scanError = ref('')
const scanPollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const scanButtonDisabled = computed(() =>
  previewImages.value.length === 0 || scanLoading.value || scanStatus.value?.status === 'running'
)
const scanButtonLabel = computed(() => {
  if (scanStatus.value?.status === 'running') return 'Ongoing scan…'
  if (scanLoading.value) return 'Scanning…'
  return 'Scan selected images'
})

async function doScan() {
  const imgs = previewImages.value
  if (imgs.length === 0) return
  scanLoading.value = true
  scanJobId.value = null
  scanStatus.value = null
  scanError.value = ''
  scanLogs.value = []
  startScanLogsPoll()
  try {
    const { scanJobId: id } = await startScan(imgs)
    scanJobId.value = id
    const poll = async () => {
      if (!scanJobId.value) return
      try {
        const st = await getScanStatus(scanJobId.value)
        scanStatus.value = st
        if (st.status === 'running') return
        if (scanPollTimer.value) {
          clearInterval(scanPollTimer.value)
          scanPollTimer.value = null
        }
      } catch (e) {
        scanError.value = e instanceof Error ? e.message : String(e)
        if (scanPollTimer.value) {
          clearInterval(scanPollTimer.value)
          scanPollTimer.value = null
        }
      }
    }
    await poll()
    const current = scanStatus.value as ScanStatusResponse | null
    if (current?.status === 'running') {
      scanPollTimer.value = setInterval(poll, 4000)
    }
  } catch (e) {
    scanError.value = e instanceof Error ? e.message : String(e)
  } finally {
    scanLoading.value = false
  }
}

async function doDownloadScanReport() {
  if (!scanJobId.value || scanStatus.value?.status !== 'completed') return
  try {
    const blob = await downloadScanReport(scanJobId.value)
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'scan-report.csv'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    scanError.value = e instanceof Error ? e.message : String(e)
  }
}

onUnmounted(() => {
  if (scanPollTimer.value) {
    clearInterval(scanPollTimer.value)
    scanPollTimer.value = null
  }
  stopScanLogsPoll()
})

// Server logs during scan
const scanLogs = ref<string[]>([])
const scanLogsPollTimer = ref<ReturnType<typeof setInterval> | null>(null)

const scanInProgress = computed(() =>
  scanLoading.value || scanStatus.value?.status === 'running'
)
const scanProgress = computed(() => {
  const st = scanStatus.value
  if (st?.percent != null && st.phase) {
    return {
      percent: st.percent,
      phase: st.phase,
      current: st.current,
      total: st.total,
    }
  }
  return { percent: 5, phase: 'Initializing vulnerability scan…' }
})
const formattedScanLogs = computed(() => formatLogsForDisplay(scanLogs.value))

function startScanLogsPoll() {
  if (scanLogsPollTimer.value) return
  async function poll() {
    try {
      scanLogs.value = await fetchLogs()
    } catch {
      // ignore
    }
    if (!scanInProgress.value && scanLogsPollTimer.value) {
      clearInterval(scanLogsPollTimer.value)
      scanLogsPollTimer.value = null
    }
  }
  poll()
  scanLogsPollTimer.value = setInterval(poll, 1500)
}

function stopScanLogsPoll() {
  if (scanLogsPollTimer.value) {
    clearInterval(scanLogsPollTimer.value)
    scanLogsPollTimer.value = null
  }
}

function imageTooltip(img: string): string {
  const parts: string[] = [img]
  const tag = imageTag(img)
  if (tag) parts.push(`Tag: ${tag}`)
  if (imageChartMap.value[img]) parts.push(`Chart: ${imageChartMap.value[img]}`)
  if (imageSourceGroup.value[img]) parts.push(`Group: ${imageSourceGroup.value[img]}`)
  const upstream = resolveUpstreamProject(img, imageChartMap.value[img], imageShortName(img), imageRepoPath(img))
  if (upstream) parts.push(`Upstream: ${upstream.label} (${upstream.repo})`)
  const avail = availResults.value[img]
  if (avail?.status === 'ok' && avail.sizeBytes) {
    parts.push(`Pull size (compressed): ${formatBytes(avail.sizeBytes)}`)
  } else if (avail?.detail) {
    parts.push(avail.detail)
  }
  return parts.join('\n')
}

function imageUpstreamTagForLink(img: string): string | undefined {
  const tag = imageTag(img)
  if (!tag || !/^v?\d/.test(tag)) return undefined
  return tag
}

function imageUpstreamReleasesLink(img: string): string | undefined {
  return imageUpstreamReleaseUrl(
    img,
    imageChartMap.value[img],
    imageShortName(img),
    imageRepoPath(img),
    imageUpstreamTagForLink(img),
  )
}

function imageUpstreamDocsLink(img: string): string | undefined {
  return imageUpstreamDocsUrl(img, imageChartMap.value[img], imageShortName(img), imageRepoPath(img))
}

function imageUpstreamName(img: string): string | undefined {
  return imageUpstreamLabel(img, imageChartMap.value[img], imageShortName(img), imageRepoPath(img))
}

const glossaryOpen = ref(false)

/** Known chart name → upstream GitHub repo for release links */
const CHART_UPSTREAM: Record<string, string> = {
  'rancher-monitoring': 'rancher/charts',
  'rancher-logging': 'rancher/charts',
  'longhorn': 'longhorn/longhorn',
  'cilium': 'cilium/cilium',
  'calico': 'projectcalico/calico',
  'traefik': 'traefik/traefik',
  'ingress-nginx': 'kubernetes/ingress-nginx',
  'fleet': 'rancher/fleet',
  'coredns': 'coredns/coredns',
}

function chartReleaseUrl(_chartId: string, label: string): string | undefined {
  const name = label.toLowerCase()
  for (const [key, repo] of Object.entries(CHART_UPSTREAM)) {
    if (name.includes(key)) return githubRelease(repo)
  }
  return undefined
}

function chartGroupTag(group: string): string {
  if (!group) return 'A'
  const g = group.toLowerCase()
  if (g === 'rancher' || g.includes('system')) return 'R'
  if (g === 'cni') return 'C'
  if (g === 'rke2' || g === 'k3s' || g === 'rke1') return g.toUpperCase()
  if (g.includes('load') || g.includes('ingress')) return 'LB'
  return 'A'
}

function chartGroupClass(group: string): string {
  const tag = chartGroupTag(group)
  if (tag === 'R') return 'tag-Rancher'
  if (tag === 'C') return 'tag-CNI'
  if (tag === 'LB') return 'tag-LoadBalancerIngress'
  if (tag === 'RKE2' || tag === 'K3S' || tag === 'RKE1') return 'tag-RKE'
  return 'tag-addons'
}

const availSummary = computed(() => {
  if (!availChecked.value) return null
  const r = availResults.value
  let ok = 0, notFound = 0, noArch = 0, error = 0
  for (const key of Object.keys(r)) {
    const entry = r[key]
    if (!entry) continue
    if (entry.status === 'ok') ok++
    else if (entry.status === 'not_found') notFound++
    else if (entry.status === 'no_arch') noArch++
    else error++
  }
  return { ok, notFound, noArch, error, total: ok + notFound + noArch + error }
})

// Total compressed pull size from the dedicated sizes fetch (not availability).
const totalSelectedSize = computed(() => {
  if (!sizesChecked.value) return 0
  let sum = 0
  for (const img of previewImages.value) {
    const r = sizeResults.value[img]
    if (r?.status === 'ok' && r.sizeBytes) sum += r.sizeBytes
  }
  return sum
})

const sizedCount = computed(() => {
  if (!sizesChecked.value) return 0
  let n = 0
  for (const img of previewImages.value) {
    if (sizeResults.value[img]?.status === 'ok' && sizeResults.value[img]?.sizeBytes) n++
  }
  return n
})

const sizesMissingArch = computed(() => {
  if (!sizesChecked.value) return 0
  let n = 0
  for (const img of previewImages.value) {
    if (sizeResults.value[img]?.status === 'no_arch') n++
  }
  return n
})

function imageLifecycle(ref: string): ComponentLifecycleAnnotation | undefined {
  const chartId = imageChartIdMap.value[ref]
  const meta = chartId ? chartMetaById.value[chartId] : undefined
  if (meta) {
    const fromChart = annotateChart(meta.label, { category: meta.category, catalog: meta.catalog })
    if (fromChart) return fromChart
  }
  const chartLabel = imageChartMap.value[ref]
  if (chartLabel) {
    const fromLabel = annotateChart(chartLabel)
    if (fromLabel) return fromLabel
  }
  const slash = ref.lastIndexOf('/')
  const tail = slash >= 0 ? ref.slice(slash + 1) : ref
  const name = tail.split(':')[0] ?? tail
  return annotateComponent(name) || annotateComponent(ref)
}

const imagesTableColspan = computed(() => {
  let n = 3 // image, tag, lifecycle
  if (availChecked.value) n++
  if (sizesChecked.value) n++
  return n
})

const imageLifecycleByRef = computed(() => {
  const m: Record<string, ComponentLifecycleAnnotation | undefined> = {}
  for (const img of previewImages.value) {
    m[img] = imageLifecycle(img)
  }
  return m
})

const upstreamLifecycleByRef = ref<Record<string, ComponentLifecycleAnnotation>>({})

const effectiveImageLifecycleMap = computed(() => {
  const m: Record<string, ComponentLifecycleAnnotation | undefined> = {}
  for (const img of previewImages.value) {
    m[img] = mergeLifecycleAnnotations(
      imageLifecycleByRef.value[img],
      upstreamLifecycleByRef.value[img],
    )
  }
  return m
})

type ImageSortKey = 'group' | 'name' | 'tag' | 'lifecycle' | 'size'
const imageSortKey = ref<ImageSortKey>('name')
const imageSortDir = ref<'asc' | 'desc'>('asc')
const imagesModalOpen = ref(false)
const imagesModalCloseRef = ref<HTMLButtonElement | null>(null)
const headerHelpOpen = ref(false)
const mirrorModalOpen = ref(false)
const mirrorModalCloseRef = ref<HTMLButtonElement | null>(null)

const compactPastSelection = computed(() => {
  const s = props.pastSelection?.trim() || ''
  if (!s) return ''
  if (s.length <= 64) return s
  return `${s.slice(0, 61)}…`
})

function openMirrorModal() {
  mirrorModalOpen.value = true
  void nextTick(() => mirrorModalCloseRef.value?.focus())
}

function closeMirrorModal() {
  mirrorModalOpen.value = false
}

watch(mirrorModalOpen, (open, _, onCleanup) => {
  if (typeof document === 'undefined') return
  if (!open) return
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') closeMirrorModal()
  }
  document.addEventListener('keydown', onKey)
  onCleanup(() => document.removeEventListener('keydown', onKey))
})

function updateDestinationRegistry(value: string) {
  emit('update:destinationRegistry', value.trim())
}

const GROUP_SORT_ORDER: Record<string, number> = {
  Rancher: 0,
  CNI: 1,
  K3s: 2,
  RKE2: 3,
  RKE1: 4,
  'Load Balancer / Ingress': 5,
  addons: 90,
}

function lifecycleSortWeight(img: string): number {
  const s = effectiveImageLifecycleMap.value[img]?.status
  if (s === 'supported') return 0
  if (s === 'maintenance') return 1
  if (s === 'eol') return 2
  return 3
}

function compareImages(a: string, b: string): number {
  let cmp = 0
  switch (imageSortKey.value) {
    case 'group': {
      const ga = imageSourceGroup.value[a] || 'addons'
      const gb = imageSourceGroup.value[b] || 'addons'
      const oa = GROUP_SORT_ORDER[ga] ?? 50
      const ob = GROUP_SORT_ORDER[gb] ?? 50
      cmp = oa - ob || ga.localeCompare(gb, undefined, { sensitivity: 'base' })
      break
    }
    case 'name':
      cmp = imageShortName(a).localeCompare(imageShortName(b), undefined, { sensitivity: 'base' })
      break
    case 'tag':
      cmp = imageTag(a).localeCompare(imageTag(b), undefined, { numeric: true, sensitivity: 'base' })
      break
    case 'lifecycle':
      cmp = lifecycleSortWeight(a) - lifecycleSortWeight(b)
      break
    case 'size':
      cmp = (sizeResults.value[a]?.sizeBytes ?? -1) - (sizeResults.value[b]?.sizeBytes ?? -1)
      break
  }
  if (cmp === 0) cmp = a.localeCompare(b)
  return imageSortDir.value === 'asc' ? cmp : -cmp
}

const sortedPreviewImages = computed(() => [...previewImages.value].sort(compareImages))

const inlinePreviewImages = computed(() => sortedPreviewImages.value.slice(0, 30))

function setImageSort(key: ImageSortKey) {
  if (key === 'size' && !sizesChecked.value) return
  if (imageSortKey.value === key) {
    imageSortDir.value = imageSortDir.value === 'asc' ? 'desc' : 'asc'
  } else {
    imageSortKey.value = key
    imageSortDir.value = 'asc'
  }
}

function sortIndicator(key: ImageSortKey): string {
  if (imageSortKey.value !== key) return ''
  return imageSortDir.value === 'asc' ? ' ▲' : ' ▼'
}

function openImagesModal() {
  imagesModalOpen.value = true
  void nextTick(() => imagesModalCloseRef.value?.focus())
}

function closeImagesModal() {
  imagesModalOpen.value = false
}

watch(imagesModalOpen, (open, _, onCleanup) => {
  if (typeof document === 'undefined') return
  document.body.style.overflow = open ? 'hidden' : ''
  if (!open) return
  const onKey = (e: KeyboardEvent) => {
    if (e.key === 'Escape') closeImagesModal()
  }
  document.addEventListener('keydown', onKey)
  onCleanup(() => {
    document.removeEventListener('keydown', onKey)
    document.body.style.overflow = ''
  })
})

async function loadUpstreamLifecycle() {
  const imgs = previewImages.value.slice(0, 150)
  const slugs = new Set<string>()
  for (const img of imgs) {
    const slug = eolProductSlug(imageShortName(img), imageChartMap.value[img])
    if (slug) slugs.add(slug)
  }
  if (slugs.size === 0) {
    upstreamLifecycleByRef.value = {}
    return
  }
  await prefetchEolProducts([...slugs])
  const m: Record<string, ComponentLifecycleAnnotation> = {}
  await Promise.all(imgs.map(async (img) => {
    const ann = await upstreamLifecycleForImageRef(img, imageChartMap.value[img], imageShortName(img))
    if (ann) m[img] = ann
  }))
  upstreamLifecycleByRef.value = m
}

watch(previewImages, () => {
  loadUpstreamLifecycle()
}, { immediate: true })

function rowLifecycle(row: Row): ComponentLifecycleAnnotation | undefined {
  const n = row.node
  if (n.kind === 'chart') {
    const meta = chartMetaById.value[n.id]
    return annotateChart(chartDisplayName(n, n.id), {
      category: meta?.category || n.category || chartCategory(n) || undefined,
      catalog: meta?.catalog,
    })
  }
  if (n.kind === 'image') return imageLifecycle(n.label)
  if (n.kind === 'component') {
    if (n.id.startsWith('addon_')) {
      return annotateComponent(n.id.slice('addon_'.length)) || annotateComponent(n.label)
    }
    if (n.id === 'source_partner_charts') return annotateComponent('source_partner_charts')
    if (n.id === 'source_ui_plugin_charts') return annotateComponent('source_ui_plugin_charts')
    if (n.id === 'addons') return annotateComponent('addons')
    const base = n.label.replace(/\s*\(.*/, '').trim()
    return annotateComponent(base) || annotateComponent(n.id)
  }
  return annotateComponent(n.id) || annotateComponent(n.label)
}

function chartLifecycle(chartId: string): ComponentLifecycleAnnotation | undefined {
  const meta = chartMetaById.value[chartId]
  const label = meta?.label || chartInfoMap.value[chartId]?.label || chartId
  return annotateChart(label, {
    category: meta?.category || chartInfoMap.value[chartId]?.category,
    catalog: meta?.catalog,
  })
}

// Release Notes
const releaseNotesOpen = ref(false)
const releaseNotes = ref<Record<string, ReleaseInfo>>({})
const releaseLoading = ref(false)
const releaseError = ref('')

const releaseVersions = computed(() => {
  const items: { repo: string; tag: string; label: string }[] = []
  for (const v of props.rancherVersions ?? []) {
    if (v) items.push({ repo: 'rancher/rancher', tag: v, label: `Rancher ${v}` })
  }
  for (const v of props.rke2Versions) {
    if (v && v !== 'all') items.push({ repo: 'rancher/rke2', tag: v, label: `RKE2 ${v}` })
  }
  for (const v of props.k3sVersions) {
    if (v && v !== 'all') items.push({ repo: 'rancher/k3s', tag: v, label: `K3s ${v}` })
  }
  return items
})

const releaseLifecycle = computed(() => {
  const m: Record<string, VersionAnnotation> = {}
  for (const v of releaseVersions.value) {
    let ann: VersionAnnotation | undefined
    if (v.repo === 'rancher/rancher') {
      ann = annotateRancherVersions([{ version: v.tag }])[0]
    } else {
      ann = annotateDistroVersions([v.tag])[0]
    }
    if (ann) m[v.label] = ann
  }
  return m
})

// rke2-patcher guidance is relevant for Rancher Prime RKE2 clusters.
const hasPartnerChartsInTree = computed(() =>
  props.roots.some((r) => r.id === 'source_partner_charts'),
)
const hasUiPluginChartsInTree = computed(() =>
  props.roots.some((r) => r.id === 'source_ui_plugin_charts'),
)

const primePatchAvailable = computed(() => !!props.isPrime && props.distros.includes('rke2'))
const patcherOpen = ref(false)

watch(() => props.arch, () => {
  availChecked.value = false
  availResults.value = {}
  sizesChecked.value = false
  sizeResults.value = {}
})

async function loadReleaseNotes() {
  if (releaseVersions.value.length === 0) return
  releaseLoading.value = true
  releaseError.value = ''
  releaseNotes.value = {}
  const results: Record<string, ReleaseInfo> = {}
  for (const v of releaseVersions.value) {
    try {
      results[v.label] = await fetchReleaseNotes(v.repo, v.tag)
    } catch (e) {
      results[v.label] = { tag: v.tag, name: v.label, publishedAt: '', url: '', prerelease: false, charts: [], changelog: [e instanceof Error ? e.message : String(e)] }
    }
  }
  releaseNotes.value = results
  releaseLoading.value = false
}

function toggleReleaseNotes() {
  releaseNotesOpen.value = !releaseNotesOpen.value
  if (releaseNotesOpen.value && Object.keys(releaseNotes.value).length === 0) {
    loadReleaseNotes()
  }
}
</script>

<template>
  <div class="step3">
    <div class="step3-header">
      <div class="step3-header-top">
        <div class="step3-header-left">
          <div class="step3-title-row">
            <h2 class="step-title">Step 3: Groups &amp; charts</h2>
            <span v-if="pastSelection" class="selection-chip" :title="pastSelection">{{ compactPastSelection }}</span>
            <button type="button" class="btn btn-hints-toggle" @click="headerHelpOpen = !headerHelpOpen">
              {{ headerHelpOpen ? 'Hide hints' : 'Hints' }}
            </button>
          </div>
        </div>
        <div class="step3-header-actions">
          <button type="button" class="btn btn-secondary" @click="emit('back')">Back</button>
          <button
            type="button"
            class="btn btn-check"
            :disabled="availLoading || previewImages.length === 0"
            @click="doCheckAvailability"
          >
            {{ availLoading ? 'Checking…' : 'Check Availability' }}
          </button>
          <button
            type="button"
            class="btn btn-sizes"
            :disabled="sizesLoading || previewImages.length === 0"
            @click="doFetchImageSizes"
          >
            {{ sizesLoading ? 'Fetching sizes…' : 'Show Image Sizes' }}
          </button>
          <button
            type="button"
            class="btn btn-scan"
            :disabled="scanButtonDisabled"
            @click="doScan"
          >
            {{ scanButtonLabel }}
          </button>
          <button type="button" class="btn btn-secondary" @click="openMirrorModal">Mirror</button>
          <button type="button" class="btn btn-primary" @click="doExport">Export image list</button>
        </div>
      </div>
      <div v-if="headerHelpOpen" class="header-help-panel">
        <ul class="header-help-list">
          <li><strong>Essentials</strong> — Rancher core, cert-manager, CNI, distro, load balancer</li>
          <li><strong>AddOns</strong> — monitoring, logging, backup, storage, etc.</li>
          <li><strong>Lifecycle</strong> — best-effort (SUSE catalog / upstream)</li>
          <li>Availability and compressed sizes are separate actions above</li>
        </ul>
        <p v-if="!includeUIPluginCharts" class="opt-in-hint opt-in-hint-compact">
          UI Plugins off — enable <em>Include UI Plugins</em> in Step 1 (separate group, not AddOns).
        </p>
        <p v-else-if="includeUIPluginCharts && !hasUiPluginChartsInTree" class="opt-in-hint opt-in-warn opt-in-hint-compact">
          UI Plugins requested but none found for this Rancher version.
        </p>
        <p v-else-if="includeUIPluginCharts && hasUiPluginChartsInTree" class="opt-in-hint opt-in-hint-compact">
          UI Plugins: top-level <strong>UI Plugins</strong> group in the tree (scroll if needed).
        </p>
        <p v-if="!includePartnerCharts" class="opt-in-hint opt-in-hint-compact">
          Partner Charts off — enable <em>Include Partner Charts</em> in Step 1.
        </p>
        <p v-else-if="includePartnerCharts && !hasPartnerChartsInTree" class="opt-in-hint opt-in-warn opt-in-hint-compact">
          Partner Charts requested but none found for this Rancher version.
        </p>
      </div>
      <div v-if="scanInProgress" class="scan-progress-wrap">
        <LoadingProgressBar :percent="scanProgress.percent" :phase="scanProgress.phase" />
      </div>
      <div v-if="scanStatus?.status === 'running'" class="scan-summary scan-ongoing">
        <span class="scan-done">Ongoing scan…</span> <span class="scan-hint">Scan selected images is disabled until the current scan finishes.</span>
      </div>
      <div v-else-if="scanStatus?.status === 'completed'" class="scan-summary">
        <span class="scan-done">Scan complete.</span>
        <template v-if="scanStatus.summary">
          <span v-if="scanStatus.summary.critical" class="scan-sev critical">{{ scanStatus.summary.critical }} critical</span>
          <span v-if="scanStatus.summary.high" class="scan-sev high">{{ scanStatus.summary.high }} high</span>
          <span v-if="scanStatus.summary.medium" class="scan-sev medium">{{ scanStatus.summary.medium }} medium</span>
          <span v-if="scanStatus.summary.low" class="scan-sev low">{{ scanStatus.summary.low }} low</span>
        </template>
        <button type="button" class="btn btn-download-report" @click="doDownloadScanReport">Download report (CSV)</button>
      </div>
      <p v-if="scanStatus?.status === 'failed' || scanError" class="error-msg">{{ scanStatus?.error || scanError }}</p>
      <div class="scan-logs-row">
        <ServerLogsPanel
          :entries="formattedScanLogs"
          title="Scan server activity"
          empty-text="Waiting for scan activity…"
          :collapsible="!scanInProgress"
          :default-open="scanInProgress"
        />
      </div>
      <div class="scan-trivy-hint scan-trivy-hint-compact">
        <strong>Scan:</strong> Trivy vulnerability scan — first run may download DB. CLI: <code>hangar scan -f images.txt -r scan-report.csv</code>
      </div>
      <div v-if="availChecked && availSummary" class="avail-summary">
        <span class="avail-ok">{{ availSummary.ok }}/{{ availSummary.total }} available for linux/{{ targetArch }}</span>
        <span v-if="availSummary.noArch > 0" class="avail-noarch">{{ availSummary.noArch }} missing linux/{{ targetArch }}</span>
        <span v-if="availSummary.notFound > 0" class="avail-fail">{{ availSummary.notFound }} not found</span>
        <span v-if="availSummary.error > 0" class="avail-err">{{ availSummary.error }} errors</span>
      </div>
      <div v-if="sizesChecked" class="size-summary">
        <span v-if="totalSelectedSize > 0" class="avail-total" :title="`Sum of compressed pull sizes for ${sizedCount} of ${previewImages.length} selected images (linux/${targetArch})`">
          Total selected: ~{{ formatBytes(totalSelectedSize) }}<span class="avail-total-sub"> ({{ sizedCount }}/{{ previewImages.length }} sized, linux/{{ targetArch }}, compressed)</span>
        </span>
        <span v-if="sizesMissingArch > 0" class="avail-noarch">{{ sizesMissingArch }} missing linux/{{ targetArch }}</span>
      </div>
      <p v-if="availError" class="error-msg">{{ availError }}</p>
      <p v-if="sizesError" class="error-msg">{{ sizesError }}</p>
    </div>

    <div class="tree-layout-wrapper">
      <div class="mobile-tabs" role="tablist" aria-label="Step 3 views">
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'groups' }"
          :aria-selected="mobileTab === 'groups'"
          @click="mobileTab = 'groups'"
        >
          <span class="mobile-tab-label">Groups</span>
        </button>
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'charts' }"
          :aria-selected="mobileTab === 'charts'"
          @click="mobileTab = 'charts'"
        >
          <span class="mobile-tab-label">Charts</span>
          <span class="mobile-tab-count">({{ previewCharts.length }})</span>
        </button>
        <button
          type="button"
          role="tab"
          class="mobile-tab"
          :class="{ active: mobileTab === 'images' }"
          :aria-selected="mobileTab === 'images'"
          @click="mobileTab = 'images'"
        >
          <span class="mobile-tab-label">Images</span>
          <span class="mobile-tab-count">({{ previewImages.length }})</span>
        </button>
      </div>
    <div class="tree-layout">
      <div class="col col-tree" :class="{ 'mobile-panel-active': mobileTab === 'groups' }">
        <h3 class="col-title">Groups</h3>
        <div class="tree">
          <div
            v-for="row in visibleRows"
            :key="row.node.id"
            class="tree-row"
            :class="['tree-row-' + row.node.kind, { 'tree-row-unselected': !selected[row.node.id] }]"
            :style="{ paddingLeft: row.depth * 12 + 8 + 'px' }"
            :title="row.node.kind === 'image' ? row.node.label : row.node.kind === 'chart' ? nodeTooltip(row.node) : nodeTooltip(row.node)"
          >
            <span
              v-if="isExpandableNode(row.node)"
              class="expand"
              @click="toggleExpand(row.node.id)"
            >
              {{ expanded[row.node.id] ? '▼' : '▶' }}
            </span>
            <span v-else class="expand-placeholder"></span>
            <label class="row-label">
              <input
                type="checkbox"
                :checked="!!selected[row.node.id]"
                @change="onSelectChange(row, $event)"
                @click.stop
              />
              <span v-if="KIND_LABELS[row.node.kind] && row.node.kind !== 'image' && row.node.kind !== 'chart'" class="kind-badge">{{ KIND_LABELS[row.node.kind] }}</span>
              <span
                v-if="resolveRowIcons(row.node).primary || resolveRowIcons(row.node).secondary"
                class="chart-icon-stack"
              >
                <img
                  v-if="resolveRowIcons(row.node).primary"
                  class="chart-icon"
                  :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveRowIcons(row.node).primary) }"
                  :src="resolveRowIcons(row.node).primary"
                  :alt="row.node.kind === 'image' ? imageChartMap[row.node.label] || '' : ''"
                  :title="resolveRowIcons(row.node).primaryTitle || resolveRowIcons(row.node).primary"
                  loading="lazy"
                  @error="hideBrokenIcon"
                />
                <img
                  v-if="resolveRowIcons(row.node).secondary"
                  class="chart-icon chart-icon-secondary"
                  :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveRowIcons(row.node).secondary) }"
                  :src="resolveRowIcons(row.node).secondary"
                  :alt="resolveRowIcons(row.node).secondaryTitle || ''"
                  :title="resolveRowIcons(row.node).secondaryTitle || resolveRowIcons(row.node).secondary"
                  loading="lazy"
                  @error="hideBrokenSecondaryIcon"
                />
              </span>
              <span v-if="row.node.kind === 'image'" class="label-text img-tree-name">{{ imageShortName(row.node.label) }}</span>
              <span v-else-if="row.node.kind === 'chart'" class="label-text">{{ chartDisplayName(row.node, row.node.id) }}</span>
              <span v-else class="label-text">{{ row.node.label }}</span>
              <template v-if="rowLifecycle(row) && row.node.kind !== 'image' && row.node.kind !== 'chart'">
                <span
                  v-if="rowLifecycle(row)!.status === 'maintenance'"
                  class="lifecycle-badge badge-eom"
                  :title="componentLifecycleTitle(rowLifecycle(row)!)"
                >{{ lifecycleStatusLabel(rowLifecycle(row)!.status) }}</span>
                <span
                  v-else-if="rowLifecycle(row)!.status === 'eol'"
                  class="lifecycle-badge badge-eol"
                  :title="componentLifecycleTitle(rowLifecycle(row)!)"
                >{{ lifecycleStatusLabel(rowLifecycle(row)!.status) }}</span>
                <span
                  v-else-if="rowLifecycle(row)!.status === 'supported'"
                  class="lifecycle-badge badge-current"
                  :title="componentLifecycleTitle(rowLifecycle(row)!)"
                >Supported</span>
              </template>
              <span v-if="row.node.count > 0 && row.node.kind !== 'image' && row.node.kind !== 'chart'" class="count">({{ row.node.count }})</span>
            </label>
          </div>
        </div>
      </div>
      <div class="col col-preview" :class="{ 'mobile-panel-active': mobileTab === 'charts' }">
        <h3 class="col-title">Charts ({{ previewCharts.length }})</h3>
        <div class="legend">
          <span class="legend-item tag-Rancher">[R] Rancher</span>
          <span class="legend-item tag-CNI">[C] CNI</span>
          <span class="legend-item tag-RKE">[RKE2] RKE2</span>
          <span class="legend-item tag-LoadBalancerIngress">[LB] Ingress</span>
          <span class="legend-item tag-addons">[A] AddOn</span>
        </div>
        <ul class="preview-list">
          <li
            v-for="c in previewCharts.slice(0, 80)"
            :key="c"
            class="preview-item chart-preview-item"
            :title="chartInfoMap[c]?.description || ''"
          >
            <span v-if="chartInfoMap[c]" class="img-tag" :class="chartGroupClass(chartInfoMap[c].group)">[{{ chartGroupTag(chartInfoMap[c].group) }}]</span>
            <span class="chart-name">{{ chartInfoMap[c]?.label || c }}</span>
            <template v-if="chartLifecycle(c)">
              <span
                v-if="chartLifecycle(c)!.status === 'maintenance'"
                class="lifecycle-badge badge-eom"
                :title="componentLifecycleTitle(chartLifecycle(c)!)"
              >{{ lifecycleStatusLabel(chartLifecycle(c)!.status) }}</span>
              <span
                v-else-if="chartLifecycle(c)!.status === 'eol'"
                class="lifecycle-badge badge-eol"
                :title="componentLifecycleTitle(chartLifecycle(c)!)"
              >{{ lifecycleStatusLabel(chartLifecycle(c)!.status) }}</span>
              <span
                v-else-if="chartLifecycle(c)!.status === 'supported'"
                class="lifecycle-badge badge-current"
                :title="componentLifecycleTitle(chartLifecycle(c)!)"
              >Supported</span>
            </template>
            <span v-if="chartInfoMap[c]?.version" class="version-badge">{{ chartInfoMap[c].version }}</span>
            <span v-if="chartInfoMap[c]?.category" class="category-badge">{{ chartInfoMap[c].category }}</span>
            <a
              v-if="chartReleaseUrl(c, chartInfoMap[c]?.label || c)"
              :href="chartReleaseUrl(c, chartInfoMap[c]?.label || c)"
              target="_blank"
              rel="noopener noreferrer"
              class="chart-release-link"
              title="Upstream releases"
              @click.stop
            >↗</a>
          </li>
          <li v-if="previewCharts.length > 80" class="preview-more">… and {{ previewCharts.length - 80 }} more</li>
        </ul>
      </div>
      <div class="col col-preview" :class="{ 'mobile-panel-active': mobileTab === 'images' }">
        <div class="col-title-row">
          <h3 class="col-title">Images ({{ previewImages.length }})</h3>
          <button
            type="button"
            class="btn btn-images-modal"
            title="Open full image list in a popup"
            @click="openImagesModal"
          >
            Full list ↗
          </button>
        </div>
        <div class="images-sort-bar">
          <span class="images-sort-label">Sort</span>
          <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'group' }" @click="setImageSort('group')">Group{{ sortIndicator('group') }}</button>
          <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'name' }" @click="setImageSort('name')">Name{{ sortIndicator('name') }}</button>
          <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'tag' }" @click="setImageSort('tag')">Tag{{ sortIndicator('tag') }}</button>
          <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'lifecycle' }" @click="setImageSort('lifecycle')">Lifecycle{{ sortIndicator('lifecycle') }}</button>
          <button v-if="sizesChecked" type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'size' }" @click="setImageSort('size')">Size{{ sortIndicator('size') }}</button>
        </div>
        <div class="legend">
          <span class="legend-item tag-Rancher">[R] Rancher</span>
          <span class="legend-item tag-CNI">[C] CNI</span>
          <span class="legend-item tag-RKE">[RKE2] RKE2</span>
          <span class="legend-item tag-LoadBalancerIngress">[LB] Ingress</span>
          <span class="legend-item tag-addons">[A] AddOn</span>
        </div>
        <div class="images-table-wrap">
          <table class="images-table">
            <thead>
              <tr>
                <th v-if="availChecked" class="col-status" title="Registry availability">✓</th>
                <th
                  class="col-image sortable-th"
                  :class="{ 'sort-active': imageSortKey === 'name' }"
                  title="Sort by image name"
                  @click="setImageSort('name')"
                >Image<span class="sort-indicator">{{ sortIndicator('name') }}</span></th>
                <th
                  class="col-tag sortable-th"
                  :class="{ 'sort-active': imageSortKey === 'tag' }"
                  title="Sort by tag"
                  @click="setImageSort('tag')"
                >Tag<span class="sort-indicator">{{ sortIndicator('tag') }}</span></th>
                <th
                  class="col-life sortable-th"
                  :class="{ 'sort-active': imageSortKey === 'lifecycle' }"
                  title="Sort by lifecycle status"
                  @click="setImageSort('lifecycle')"
                >Lifecycle<span class="sort-indicator">{{ sortIndicator('lifecycle') }}</span></th>
                <th
                  v-if="sizesChecked"
                  class="col-size sortable-th"
                  :class="{ 'sort-active': imageSortKey === 'size' }"
                  title="Sort by compressed size"
                  @click="setImageSort('size')"
                >Size<span class="sort-indicator">{{ sortIndicator('size') }}</span></th>
              </tr>
            </thead>
            <tbody>
              <template v-for="img in inlinePreviewImages" :key="img">
                <tr
                  class="images-table-row"
                  :class="{ 'is-expanded': expandedImages[img] }"
                  :title="imageTooltip(img)"
                  @click="toggleImageExpand(img)"
                >
                  <td v-if="availChecked" class="col-status" @click.stop>
                    <span v-if="availResults[img]" class="avail-dot" :class="{
                      'dot-ok': availResults[img]?.status === 'ok',
                      'dot-fail': availResults[img]?.status === 'not_found' || availResults[img]?.status === 'error',
                      'dot-noarch': availResults[img]?.status === 'no_arch',
                    }">{{ availResults[img]?.status === 'ok' ? '\u2713' : availResults[img]?.status === 'no_arch' ? '!' : '\u2717' }}</span>
                  </td>
                  <td class="col-image">
                    <div class="img-cell-inner">
                      <span class="img-expand">{{ expandedImages[img] ? '▼' : '▶' }}</span>
                      <span
                        v-if="resolveImageRowIcons(img).primary || resolveImageRowIcons(img).secondary"
                        class="chart-icon-stack img-row-icon-stack"
                      >
                        <img
                          v-if="resolveImageRowIcons(img).primary"
                          class="chart-icon img-row-icon"
                          :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveImageRowIcons(img).primary) }"
                          :src="resolveImageRowIcons(img).primary"
                          :alt="imageChartMap[img] || ''"
                          :title="resolveImageRowIcons(img).primaryTitle || resolveImageRowIcons(img).primary"
                          loading="lazy"
                          @error="hideBrokenIcon"
                        />
                        <img
                          v-if="resolveImageRowIcons(img).secondary"
                          class="chart-icon img-row-icon chart-icon-secondary"
                          :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveImageRowIcons(img).secondary) }"
                          :src="resolveImageRowIcons(img).secondary"
                          :alt="resolveImageRowIcons(img).secondaryTitle || ''"
                          :title="resolveImageRowIcons(img).secondaryTitle || resolveImageRowIcons(img).secondary"
                          loading="lazy"
                          @error="hideBrokenSecondaryIcon"
                        />
                      </span>
                      <div class="img-name-stack">
                        <span class="img-name" :title="imageChartMap[img] ? `${imageShortName(img)} (${imageChartMap[img]})` : img">{{ imageShortName(img) }}</span>
                        <span v-if="imageRepoPath(img)" class="img-repo-path" :title="imageRepoPath(img)">{{ imageRepoPath(img) }}</span>
                      </div>
                    </div>
                  </td>
                  <td class="col-tag"><span v-if="imageTag(img)" class="tag-badge">{{ imageTag(img) }}</span></td>
                  <td class="col-life">
                    <template v-if="effectiveImageLifecycleMap[img]">
                      <span
                        v-if="effectiveImageLifecycleMap[img]!.status === 'maintenance'"
                        class="lifecycle-badge badge-eom"
                        :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                      >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                      <span
                        v-else-if="effectiveImageLifecycleMap[img]!.status === 'eol'"
                        class="lifecycle-badge badge-eol"
                        :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                      >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                      <span
                        v-else-if="effectiveImageLifecycleMap[img]!.status === 'supported'"
                        class="lifecycle-badge badge-current"
                        :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                      >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                    </template>
                  </td>
                  <td v-if="sizesChecked" class="col-size">
                    <span v-if="sizeResults[img]?.sizeBytes" class="size-badge">{{ formatBytes(sizeResults[img]!.sizeBytes!) }}</span>
                  </td>
                </tr>
                <tr v-if="expandedImages[img]" class="images-table-detail-row">
                  <td :colspan="imagesTableColspan" class="images-table-detail">
                    <div class="img-detail-grid">
                      <div class="img-detail-item">
                        <span class="img-detail-label">Full ref</span>
                        <code class="img-detail-value">{{ img }}</code>
                      </div>
                      <div class="img-detail-item">
                        <span class="img-detail-label">Registry</span>
                        <span class="img-detail-value">{{ imageRegistryHost(img) }}</span>
                      </div>
                      <div class="img-detail-item">
                        <span class="img-detail-label">Repository</span>
                        <span class="img-detail-value">{{ imageRepoPath(img) }}</span>
                      </div>
                      <div v-if="imageSourceGroup[img]" class="img-detail-item">
                        <span class="img-detail-label">Source group</span>
                        <span class="img-detail-value">{{ imageSourceGroup[img] }}</span>
                      </div>
                      <div v-if="imageChartMap[img]" class="img-detail-item">
                        <span class="img-detail-label">Chart</span>
                        <span class="img-detail-value">{{ imageChartMap[img] }}</span>
                      </div>
                      <div v-if="resolveUpstreamProject(img, imageChartMap[img], imageShortName(img), imageRepoPath(img))" class="img-detail-item">
                        <span class="img-detail-label">Upstream OSS</span>
                        <span class="img-detail-value">{{ imageUpstreamName(img) }} · {{ resolveUpstreamProject(img, imageChartMap[img], imageShortName(img), imageRepoPath(img))!.repo }}</span>
                      </div>
                      <div v-if="effectiveImageLifecycleMap[img]?.eol" class="img-detail-item">
                        <span class="img-detail-label">Upstream EOL</span>
                        <span class="img-detail-value">{{ effectiveImageLifecycleMap[img]!.eol }}</span>
                      </div>
                    </div>
                    <div class="img-detail-links">
                      <a
                        v-if="imageUpstreamReleasesLink(img)"
                        :href="imageUpstreamReleasesLink(img)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="img-detail-link"
                        @click.stop
                      >{{ imageUpstreamName(img) || 'Upstream' }} releases ↗</a>
                      <a
                        v-if="imageUpstreamDocsLink(img)"
                        :href="imageUpstreamDocsLink(img)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="img-detail-link"
                        @click.stop
                      >{{ imageUpstreamName(img) || 'Upstream' }} docs ↗</a>
                      <a
                        v-if="imageRegistryUrl(img)"
                        :href="imageRegistryUrl(img)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="img-detail-link"
                        @click.stop
                      >Registry ↗</a>
                      <a
                        v-if="eolProductSlug(imageShortName(img), imageChartMap[img])"
                        :href="eolProductPageUrl(eolProductSlug(imageShortName(img), imageChartMap[img])!)"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="img-detail-link"
                        @click.stop
                      >Upstream lifecycle ↗</a>
                      <a
                        v-if="imageChartIdMap[img] && chartReleaseUrl(imageChartIdMap[img], imageChartMap[img] || '')"
                        :href="chartReleaseUrl(imageChartIdMap[img], imageChartMap[img] || '')"
                        target="_blank"
                        rel="noopener noreferrer"
                        class="img-detail-link"
                        @click.stop
                      >Chart releases ↗</a>
                    </div>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
          <p v-if="previewImages.length > 30" class="preview-more preview-more-popup-hint">
            Showing {{ inlinePreviewImages.length }} of {{ previewImages.length }} here —
            <button type="button" class="link-btn" @click="openImagesModal">open full list popup</button>
          </p>
        </div>
      </div>
    </div>
    </div>

    <Teleport to="body">
      <div
        v-if="imagesModalOpen"
        class="images-modal-backdrop"
        role="presentation"
        @click.self="closeImagesModal"
      >
        <div class="images-modal" role="dialog" aria-modal="true" aria-labelledby="images-modal-title" @click.stop>
          <div class="images-modal-header">
            <h3 id="images-modal-title" class="images-modal-title">Images ({{ previewImages.length }})</h3>
            <div class="images-modal-actions">
              <div class="images-sort-bar images-sort-bar-modal">
                <span class="images-sort-label">Sort</span>
                <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'group' }" @click="setImageSort('group')">Group{{ sortIndicator('group') }}</button>
                <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'name' }" @click="setImageSort('name')">Name{{ sortIndicator('name') }}</button>
                <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'tag' }" @click="setImageSort('tag')">Tag{{ sortIndicator('tag') }}</button>
                <button type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'lifecycle' }" @click="setImageSort('lifecycle')">Lifecycle{{ sortIndicator('lifecycle') }}</button>
                <button v-if="sizesChecked" type="button" class="images-sort-btn" :class="{ active: imageSortKey === 'size' }" @click="setImageSort('size')">Size{{ sortIndicator('size') }}</button>
              </div>
              <button type="button" ref="imagesModalCloseRef" class="btn btn-modal-close" aria-label="Close" @click="closeImagesModal">✕</button>
            </div>
          </div>
          <div class="images-modal-body">
            <table class="images-table images-table-modal">
              <thead>
                <tr>
                  <th v-if="availChecked" class="col-status" title="Registry availability">✓</th>
                  <th
                    class="col-image sortable-th"
                    :class="{ 'sort-active': imageSortKey === 'name' }"
                    title="Sort by image name"
                    @click="setImageSort('name')"
                  >Image<span class="sort-indicator">{{ sortIndicator('name') }}</span></th>
                  <th
                    class="col-tag sortable-th"
                    :class="{ 'sort-active': imageSortKey === 'tag' }"
                    title="Sort by tag"
                    @click="setImageSort('tag')"
                  >Tag<span class="sort-indicator">{{ sortIndicator('tag') }}</span></th>
                  <th
                    class="col-life sortable-th"
                    :class="{ 'sort-active': imageSortKey === 'lifecycle' }"
                    title="Sort by lifecycle status"
                    @click="setImageSort('lifecycle')"
                  >Lifecycle<span class="sort-indicator">{{ sortIndicator('lifecycle') }}</span></th>
                  <th
                    v-if="sizesChecked"
                    class="col-size sortable-th"
                    :class="{ 'sort-active': imageSortKey === 'size' }"
                    title="Sort by compressed size"
                    @click="setImageSort('size')"
                  >Size<span class="sort-indicator">{{ sortIndicator('size') }}</span></th>
                </tr>
              </thead>
              <tbody>
                <template v-for="img in sortedPreviewImages" :key="'modal-' + img">
                  <tr
                    class="images-table-row"
                    :class="{ 'is-expanded': expandedImages[img] }"
                    :title="imageTooltip(img)"
                    @click="toggleImageExpand(img)"
                  >
                    <td v-if="availChecked" class="col-status" @click.stop>
                      <span v-if="availResults[img]" class="avail-dot" :class="{
                        'dot-ok': availResults[img]?.status === 'ok',
                        'dot-fail': availResults[img]?.status === 'not_found' || availResults[img]?.status === 'error',
                        'dot-noarch': availResults[img]?.status === 'no_arch',
                      }">{{ availResults[img]?.status === 'ok' ? '\u2713' : availResults[img]?.status === 'no_arch' ? '!' : '\u2717' }}</span>
                    </td>
                    <td class="col-image">
                      <div class="img-cell-inner">
                        <span class="img-expand">{{ expandedImages[img] ? '▼' : '▶' }}</span>
                        <span
                          v-if="resolveImageRowIcons(img).primary || resolveImageRowIcons(img).secondary"
                          class="chart-icon-stack img-row-icon-stack"
                        >
                          <img
                            v-if="resolveImageRowIcons(img).primary"
                            class="chart-icon img-row-icon"
                            :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveImageRowIcons(img).primary) }"
                            :src="resolveImageRowIcons(img).primary"
                            :alt="imageChartMap[img] || ''"
                            :title="resolveImageRowIcons(img).primaryTitle || resolveImageRowIcons(img).primary"
                            loading="lazy"
                            @error="hideBrokenIcon"
                          />
                          <img
                            v-if="resolveImageRowIcons(img).secondary"
                            class="chart-icon img-row-icon chart-icon-secondary"
                            :class="{ 'rancher-cow-icon': isRancherCowIcon(resolveImageRowIcons(img).secondary) }"
                            :src="resolveImageRowIcons(img).secondary"
                            :alt="resolveImageRowIcons(img).secondaryTitle || ''"
                            :title="resolveImageRowIcons(img).secondaryTitle || resolveImageRowIcons(img).secondary"
                            loading="lazy"
                            @error="hideBrokenSecondaryIcon"
                          />
                        </span>
                        <div class="img-name-stack">
                          <span class="img-name" :title="imageChartMap[img] ? `${imageShortName(img)} (${imageChartMap[img]})` : img">{{ imageShortName(img) }}</span>
                          <span v-if="imageRepoPath(img)" class="img-repo-path" :title="imageRepoPath(img)">{{ imageRepoPath(img) }}</span>
                        </div>
                      </div>
                    </td>
                    <td class="col-tag"><span v-if="imageTag(img)" class="tag-badge">{{ imageTag(img) }}</span></td>
                    <td class="col-life">
                      <template v-if="effectiveImageLifecycleMap[img]">
                        <span
                          v-if="effectiveImageLifecycleMap[img]!.status === 'maintenance'"
                          class="lifecycle-badge badge-eom"
                          :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                        >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                        <span
                          v-else-if="effectiveImageLifecycleMap[img]!.status === 'eol'"
                          class="lifecycle-badge badge-eol"
                          :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                        >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                        <span
                          v-else-if="effectiveImageLifecycleMap[img]!.status === 'supported'"
                          class="lifecycle-badge badge-current"
                          :title="componentLifecycleTitle(effectiveImageLifecycleMap[img]!)"
                        >{{ lifecycleBadgeLabel(effectiveImageLifecycleMap[img]!.status) }}</span>
                      </template>
                    </td>
                    <td v-if="sizesChecked" class="col-size">
                      <span v-if="sizeResults[img]?.sizeBytes" class="size-badge">{{ formatBytes(sizeResults[img]!.sizeBytes!) }}</span>
                    </td>
                  </tr>
                  <tr v-if="expandedImages[img]" class="images-table-detail-row">
                    <td :colspan="imagesTableColspan" class="images-table-detail">
                      <div class="img-detail-grid">
                        <div class="img-detail-item">
                          <span class="img-detail-label">Full ref</span>
                          <code class="img-detail-value">{{ img }}</code>
                        </div>
                        <div class="img-detail-item">
                          <span class="img-detail-label">Registry</span>
                          <span class="img-detail-value">{{ imageRegistryHost(img) }}</span>
                        </div>
                        <div class="img-detail-item">
                          <span class="img-detail-label">Repository</span>
                          <span class="img-detail-value">{{ imageRepoPath(img) }}</span>
                        </div>
                        <div v-if="imageSourceGroup[img]" class="img-detail-item">
                          <span class="img-detail-label">Source group</span>
                          <span class="img-detail-value">{{ imageSourceGroup[img] }}</span>
                        </div>
                        <div v-if="imageChartMap[img]" class="img-detail-item">
                          <span class="img-detail-label">Chart</span>
                          <span class="img-detail-value">{{ imageChartMap[img] }}</span>
                        </div>
                        <div v-if="resolveUpstreamProject(img, imageChartMap[img], imageShortName(img), imageRepoPath(img))" class="img-detail-item">
                          <span class="img-detail-label">Upstream OSS</span>
                          <span class="img-detail-value">{{ imageUpstreamName(img) }} · {{ resolveUpstreamProject(img, imageChartMap[img], imageShortName(img), imageRepoPath(img))!.repo }}</span>
                        </div>
                        <div v-if="effectiveImageLifecycleMap[img]?.eol" class="img-detail-item">
                          <span class="img-detail-label">Upstream EOL</span>
                          <span class="img-detail-value">{{ effectiveImageLifecycleMap[img]!.eol }}</span>
                        </div>
                      </div>
                      <div class="img-detail-links">
                        <a
                          v-if="imageUpstreamReleasesLink(img)"
                          :href="imageUpstreamReleasesLink(img)"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="img-detail-link"
                          @click.stop
                        >{{ imageUpstreamName(img) || 'Upstream' }} releases ↗</a>
                        <a
                          v-if="imageUpstreamDocsLink(img)"
                          :href="imageUpstreamDocsLink(img)"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="img-detail-link"
                          @click.stop
                        >{{ imageUpstreamName(img) || 'Upstream' }} docs ↗</a>
                        <a
                          v-if="imageRegistryUrl(img)"
                          :href="imageRegistryUrl(img)"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="img-detail-link"
                          @click.stop
                        >Registry ↗</a>
                        <a
                          v-if="eolProductSlug(imageShortName(img), imageChartMap[img])"
                          :href="eolProductPageUrl(eolProductSlug(imageShortName(img), imageChartMap[img])!)"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="img-detail-link"
                          @click.stop
                        >Upstream lifecycle ↗</a>
                        <a
                          v-if="imageChartIdMap[img] && chartReleaseUrl(imageChartIdMap[img], imageChartMap[img] || '')"
                          :href="chartReleaseUrl(imageChartIdMap[img], imageChartMap[img] || '')"
                          target="_blank"
                          rel="noopener noreferrer"
                          class="img-detail-link"
                          @click.stop
                        >Chart releases ↗</a>
                      </div>
                    </td>
                  </tr>
                </template>
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </Teleport>

    <Teleport to="body">
      <div
        v-if="mirrorModalOpen"
        class="images-modal-backdrop mirror-modal-backdrop"
        @click.self="closeMirrorModal"
      >
        <div class="images-modal mirror-modal" role="dialog" aria-modal="true" aria-labelledby="mirror-modal-title" @click.stop>
          <div class="images-modal-header">
            <h3 id="mirror-modal-title" class="images-modal-title">Mirror to registry</h3>
            <button ref="mirrorModalCloseRef" type="button" class="btn btn-modal-close" aria-label="Close" @click="closeMirrorModal">×</button>
          </div>
          <div class="images-modal-body mirror-modal-body">
            <label class="dest-registry-label" for="mirror-dest-registry">Destination registry</label>
            <input
              id="mirror-dest-registry"
              type="text"
              class="dest-registry-input dest-registry-input-full"
              :value="props.destinationRegistry ?? ''"
              placeholder="e.g. my-registry.example.com"
              @input="updateDestinationRegistry(($event.target as HTMLInputElement)?.value ?? '')"
            />
            <p class="mirror-modal-desc">
              After exporting <code>images.txt</code>, mirror into your registry or create a zip bundle with Hangar or
              <a href="https://github.com/rancher/hauler" target="_blank" rel="noopener noreferrer">Hauler</a>.
            </p>
            <ul class="next-steps-commands mirror-commands">
              <li><strong>Mirror:</strong> <code class="cmd">hangar mirror -f images.txt -d {{ props.destinationRegistry || '&lt;registry&gt;' }}</code></li>
              <li><strong>Zip bundle:</strong> <code class="cmd">hangar save -f images.txt -d bundle.zip</code> then <code class="cmd">hangar load -s bundle.zip -d {{ props.destinationRegistry || '&lt;registry&gt;' }}</code></li>
            </ul>
          </div>
        </div>
      </div>
    </Teleport>

    <div v-if="primePatchAvailable" class="patcher-section">
      <button type="button" class="btn btn-patcher-toggle" @click="patcherOpen = !patcherOpen">
        {{ patcherOpen ? '▼' : '▶' }} Prime patch: apply patched images with rke2-patcher &amp; HelmChartConfig
      </button>
      <div v-if="patcherOpen" class="patcher-body">
        <p class="patcher-intro">
          You selected <strong>Rancher Prime</strong> + <strong>RKE2</strong>. Prime RKE2 clusters can swap packaged
          component images (CoreDNS, ingress-nginx, Canal/Calico/Cilium, metrics-server, snapshot-controller, …) for
          CVE-fixed or newer tags without upgrading RKE2, using <code>rke2-patcher</code> or manual
          <code>HelmChartConfig</code> resources. Node-level images (<code>rke2-runtime</code>,
          <code>hardened-kubernetes</code>) are not Helm-managed and need a node rollout (see below).
        </p>

        <h5 class="patcher-sub">1. Prerequisites</h5>
        <ul class="patcher-list">
          <li>RKE2 Prime cluster with <code>prime: true</code> in <code>/etc/rancher/rke2/config.yaml</code>.</li>
          <li><code>kubeconfig</code> access + RBAC to create/update <code>HelmChartConfig</code> (namespace <code>kube-system</code>).</li>
          <li>Install <code>rke2-patcher</code> (Prime tools) — see
            <a href="https://documentation.suse.com/cloudnative/rke2/latest/en/prime-tools/rke2-patcher.html" target="_blank" rel="noopener noreferrer">rke2-patcher docs ↗</a>.</li>
          <li>Mirror the patched images into your registry first (use the <em>Export image list</em> button above, then <code>hangar mirror -f images.txt -d {{ props.destinationRegistry || '&lt;your-registry&gt;' }}</code>).</li>
        </ul>

        <h5 class="patcher-sub">2. Inspect &amp; patch with rke2-patcher</h5>
        <pre class="patcher-cmd"># Show effective config (registry, scanner, patch state)
rke2-patcher --config

# List currently-running packaged component images
rke2-patcher images

# Preview the HelmChartConfig that WOULD be applied (no changes)
rke2-patcher image-patch &lt;component&gt; --dry-run

# Apply the patch (creates/updates a HelmChartConfig, records patch state)
rke2-patcher image-patch &lt;component&gt; --yes

# Re-apply recorded patches after an RKE2 upgrade
rke2-patcher reconcile</pre>
        <p class="patcher-note">
          <code>&lt;component&gt;</code> is a packaged chart name, e.g. <code>rke2-coredns</code>,
          <code>rke2-ingress-nginx</code>, <code>rke2-canal</code>, <code>rke2-metrics-server</code>,
          <code>rke2-snapshot-controller</code>. <code>rke2-patcher</code> enforces a patch window — if the target
          tag is outside the allowed range it will tell you to upgrade RKE2 first.
        </p>

        <h5 class="patcher-sub">3. Manual HelmChartConfig (no rke2-patcher)</h5>
        <p class="patcher-text">
          Drop a manifest in <code>/var/lib/rancher/rke2/server/manifests/</code> on a server node (RKE2 reconciles it
          automatically). Match the <code>HelmChart</code> name/namespace:
        </p>
        <pre class="patcher-cmd">apiVersion: helm.cattle.io/v1
kind: HelmChartConfig
metadata:
  name: rke2-coredns            # e.g. rke2-ingress-nginx, rke2-canal, rke2-metrics-server
  namespace: kube-system
spec:
  valuesContent: |-
    image:
      repository: {{ props.destinationRegistry || '&lt;your-registry&gt;' }}/rancher/hardened-coredns
      tag: v1.14.4-build20260610   # the patched tag from your exported list</pre>

        <h5 class="patcher-sub">4. Node-level images (rke2-runtime, hardened-kubernetes)</h5>
        <p class="patcher-text">
          These are resolved by RKE2 at process startup from node config — there is no
          <code>HelmChartConfig</code> for them. Roll them out node-by-node with the system-upgrade-controller using
          <code>rke2-patcher node-plan</code> and the <a href="https://github.com/cwayne18/rke2-runtime-upgrader" target="_blank" rel="noopener noreferrer">rke2-runtime-upgrader ↗</a>:
        </p>
        <pre class="patcher-cmd">rke2-patcher node-plan --image-tag v1.35.6-rke2r1 \
  --upgrader-image ghcr.io/cwayne18/rke2-runtime-upgrader:latest rke2-runtime</pre>

        <p class="patcher-warn">
          <strong>Heads-up:</strong> if your cluster is managed by Rancher, prefer configuring packaged components
          through the Rancher UI / managed add-ons — direct <code>HelmChartConfig</code> edits can conflict with
          Rancher-managed charts. Reference:
          <a href="https://docs.rke2.io/add-ons/helm" target="_blank" rel="noopener noreferrer">RKE2 Helm/HelmChartConfig docs ↗</a>.
        </p>
      </div>
    </div>

    <div class="glossary-section">
      <button type="button" class="btn btn-glossary-toggle" @click="glossaryOpen = !glossaryOpen">
        {{ glossaryOpen ? '▼' : '▶' }} What is what in Rancher?
      </button>
      <div v-if="glossaryOpen" class="glossary-body">
        <p class="glossary-intro">The tree mirrors how Rancher bundles images: <strong>Essentials</strong> are required for your cluster profile; <strong>AddOns</strong> are optional Helm charts from the Rancher catalog.</p>
        <dl class="glossary-list">
          <dt>Essentials / Rancher</dt>
          <dd>Management server, agent, webhooks, Fleet (GitOps), Rancher Turtles (CAPI — default 2.13+, only path in 2.14+), cert-manager (TLS), and system charts Rancher auto-installs.</dd>
          <dt>CNI</dt>
          <dd>Pod networking — Calico, Canal, Flannel, or Cilium depending on your Step 1 choice.</dd>
          <dt>RKE2 / K3s</dt>
          <dd>Distribution-specific node and system images for the Kubernetes version(s) you selected.</dd>
          <dt>Load Balancer / Ingress</dt>
          <dd>Ingress controller images (nginx, Traefik) for exposing workloads outside the cluster.</dd>
          <dt>Charts vs Images</dt>
          <dd>A <em>chart</em> is a Helm package; each chart references one or more container <em>images</em> to mirror.</dd>
          <dt>Version badges</dt>
          <dd>Inferred from image tags in the chart — use Release Notes below for official Rancher release chart versions.</dd>
          <dt>Check Availability sizes</dt>
          <dd>Compressed pull size (linux/amd64 layers) from the container registry — run after selecting images.</dd>
        </dl>
        <div class="glossary-links">
          <h5 class="glossary-links-title">Component release links</h5>
          <div class="glossary-link-grid">
            <a v-if="cniForStandard" :href="cniDocs(cniForStandard)" target="_blank" rel="noopener noreferrer">{{ cniForStandard.replace('cni_', '') }} docs ↗</a>
            <a v-if="cniRelease(cniForStandard)" :href="cniRelease(cniForStandard)" target="_blank" rel="noopener noreferrer">{{ cniForStandard.replace('cni_', '') }} releases ↗</a>
            <a :href="githubRelease(STACK_COMPONENTS.coredns.repo)" target="_blank" rel="noopener noreferrer">CoreDNS ↗</a>
            <a :href="githubRelease(STACK_COMPONENTS.fleet.repo)" target="_blank" rel="noopener noreferrer">Fleet ↗</a>
            <a :href="githubRelease(STACK_COMPONENTS.ingressNginx.repo)" target="_blank" rel="noopener noreferrer">Ingress NGINX ↗</a>
            <a :href="githubRelease(STACK_COMPONENTS.traefik.repo)" target="_blank" rel="noopener noreferrer">Traefik ↗</a>
            <a :href="githubRelease('cilium/cilium')" target="_blank" rel="noopener noreferrer">Cilium ↗</a>
            <a :href="githubRelease('projectcalico/calico')" target="_blank" rel="noopener noreferrer">Calico ↗</a>
            <a :href="githubRelease('flannel-io/flannel')" target="_blank" rel="noopener noreferrer">Flannel ↗</a>
          </div>
        </div>
      </div>
    </div>

    <div v-if="releaseVersions.length > 0" class="release-section">
      <button type="button" class="btn btn-release-toggle" @click="toggleReleaseNotes">
        {{ releaseNotesOpen ? '▼' : '▶' }} Release Notes &amp; Chart Versions
      </button>
      <div v-if="releaseNotesOpen" class="release-body">
        <p v-if="releaseLoading" class="loading-msg">Fetching release notes from GitHub…</p>
        <p v-if="releaseError" class="error-msg">{{ releaseError }}</p>
        <div v-for="(info, label) in releaseNotes" :key="label" class="release-card">
          <h4 class="release-card-title">
            {{ label }}
            <template v-if="releaseLifecycle[label]">
              <span v-if="releaseLifecycle[label].isCurrentMinor" class="lifecycle-badge badge-current" :title="lifecycleStatusTitle(releaseLifecycle[label])">Current</span>
              <span v-else-if="releaseLifecycle[label].isLatestPatch" class="lifecycle-badge badge-latest" :title="lifecycleStatusTitle(releaseLifecycle[label])">Latest patch</span>
              <span v-if="releaseLifecycle[label].status === 'maintenance'" class="lifecycle-badge badge-eom" :title="lifecycleStatusTitle(releaseLifecycle[label])">{{ lifecycleStatusLabel(releaseLifecycle[label].status) }}</span>
              <span v-if="releaseLifecycle[label].status === 'eol'" class="lifecycle-badge badge-eol" :title="lifecycleStatusTitle(releaseLifecycle[label])">{{ lifecycleStatusLabel(releaseLifecycle[label].status) }}</span>
            </template>
            <span v-if="info.publishedAt" class="release-date">{{ info.publishedAt.split('T')[0] }}</span>
            <a v-if="info.url" :href="info.url" target="_blank" class="release-link">GitHub ↗</a>
          </h4>

          <div v-if="info.charts && info.charts.length > 0" class="release-table-wrap">
            <h5 class="release-sub">Charts Versions</h5>
            <table class="release-table">
              <thead>
                <tr><th>Component</th><th>Version</th></tr>
              </thead>
              <tbody>
                <tr v-for="c in info.charts" :key="c.name">
                  <td>{{ c.name }}</td>
                  <td>{{ c.version }}</td>
                </tr>
              </tbody>
            </table>
          </div>

          <div v-if="info.changelog && info.changelog.length > 0" class="release-changelog">
            <h5 class="release-sub">Changelog</h5>
            <ul class="changelog-list">
              <li v-for="(entry, i) in info.changelog" :key="i">{{ entry }}</li>
            </ul>
          </div>
          <div v-else-if="info.body" class="release-body-text">
            <h5 class="release-sub">Release notes</h5>
            <pre class="release-notes-pre">{{ info.body }}</pre>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.step3 {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
  flex: 1;
}
.chart-release-link {
  flex-shrink: 0;
  font-size: 0.75rem;
  color: var(--accent);
  text-decoration: none;
  margin-left: 2px;
}
.chart-release-link:hover {
  text-decoration: underline;
}
.glossary-links {
  margin-top: 0.75rem;
  padding-top: 0.65rem;
  border-top: 1px dashed var(--border);
}
.glossary-links-title {
  margin: 0 0 0.4rem;
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--text-muted);
  text-transform: uppercase;
  letter-spacing: 0.04em;
}
.glossary-link-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.glossary-link-grid a {
  font-size: 0.75rem;
  padding: 0.2rem 0.45rem;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--accent);
  text-decoration: none;
}
.glossary-link-grid a:hover {
  border-color: var(--accent);
}
.step3-header {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.step3-header-top {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: wrap;
}
.step3-header-left {
  flex: 1;
  min-width: 0;
}
.step3-title-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 0.5rem 0.65rem;
}
.step3-title-row .step-title {
  padding-bottom: 0;
  border-bottom: none;
}
.selection-chip {
  max-width: min(420px, 55vw);
  padding: 0.15rem 0.55rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: color-mix(in srgb, var(--panel) 85%, var(--bg));
  color: var(--text-muted);
  font-size: 0.75rem;
  line-height: 1.35;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  cursor: default;
}
.btn-hints-toggle {
  padding: 0.2rem 0.55rem;
  font-size: 0.75rem;
  border-radius: 999px;
  border: 1px solid var(--border);
  background: var(--panel);
  color: var(--text-muted);
  cursor: pointer;
}
.btn-hints-toggle:hover {
  border-color: var(--accent);
  color: var(--accent);
}
.header-help-panel {
  padding: 0.55rem 0.75rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--panel) 90%, var(--bg));
}
.header-help-list {
  margin: 0;
  padding-left: 1.1rem;
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.45;
}
.header-help-list li {
  margin-bottom: 0.2rem;
}
.opt-in-hint-compact {
  margin: 0.35rem 0 0;
  padding: 0.35rem 0.55rem;
  font-size: 0.75rem;
}
.step3-header-actions {
  display: flex;
  gap: 0.5rem;
  flex-shrink: 0;
  flex-wrap: wrap;
}
.step-title {
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: var(--text);
  margin: 0;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border);
}
.step-desc {
  margin: 0.5rem 0 0;
  color: var(--text-muted);
  font-size: 0.8125rem;
}
.opt-in-hint {
  margin: 0.45rem 0 0;
  padding: 0.45rem 0.65rem;
  border-radius: 6px;
  border: 1px solid color-mix(in srgb, var(--yellow) 35%, var(--border));
  background: color-mix(in srgb, var(--yellow) 8%, var(--panel));
  color: var(--text-muted);
  font-size: 0.78rem;
  line-height: 1.45;
}
.opt-in-hint.opt-in-warn {
  border-color: color-mix(in srgb, var(--yellow) 50%, var(--border));
}
.past-selection {
  margin: 0;
  font-size: 0.8125rem;
  color: var(--text-muted);
}
.destination-registry-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-top: 0.5rem;
  flex-wrap: wrap;
  width: 100%;
}
.dest-registry-label {
  font-size: 0.9rem;
  white-space: nowrap;
}
.dest-registry-input {
  flex: 1;
  min-width: 180px;
  max-width: 320px;
  padding: 0.35rem 0.5rem;
  font-size: 0.9rem;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: var(--bg);
  color: var(--text);
}
.dest-registry-input-full {
  display: block;
  width: 100%;
  max-width: none;
  margin-top: 0.35rem;
}
.mirror-modal {
  max-width: 560px;
}
.mirror-modal-body {
  padding: 1rem 1.25rem 1.25rem;
}
.mirror-modal-desc {
  margin: 0.75rem 0 0.5rem;
  font-size: 0.85rem;
  color: var(--text-muted);
  line-height: 1.45;
}
.mirror-commands {
  margin-top: 0.35rem;
}
.scan-trivy-hint-compact {
  font-size: 0.75rem;
  line-height: 1.35;
  margin-top: 0;
}
.next-steps-box {
  margin-top: 0.75rem;
  padding: 0.75rem 1rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--panel) 80%, var(--bg));
  width: 100%;
}
.next-steps-title {
  margin: 0 0 0.35rem 0;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text);
}
.next-steps-desc {
  margin: 0 0 0.5rem 0;
  font-size: 0.85rem;
  opacity: 0.9;
}
.next-steps-desc code,
.next-steps-commands .cmd {
  background: var(--bg);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.85em;
}
.next-steps-commands {
  margin: 0;
  padding-left: 1.25rem;
  font-size: 0.85rem;
  line-height: 1.5;
}
.next-steps-commands li {
  margin-bottom: 0.35rem;
}
.next-steps-commands a {
  color: var(--cyan);
}
.tree-layout-wrapper {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
}
.mobile-tabs {
  display: none; /* shown only in @media (max-width: 768px) */
}
.tree-layout {
  display: grid;
  grid-template-columns: 1fr 1fr 1.2fr;
  gap: 1rem;
  flex: 1;
  min-height: 0;
}
.col {
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 0.75rem;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  background: var(--panel);
}
.col-title {
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  margin: 0 0 0.5rem 0;
  color: var(--text-muted);
}
.tree {
  flex: 1;
  overflow-y: auto;
  font-size: 0.9rem;
}
.tree-row {
  display: flex;
  align-items: center;
  gap: 4px;
  padding: 2px 0;
  cursor: default;
}
.expand {
  cursor: pointer;
  width: 16px;
  color: var(--text-muted);
  user-select: none;
}
.expand-placeholder {
  width: 16px;
  display: inline-block;
}
.row-label {
  display: flex;
  align-items: center;
  gap: 6px;
  cursor: pointer;
  flex: 1;
}
.label-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.count {
  opacity: 0.8;
  font-size: 0.85em;
}
.chart-icon {
  flex-shrink: 0;
  width: 16px;
  height: 16px;
  object-fit: contain;
  border-radius: 3px;
  background: color-mix(in srgb, var(--border) 25%, transparent);
}
.chart-icon-stack {
  display: inline-flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
}
.chart-icon-secondary {
  width: 14px;
  height: 14px;
}
.img-row-icon-stack {
  margin-right: 2px;
}
.chart-icon.rancher-cow-icon {
  filter: brightness(0) invert(1);
}
[data-theme="light"] .chart-icon.rancher-cow-icon {
  filter: brightness(0);
}

.kind-badge {
  flex-shrink: 0;
  font-size: 0.65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.03em;
  padding: 1px 4px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--border) 40%, transparent);
  color: var(--text-muted);
}
.version-badge {
  flex-shrink: 0;
  font-size: 0.72rem;
  font-weight: 600;
  padding: 1px 5px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--cyan) 18%, transparent);
  color: var(--cyan);
  font-family: ui-monospace, monospace;
}
.category-badge {
  flex-shrink: 0;
  font-size: 0.68rem;
  padding: 1px 5px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--yellow) 15%, transparent);
  color: var(--yellow);
  text-transform: lowercase;
}
.tag-badge {
  flex-shrink: 0;
  font-size: 0.68rem;
  padding: 1px 4px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--green) 12%, transparent);
  color: var(--green);
  font-family: ui-monospace, monospace;
}
.size-badge {
  flex-shrink: 0;
  font-size: 0.68rem;
  padding: 1px 4px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--border) 35%, transparent);
  color: var(--text-muted);
  font-family: ui-monospace, monospace;
}
.chart-preview-item {
  flex-wrap: wrap;
  gap: 4px;
}
.chart-name {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.image-preview-item {
  flex-wrap: wrap;
  gap: 4px;
}
.glossary-section {
  border-top: 1px solid var(--border);
  padding-top: 0.5rem;
}
.btn-glossary-toggle {
  background: none;
  border: none;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 0.85rem;
  font-weight: 500;
  padding: 0.25rem 0;
}
.btn-glossary-toggle:hover {
  color: var(--cyan);
}
.glossary-body {
  margin-top: 0.35rem;
  padding: 0.65rem 0.85rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: color-mix(in srgb, var(--panel) 85%, var(--bg));
  font-size: 0.82rem;
}
.glossary-intro {
  margin: 0 0 0.5rem;
  line-height: 1.45;
}
.glossary-list {
  margin: 0;
  display: grid;
  gap: 0.35rem 1rem;
  grid-template-columns: minmax(120px, 160px) 1fr;
}
.glossary-list dt {
  font-weight: 600;
  color: var(--cyan);
  margin: 0;
}
.glossary-list dd {
  margin: 0;
  opacity: 0.9;
  line-height: 1.4;
}
.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-bottom: 0.5rem;
  font-size: 0.75rem;
  opacity: 0.85;
}
.legend-item {
  font-weight: 600;
}
.preview-list {
  flex: 1;
  overflow-y: auto;
  margin: 0;
  padding-left: 0.5rem;
  font-size: 0.8rem;
  list-style: none;
}
.preview-list.images {
  font-family: ui-monospace, monospace;
}
.images-table-wrap {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.images-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: fixed;
  font-size: 0.78rem;
  font-family: ui-monospace, monospace;
}
.images-table thead {
  position: sticky;
  top: 0;
  z-index: 1;
  background: var(--panel);
}
.images-table th {
  text-align: left;
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 4px 6px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
}
.images-table td {
  padding: 3px 6px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
  vertical-align: middle;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.images-table-row {
  cursor: pointer;
}
.images-table-row:hover td {
  background: color-mix(in srgb, var(--border) 20%, transparent);
}
.images-table-row.is-expanded td {
  background: color-mix(in srgb, var(--cyan) 8%, transparent);
  border-bottom: none;
}
.images-table .col-image {
  width: 58%;
}
.images-table td.col-image {
  overflow: hidden;
}
.img-cell-inner {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.images-table .img-row-icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  object-fit: contain;
  border-radius: 3px;
}
.tree-row-unselected {
  opacity: 0.45;
}
.tree-row-unselected .row-label {
  text-decoration: line-through;
  text-decoration-color: color-mix(in srgb, var(--text-muted) 70%, transparent);
}
.images-table .img-expand {
  flex-shrink: 0;
  display: inline-block;
  width: 14px;
  color: var(--text-muted);
  font-size: 0.65rem;
}
.img-name-stack {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
  overflow: hidden;
}
.images-table .img-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 0.92rem;
  font-weight: 600;
  letter-spacing: -0.01em;
}
.img-repo-path {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.68rem;
  color: var(--text-muted);
  opacity: 0.85;
  font-family: ui-monospace, monospace;
}
.img-tree-name {
  font-weight: 600;
  font-size: 0.92rem;
}
.tree-row-image .row-label {
  align-items: baseline;
}
.images-table-detail-row td {
  padding: 0 6px 6px 24px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
}
.images-table-detail {
  background: color-mix(in srgb, var(--cyan) 6%, var(--panel));
}
.img-detail-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.35rem 1rem;
  font-size: 0.72rem;
}
.img-detail-item {
  min-width: 0;
}
.img-detail-label {
  display: block;
  font-size: 0.62rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-bottom: 1px;
}
.img-detail-value {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-family: ui-monospace, monospace;
}
.img-detail-value code,
code.img-detail-value {
  font-size: inherit;
  background: color-mix(in srgb, var(--border) 30%, transparent);
  padding: 1px 4px;
  border-radius: 3px;
}
.images-table .col-status { width: 28px; text-align: center; }
.images-table .col-tag { width: 18%; }
.images-table .col-chart { width: 16%; }
.images-table .col-life { width: 72px; }
.images-table .col-size { width: 56px; }
.sortable-th {
  cursor: pointer;
  user-select: none;
}
.sortable-th:hover {
  color: var(--cyan);
}
.sortable-th.sort-active {
  color: var(--cyan);
}
.sort-indicator {
  font-size: 0.62rem;
  opacity: 0.9;
}
.col-title-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  margin-bottom: 0.25rem;
}
.col-title-row .col-title {
  margin: 0;
}
.btn-images-modal {
  flex-shrink: 0;
  padding: 0.2rem 0.55rem;
  font-size: 0.72rem;
  font-weight: 600;
  border: 1px solid var(--border);
  border-radius: 4px;
  background: color-mix(in srgb, var(--border) 25%, transparent);
  color: var(--text);
  cursor: pointer;
}
.btn-images-modal:hover {
  border-color: var(--cyan);
  color: var(--cyan);
}
.images-sort-bar {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem 0.35rem;
  margin-bottom: 0.35rem;
}
.images-sort-label {
  font-size: 0.65rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  margin-right: 0.15rem;
}
.images-sort-btn {
  padding: 0.15rem 0.45rem;
  font-size: 0.68rem;
  font-weight: 600;
  border: 1px solid transparent;
  border-radius: 4px;
  background: color-mix(in srgb, var(--border) 20%, transparent);
  color: var(--text-muted);
  cursor: pointer;
  font-family: system-ui, -apple-system, sans-serif;
}
.images-sort-btn:hover {
  color: var(--text);
  border-color: var(--border);
}
.images-sort-btn.active {
  color: var(--cyan);
  border-color: color-mix(in srgb, var(--cyan) 50%, var(--border));
  background: color-mix(in srgb, var(--cyan) 10%, transparent);
}
.link-btn {
  background: none;
  border: none;
  padding: 0;
  font: inherit;
  color: var(--cyan);
  cursor: pointer;
  text-decoration: underline;
}
.img-chart-name {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  font-size: 0.72rem;
  color: var(--text-muted);
}
.preview-more-popup-hint {
  font-size: 0.72rem;
  color: var(--text-muted);
}
.img-detail-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem 1rem;
  margin-top: 0.5rem;
  padding-top: 0.35rem;
  border-top: 1px solid color-mix(in srgb, var(--border) 40%, transparent);
}
.img-detail-link {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--cyan);
  text-decoration: none;
  font-family: system-ui, -apple-system, sans-serif;
}
.img-detail-link:hover {
  text-decoration: underline;
}
.images-table .img-ref {
  display: block;
  overflow: hidden;
  text-overflow: ellipsis;
}
.images-table .img-origin {
  display: block;
  font-size: 0.68rem;
  opacity: 0.65;
  overflow: hidden;
  text-overflow: ellipsis;
}
.images-table .lifecycle-badge {
  font-size: 0.62rem;
  padding: 1px 4px;
}
.preview-item {
  display: flex;
  align-items: center;
  gap: 2px;
  padding: 2px 0;
  overflow: hidden;
  white-space: nowrap;
}
.img-tag {
  margin-right: 6px;
  font-weight: 600;
  font-size: 0.85em;
}
.tag-Rancher,
.tag-Fleet {
  color: var(--cyan);
}
.tag-CNI {
  color: var(--yellow);
}
.tag-LoadBalancerIngress {
  color: #6b7280;
}
.tag-Ks,
.tag-RKE,
.tag-RKE1 {
  color: var(--green);
}
.tag-addons {
  color: var(--green);
}
.img-ref {
  flex-shrink: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}
.img-origin {
  flex-shrink: 0;
  font-size: 0.7rem;
  opacity: 0.55;
  margin-left: auto;
  padding-left: 6px;
  white-space: nowrap;
}
.preview-more {
  opacity: 0.8;
  font-style: italic;
}
.btn {
  padding: 0.5rem 1rem;
  border-radius: 4px;
  border: 1px solid var(--border);
  cursor: pointer;
  font-size: 0.95rem;
}
.btn-primary {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
  font-weight: 500;
}
.btn-primary:hover {
  background: var(--accent-hover);
  border-color: var(--accent-hover);
}
.btn-secondary {
  background: var(--panel);
  color: var(--text);
}
.btn-check {
  background: var(--panel);
  color: var(--text);
  border-color: var(--border);
}
.btn-check:hover:not(:disabled) {
  border-color: var(--cyan);
}
.btn-sizes {
  background: color-mix(in srgb, var(--cyan) 18%, var(--panel));
  color: var(--text);
  border-color: color-mix(in srgb, var(--cyan) 40%, transparent);
}
.btn-sizes:hover:not(:disabled) {
  border-color: var(--cyan);
}
.btn-scan {
  background: var(--panel);
  color: var(--text);
  border-color: var(--border);
}
.btn-scan:hover:not(:disabled) {
  border-color: var(--cyan);
}
.scan-summary {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  font-size: 0.85rem;
  flex-wrap: wrap;
}
.scan-done {
  color: var(--green);
}
.scan-ongoing .scan-done {
  color: var(--cyan);
}
.scan-hint {
  font-size: 0.8rem;
  opacity: 0.85;
}
.scan-sev {
  padding: 2px 6px;
  border-radius: 3px;
}
.scan-sev.critical { background: #4a1515; color: #ffa0a0; }
.scan-sev.high { background: #4a3010; color: #ffc060; }
.scan-sev.medium { background: #2a3a2a; color: #a0d0a0; }
.scan-sev.low { background: #1a2a3a; color: #a0c0e0; }
.btn-download-report {
  font-size: 0.85rem;
  padding: 4px 10px;
  background: var(--panel);
  color: var(--cyan);
  border: 1px solid var(--border);
  border-radius: 4px;
  cursor: pointer;
}
.btn-download-report:hover {
  border-color: var(--cyan);
}
.scan-logs-row {
  margin-top: 0.5rem;
}
.scan-progress-wrap {
  max-width: 520px;
  margin: 0.35rem 0 0.5rem;
}
.scan-trivy-hint {
  font-size: 0.8rem;
  opacity: 0.85;
  margin-top: 0.25rem;
}
.scan-trivy-hint code {
  background: var(--bg);
  padding: 2px 6px;
  border-radius: 3px;
  font-size: 0.9em;
}
.avail-summary {
  display: flex;
  gap: 1rem;
  font-size: 0.85rem;
  padding: 0.25rem 0;
}
.avail-ok {
  color: var(--green);
  font-weight: 600;
}
.avail-fail {
  color: #ef4444;
  font-weight: 600;
}
.avail-noarch {
  color: var(--yellow, #eab308);
  font-weight: 600;
}
.avail-err {
  color: var(--yellow);
}
.size-summary {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 0.75rem;
  align-items: center;
  font-size: 0.82rem;
  padding: 0.25rem 0;
}
.img-source-link {
  margin-left: 4px;
  color: var(--cyan);
  text-decoration: none;
  font-size: 0.75rem;
  opacity: 0.85;
  flex-shrink: 0;
}
.img-source-link:hover {
  opacity: 1;
  text-decoration: underline;
}
.img-upstream-link {
  font-size: 0.62rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  font-family: system-ui, -apple-system, sans-serif;
}
.avail-total {
  margin-left: auto;
  font-weight: 600;
  color: var(--text);
  background: color-mix(in srgb, var(--cyan) 12%, transparent);
  border: 1px solid color-mix(in srgb, var(--cyan) 30%, transparent);
  padding: 1px 8px;
  border-radius: 4px;
  font-family: ui-monospace, monospace;
}
.avail-total-sub {
  font-weight: 400;
  opacity: 0.75;
  font-family: var(--font-sans, inherit);
}
.lifecycle-badge {
  font-size: 0.58rem;
  font-weight: 700;
  padding: 0 4px;
  border-radius: var(--radius-sm, 3px);
  text-transform: uppercase;
  letter-spacing: 0.04em;
  line-height: 1.35;
  flex-shrink: 0;
}
.badge-current {
  background: var(--green, #22c55e);
  color: #fff;
}
.badge-latest {
  background: color-mix(in srgb, var(--accent) 18%, var(--panel));
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
}
.badge-eom {
  background: #eab308;
  color: #000;
}
.badge-eol {
  background: #64748b;
  color: #fff;
}
.avail-dot {
  margin-right: 4px;
  font-weight: 700;
  font-size: 0.9em;
}
.dot-ok {
  color: var(--green);
}
.dot-fail {
  color: #ef4444;
}
.dot-noarch {
  color: var(--yellow, #eab308);
}
.error-msg {
  color: #ef4444;
  font-size: 0.85rem;
}

.release-section {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
}
.btn-release-toggle {
  background: none;
  border: none;
  color: var(--accent);
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 600;
  padding: 0.25rem 0;
}
.btn-release-toggle:hover {
  text-decoration: underline;
}
.release-body {
  margin-top: 0.5rem;
}
.loading-msg {
  opacity: 0.7;
  font-size: 0.9rem;
}
.release-card {
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.75rem 1rem;
  margin-bottom: 0.75rem;
}
.release-card-title {
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--text);
  margin: 0 0 0.5rem 0;
  display: flex;
  align-items: center;
  gap: 0.75rem;
}
.release-date {
  font-size: 0.8rem;
  opacity: 0.7;
  font-weight: 400;
}
.release-link {
  font-size: 0.8rem;
  color: var(--cyan);
  text-decoration: none;
  font-weight: 400;
}
.release-link:hover {
  text-decoration: underline;
}
.release-sub {
  font-size: 0.85rem;
  color: var(--yellow);
  margin: 0.5rem 0 0.25rem;
}
.release-table-wrap {
  overflow-x: auto;
}
.release-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.82rem;
}
.release-table th,
.release-table td {
  text-align: left;
  padding: 3px 12px 3px 0;
  border-bottom: 1px solid var(--border);
}
.release-table th {
  font-weight: 600;
  opacity: 0.8;
}
.changelog-list {
  padding-left: 1.25rem;
  margin: 0.25rem 0 0;
  font-size: 0.82rem;
}
.changelog-list li {
  padding: 2px 0;
}
.release-body-text {
  margin-top: 0.5rem;
}
.release-notes-pre {
  white-space: pre-wrap;
  word-break: break-word;
  font-size: 0.82rem;
  max-height: 20em;
  overflow-y: auto;
  margin: 0.25rem 0 0;
  padding: 0.5rem;
  background: color-mix(in srgb, var(--border) 20%, transparent);
  border-radius: 4px;
}

.patcher-section {
  border-top: 1px solid var(--border);
  padding-top: 0.75rem;
}
.btn-patcher-toggle {
  background: none;
  border: none;
  color: var(--accent);
  cursor: pointer;
  font-size: 0.875rem;
  font-weight: 600;
  padding: 0.25rem 0;
}
.btn-patcher-toggle:hover {
  text-decoration: underline;
}
.patcher-body {
  margin-top: 0.5rem;
  padding: 0.75rem 1rem;
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--accent) 6%, var(--panel));
  font-size: 0.82rem;
  line-height: 1.5;
}
.patcher-intro {
  margin: 0 0 0.5rem;
}
.patcher-sub {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--cyan);
  margin: 0.75rem 0 0.25rem;
}
.patcher-list,
.patcher-text {
  margin: 0.25rem 0;
  padding-left: 1.1rem;
}
.patcher-list li {
  margin-bottom: 0.25rem;
}
.patcher-cmd {
  margin: 0.35rem 0;
  padding: 0.6rem 0.8rem;
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 5px;
  font-family: ui-monospace, monospace;
  font-size: 0.78rem;
  line-height: 1.45;
  white-space: pre-wrap;
  word-break: break-word;
  overflow-x: auto;
}
.patcher-note {
  margin: 0.35rem 0 0;
  font-size: 0.8rem;
  opacity: 0.9;
}
.patcher-warn {
  margin: 0.75rem 0 0;
  padding: 0.5rem 0.7rem;
  border-left: 3px solid var(--yellow, #eab308);
  background: color-mix(in srgb, var(--yellow, #eab308) 8%, transparent);
  font-size: 0.8rem;
  border-radius: 0 4px 4px 0;
}
.patcher-body code {
  background: var(--bg);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 0.85em;
}
.patcher-body a {
  color: var(--cyan);
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .step3-header {
    flex-direction: column;
    gap: 0.75rem;
    align-items: stretch;
  }
  .step3-header-actions {
    flex-wrap: wrap;
    width: 100%;
  }
  .step3-header-actions .btn {
    flex: 1 1 auto;
    min-width: 120px;
  }
  .destination-registry-row {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.35rem;
  }
  .dest-registry-input {
    width: 100%;
    max-width: none;
    min-width: 0;
  }
  .next-steps-commands {
    padding-left: 1rem;
    font-size: 0.8rem;
  }
  .next-steps-commands .cmd {
    display: block;
    margin-top: 0.25rem;
    overflow-x: auto;
    white-space: pre;
    padding: 0.35rem 0.5rem;
  }
  /* Mobile: tab bar + single full-width panel */
  .mobile-tabs {
    display: flex;
    gap: 0;
    padding: 0.25rem;
    background: var(--bg);
    border: 1px solid var(--border);
    border-radius: 10px;
    margin-bottom: 0.75rem;
    flex-shrink: 0;
  }
  .mobile-tab {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35rem;
    padding: 0.6rem 0.5rem;
    font-size: 0.85rem;
    font-weight: 600;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: var(--text);
    opacity: 0.75;
    cursor: pointer;
    transition: background 0.2s, opacity 0.2s;
  }
  .mobile-tab:hover {
    opacity: 1;
  }
  .mobile-tab.active {
    background: var(--panel);
    color: var(--cyan);
    opacity: 1;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
  }
  .mobile-tab-count {
    font-size: 0.75rem;
    font-weight: 500;
    opacity: 0.9;
  }
  .tree-layout {
    display: flex;
    flex-direction: column;
    gap: 0;
    flex: 1;
    min-height: 0;
  }
  .tree-layout .col {
    display: none;
    flex: 1;
    min-height: 280px;
    max-height: 100%;
  }
  .tree-layout .col.mobile-panel-active {
    display: flex;
  }
  .col {
    min-height: 280px;
  }
  .tree {
    font-size: 0.85rem;
  }
  .preview-list {
    font-size: 0.75rem;
  }
  .preview-item {
    flex-wrap: wrap;
    gap: 2px;
  }
  .img-ref {
    min-width: 0;
    word-break: break-all;
    white-space: normal;
  }
  .legend {
    gap: 0.35rem;
  }
  .legend-item {
    font-size: 0.7rem;
  }
  .scan-summary {
    flex-direction: column;
    align-items: flex-start;
  }
  .avail-summary {
    flex-wrap: wrap;
    gap: 0.5rem;
  }
}

@media (max-width: 480px) {
  .step-title {
    font-size: 1.1rem;
  }
  .step-desc {
    font-size: 0.8rem;
  }
  .step3-header-actions {
    flex-direction: column;
  }
  .step3-header-actions .btn {
    width: 100%;
    min-width: 0;
  }
  .dest-registry-label {
    font-size: 0.85rem;
  }
  .next-steps-box {
    padding: 0.5rem 0.75rem;
  }
  .next-steps-title {
    font-size: 0.9rem;
  }
  .next-steps-commands li {
    margin-bottom: 0.5rem;
  }
  .next-steps-commands .cmd {
    font-size: 0.75em;
    white-space: pre-wrap;
    word-break: break-all;
  }
  .mobile-tabs {
    padding: 0.2rem;
  }
  .mobile-tab {
    flex-direction: column;
    gap: 0.15rem;
    padding: 0.5rem 0.25rem;
    font-size: 0.8rem;
  }
  .mobile-tab-label {
    white-space: nowrap;
  }
  .mobile-tab-count {
    font-size: 0.7rem;
  }
  .tree-layout .col {
    min-height: 220px;
  }
  .col {
    padding: 0.5rem;
    min-height: 220px;
  }
  .col-title {
    font-size: 0.85rem;
  }
  .tree-row {
    padding: 3px 0;
  }
  .row-label {
    font-size: 0.85rem;
  }
  .label-text {
    white-space: normal;
    word-break: break-word;
  }
  .preview-list.images .preview-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
  }
  .images-table .col-image {
    width: 58%;
  }
  .img-ref {
    font-size: 0.7rem;
  }
  .btn,
  .btn-check,
  .btn-scan,
  .btn-primary,
  .btn-secondary {
    padding: 0.45rem 0.75rem;
    font-size: 0.88rem;
  }
  .release-card {
    padding: 0.5rem 0.75rem;
  }
  .release-card-title {
    flex-wrap: wrap;
    font-size: 0.9rem;
  }
  .release-table th,
  .release-table td {
    padding: 2px 8px 2px 0;
    font-size: 0.78rem;
  }
}
</style>

<style>
.images-modal-backdrop {
  position: fixed;
  inset: 0;
  z-index: 100000;
  background: rgba(0, 0, 0, 0.62);
  backdrop-filter: blur(2px);
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 1rem;
  box-sizing: border-box;
}
.images-modal {
  width: min(1200px, 96vw);
  max-height: min(92vh, 900px);
  display: flex;
  flex-direction: column;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  box-shadow: 0 24px 64px rgba(0, 0, 0, 0.45);
  overflow: hidden;
}
.images-modal-header {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.75rem 1rem;
  border-bottom: 1px solid var(--border);
  flex-shrink: 0;
}
.images-modal-title {
  margin: 0;
  font-size: 1.05rem;
  font-weight: 600;
}
.images-modal-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.5rem;
}
.images-sort-bar-modal {
  margin-bottom: 0;
}
.images-modal-backdrop .btn-modal-close {
  flex-shrink: 0;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: transparent;
  color: var(--text-muted);
  cursor: pointer;
  font-size: 1rem;
  line-height: 1;
}
.images-modal-backdrop .btn-modal-close:hover {
  color: var(--text);
  border-color: var(--text-muted);
  background: color-mix(in srgb, var(--border) 30%, transparent);
}
.images-modal-body {
  flex: 1;
  min-height: 0;
  overflow: auto;
  padding: 0.5rem 1rem 1rem;
  overscroll-behavior: contain;
}
.images-modal-backdrop .images-table {
  width: 100%;
  border-collapse: collapse;
  table-layout: auto;
  font-size: 0.82rem;
  font-family: ui-monospace, monospace;
}
.images-modal-backdrop .images-table th {
  text-align: left;
  font-size: 0.68rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
  padding: 6px 8px;
  border-bottom: 1px solid var(--border);
  white-space: nowrap;
  position: sticky;
  top: 0;
  background: var(--panel);
  z-index: 1;
}
.images-modal-backdrop .images-table td {
  padding: 4px 8px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 50%, transparent);
  vertical-align: middle;
}
.images-modal-backdrop .images-table-row {
  cursor: pointer;
}
.images-modal-backdrop .images-table-row:hover td {
  background: color-mix(in srgb, var(--border) 20%, transparent);
}
.images-modal-backdrop .images-table-row.is-expanded td {
  background: color-mix(in srgb, var(--cyan) 8%, transparent);
  border-bottom: none;
}
.images-modal-backdrop .img-cell-inner {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.images-modal-backdrop .img-row-icon {
  flex-shrink: 0;
  width: 18px;
  height: 18px;
  object-fit: contain;
  border-radius: 3px;
}
.images-modal-backdrop .col-image {
  min-width: 180px;
  width: 28%;
}
.images-modal-backdrop .img-cell-inner {
  display: flex;
  align-items: center;
  gap: 6px;
  min-width: 0;
}
.images-modal-backdrop .img-name {
  font-family: system-ui, -apple-system, sans-serif;
  font-size: 0.92rem;
  font-weight: 600;
  white-space: normal;
  word-break: break-word;
}
.images-modal-backdrop .img-repo-path {
  font-size: 0.68rem;
  color: var(--text-muted);
  white-space: normal;
  word-break: break-all;
}
.images-modal-backdrop .img-name-stack {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
}
.images-modal-backdrop .img-expand {
  flex-shrink: 0;
  width: 14px;
  color: var(--text-muted);
  font-size: 0.65rem;
}
.images-modal-backdrop .images-table-detail {
  background: color-mix(in srgb, var(--cyan) 6%, var(--panel));
  padding: 0 8px 8px 28px;
}
.images-modal-backdrop .img-detail-link {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--cyan);
  text-decoration: none;
}
.images-modal-backdrop .img-detail-link:hover {
  text-decoration: underline;
}
.images-modal-backdrop .images-sort-btn {
  padding: 0.15rem 0.45rem;
  font-size: 0.68rem;
  font-weight: 600;
  border: 1px solid transparent;
  border-radius: 4px;
  background: color-mix(in srgb, var(--border) 20%, transparent);
  color: var(--text-muted);
  cursor: pointer;
}
.images-modal-backdrop .images-sort-btn.active {
  color: var(--cyan);
  border-color: color-mix(in srgb, var(--cyan) 50%, var(--border));
}
</style>
