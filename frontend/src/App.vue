<script setup lang="ts">
import { ref, reactive, onMounted, watch, onUnmounted, computed, defineAsyncComponent } from 'vue'
import {
  fetchRancherVersions,
  fetchStep1OptionsMerged,
  generate,
  exportImageList,
  fetchLogs,
  fetchProgress,
  peekRancherVersionsCache,
  peekStep1OptionsCache,
  type RancherVersionInfo,
} from './api/genesis'
import type { Step1OptionsResponse, GenerateRequest, GenerateResponse } from './types/genesis'
import { isRancherPreRelease, pickLatestStableRancherVersion } from './utils/versionLifecycle'
import Step1Form from './components/Step1Form.vue'
import Step3Tree from './components/Step3Tree.vue'
import LoadingShapes from './components/LoadingShapes.vue'
import LoadingProgressBar from './components/LoadingProgressBar.vue'
import ServerLogsPanel from './components/ServerLogsPanel.vue'
import GenesisRootMark from './components/GenesisRootMark.vue'
import { deriveGenerateProgress, formatLogsForDisplay } from './utils/serverLogs'
import type { ProgressResponse } from './api/genesis'

const DocsViewer = defineAsyncComponent(() => import('./components/DocsViewer.vue'))
import {
  cniDocs,
  cniRelease,
  githubRelease,
  loadBalancerLinks,
  rancherRelease,
  STACK_COMPONENTS,
} from './utils/componentLinks'

const VERSION = '0.1'
const theme = ref<'dark' | 'light'>('dark')

onMounted(async () => {
  const saved = localStorage.getItem('genesis-theme') as 'dark' | 'light' | null
  if (saved) theme.value = saved
  if (theme.value === 'light') document.documentElement.setAttribute('data-theme', 'light')
})

function toggleTheme() {
  theme.value = theme.value === 'dark' ? 'light' : 'dark'
  if (theme.value === 'light') {
    document.documentElement.setAttribute('data-theme', 'light')
  } else {
    document.documentElement.removeAttribute('data-theme')
  }
  localStorage.setItem('genesis-theme', theme.value)
}

const step = ref<'step1' | 'loading' | 'step3'>('step1')
// Hash-based page: '' = app, 'docs' = documentation (separate page)
function normalizeHash(raw: string): string {
  return raw.replace(/^\/+/, '')
}

function isDocsHash(raw: string): boolean {
  const h = normalizeHash(raw)
  return h === 'docs' || h.startsWith('docs/')
}

const page = ref<'app' | 'docs'>(
  typeof window !== 'undefined' && isDocsHash(window.location.hash.slice(1)) ? 'docs' : 'app'
)
function updatePageFromHash() {
  page.value = isDocsHash(window.location.hash.slice(1)) ? 'docs' : 'app'
}
function goBackToApp() {
  window.location.hash = ''
  updatePageFromHash()
}
onMounted(() => {
  updatePageFromHash()
  window.addEventListener('hashchange', updatePageFromHash)
})
onUnmounted(() => {
  window.removeEventListener('hashchange', updatePageFromHash)
})
const step1Options = ref<Step1OptionsResponse | null>(null)
const rancherVersions = ref<RancherVersionInfo[]>([])
const step1Error = ref('')
const includeRC = ref(false)
const includeGitHubVersions = ref(false)
const genRequest = reactive<GenerateRequest>({
  rancherVersion: '',
  rancherVersions: [],
  isRPMGC: false,
  includeCommunityImageLists: true,
  includeAppCollectionCharts: false,
  includePartnerCharts: false,
  includeUIPluginCharts: false,
  appCollectionAPIUser: '',
  appCollectionAPIPassword: '',
  distros: ['rke2'],
  cni: 'cni_calico',
  cnis: [],
  arch: 'amd64',
  loadBalancer: true,
  lbK3sKlipper: false,
  lbK3sTraefik: false,
  lbRKE2Nginx: true,
  lbRKE2Traefik: false,
  includeWindows: false,
  k3sVersions: [],
  rke2Versions: [],
  includeDeprecatedPatches: false,
  destinationRegistry: '',
})

const LS_RANCHER_VERSIONS = 'genesis-rancher-versions'

/** K3s/RKE2 GitHub pre-releases only apply when that opt-in is enabled. */
const includeDistroRC = computed(() => includeGitHubVersions.value && includeRC.value)

function restorePersistedRancherVersions() {
  try {
    const raw = localStorage.getItem(LS_RANCHER_VERSIONS)
    if (!raw || genRequest.rancherVersions?.length) return
    const versions = (JSON.parse(raw) as string[]).filter((v) => !isRancherPreRelease(v))
    if (!versions?.length) return
    genRequest.rancherVersions = versions
    genRequest.rancherVersion = versions[0] ?? ''
  } catch {
    /* ignore */
  }
}

function hydrateFromClientCache() {
  const cachedVersions = peekRancherVersionsCache(false)
  if (cachedVersions?.length) rancherVersions.value = cachedVersions

  const selected = genRequest.rancherVersions?.length
    ? genRequest.rancherVersions
    : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])
  if (selected.length === 1 && selected[0]) {
    const cachedOptions = peekStep1OptionsCache(selected[0], includeDistroRC.value, includeGitHubVersions.value, genRequest.includeDeprecatedPatches ?? false)
    if (cachedOptions) step1Options.value = cachedOptions
  }
}

restorePersistedRancherVersions()
hydrateFromClientCache()

const genResponse = ref<GenerateResponse | null>(null)
const genError = ref('')
const exportError = ref('')
const serverLogs = ref<string[]>([])
const loadingProgress = ref<ProgressResponse>({ active: true, percent: 0, phase: 'Starting generation…' })
let logsPollTimer: ReturnType<typeof setInterval> | null = null
let loadingStartedAt = 0

const formattedServerLogs = computed(() => formatLogsForDisplay(serverLogs.value))

function startLogsPoll() {
  if (logsPollTimer) return
  async function poll() {
    try {
      const [lines, progress] = await Promise.all([fetchLogs(), fetchProgress()])
      serverLogs.value = lines
      const logProgress = deriveGenerateProgress(lines, loadingStartedAt)
      const prev = loadingProgress.value
      const apiUsable = progress.active && progress.phase !== 'Idle'
      if (apiUsable) {
        loadingProgress.value = {
          active: true,
          percent: Math.max(prev.percent, progress.percent),
          phase: progress.phase,
          detail: progress.detail,
          current: progress.current,
          total: progress.total,
        }
      } else {
        loadingProgress.value = {
          active: true,
          percent: Math.max(prev.percent, logProgress.percent),
          phase: logProgress.phase,
        }
      }
    } catch {
      // ignore
    }
  }
  poll()
  logsPollTimer = setInterval(poll, 500)
}

function stopLogsPoll() {
  if (logsPollTimer) {
    clearInterval(logsPollTimer)
    logsPollTimer = null
  }
}

watch(step, (s) => {
  if (s === 'loading') startLogsPoll()
  else stopLogsPoll()
})
onUnmounted(stopLogsPoll)

const availableRancherVersionIds = computed(() => rancherVersions.value.map((rv) => rv.version))
const availableK3sVersionIds = computed(() => step1Options.value?.capabilities?.k3s?.versions ?? [])
const availableRke2VersionIds = computed(() => step1Options.value?.capabilities?.rke2?.versions ?? [])

const selectedLoadBalancers = computed(() => {
  const labels: string[] = []
  if (genRequest.lbK3sKlipper) labels.push('K3s Klipper')
  if (genRequest.lbK3sTraefik) labels.push('K3s Traefik')
  if (genRequest.lbRKE2Nginx) labels.push('RKE2 NGINX')
  if (genRequest.lbRKE2Traefik) labels.push('RKE2 Traefik')
  return labels
})

const lbComponentLinks = computed(() =>
  loadBalancerLinks({
    lbK3sKlipper: genRequest.lbK3sKlipper,
    lbK3sTraefik: genRequest.lbK3sTraefik,
    lbRKE2Nginx: genRequest.lbRKE2Nginx,
    lbRKE2Traefik: genRequest.lbRKE2Traefik,
  })
)

const optionsLoading = ref(false)
let loadAbort: AbortController | null = null

function setRancherSelection(versions: string[]) {
  genRequest.rancherVersions = versions
  genRequest.rancherVersion = versions[0] ?? ''
}

function sanitizeRancherSelection() {
  const available = rancherVersions.value.map((v) => v.version)
  const allowed = new Set(available)
  let sel = (genRequest.rancherVersions ?? []).filter((v) => allowed.has(v) && !isRancherPreRelease(v))
  if (!sel.length) {
    const latest = pickLatestStableRancherVersion(available)
    if (latest) sel = [latest]
  }
  if (sel.length) setRancherSelection(sel)
}

function applyLatestRancherDefault() {
  if (genRequest.rancherVersions?.length) {
    sanitizeRancherSelection()
    return
  }
  const latest = pickLatestStableRancherVersion(rancherVersions.value.map((v) => v.version))
  if (!latest) return
  setRancherSelection([latest])
}

async function loadRancherVersions() {
  try {
    // Rancher version dropdown is stable releases only; RC/alpha are not mixed in from K3s/RKE2 toggles.
    rancherVersions.value = await fetchRancherVersions(false)
    applyLatestRancherDefault()
  } catch { /* ignore */ }
}

async function loadOptions() {
  if (loadAbort) loadAbort.abort()
  loadAbort = new AbortController()
  step1Error.value = ''
  optionsLoading.value = true
  const versions = genRequest.rancherVersions?.length
    ? genRequest.rancherVersions
    : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])
  if (!versions.length) {
    optionsLoading.value = false
    return
  }
  try {
    step1Options.value = await fetchStep1OptionsMerged(versions, includeDistroRC.value, includeGitHubVersions.value, genRequest.includeDeprecatedPatches ?? false)
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    step1Error.value = e instanceof Error ? e.message : String(e)
  } finally {
    optionsLoading.value = false
  }
}

watch(() => [genRequest.rancherVersion, genRequest.rancherVersions], () => { loadOptions() }, { deep: true })
watch(includeRC, () => { loadOptions() })
watch(includeGitHubVersions, (on) => {
  if (!on) includeRC.value = false
  loadOptions()
})
watch(() => genRequest.includeDeprecatedPatches, () => { loadOptions() })

watch(
  () => genRequest.rancherVersions,
  (versions) => {
    if (versions?.length) {
      try {
        localStorage.setItem(LS_RANCHER_VERSIONS, JSON.stringify(versions))
      } catch {
        /* quota */
      }
    }
  },
  { deep: true }
)

onMounted(() => {
  if (genRequest.rancherVersions?.length || genRequest.rancherVersion) {
    void loadOptions()
  }
  void loadRancherVersions()
})

async function runGenerate() {
  genError.value = ''
  const versions = genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])
  if (!versions.length) {
    genError.value = 'Select at least one Rancher version.'
    return
  }
  if (!genRequest.includeCommunityImageLists && !genRequest.isRPMGC) {
    genError.value = 'Select at least one image list source (Community and/or Rancher Prime).'
    return
  }
  step.value = 'loading'
  loadingStartedAt = Date.now()
  loadingProgress.value = { active: true, percent: 5, phase: 'Starting generation…' }
  serverLogs.value = []
  startLogsPoll()
  genRequest.loadBalancer = genRequest.lbK3sKlipper || genRequest.lbK3sTraefik || genRequest.lbRKE2Nginx || genRequest.lbRKE2Traefik
  try {
    genResponse.value = await generate(genRequest)
    stopLogsPoll()
    serverLogs.value = []
    step.value = 'step3'
  } catch (e) {
    genError.value = e instanceof Error ? e.message : String(e)
    stopLogsPoll()
    step.value = 'step1'
  }
}

async function runExport(
  selectedComponentIDs: string[],
  chartNames: string[],
  selectedImageRefs: string[]
) {
  if (!genResponse.value) return
  exportError.value = ''
  try {
    const blob = await exportImageList({
      jobId: genResponse.value.jobId,
      selectedComponentIDs,
      chartNames,
      selectedImageRefs,
    })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'images.txt'
    a.click()
    URL.revokeObjectURL(a.href)
  } catch (e) {
    exportError.value = e instanceof Error ? e.message : String(e)
  }
}

function backToStep1() {
  step.value = 'step1'
  genResponse.value = null
}
</script>

<template>
  <div class="app">
    <header class="hero">
      <div class="hero-inner">
        <a href="#" class="hero-brand" @click.prevent="goBackToApp">
          <GenesisRootMark class="hero-logo-mark" />
          <h1 class="hero-brand-lockup">
            <span class="hero-name">GenesisRK</span>
          </h1>
          <span class="hero-version">v{{ VERSION }}</span>
        </a>
        <div class="hero-actions">
          <a href="https://github.com/aeltai/GenesisRK" target="_blank" rel="noopener noreferrer" class="hero-link" title="GitHub Repository">
            <svg viewBox="0 0 16 16" width="14" height="14" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
            GitHub
          </a>
          <a href="#" class="hero-link" :class="{ active: page === 'app' }" title="Image list generator" @click.prevent="goBackToApp">Generator</a>
          <a href="#docs" class="hero-link" :class="{ active: page === 'docs' }" title="Documentation">Docs</a>
          <button type="button" class="theme-toggle" :title="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'" @click="toggleTheme">
            {{ theme === 'dark' ? 'Light' : 'Dark' }}
          </button>
        </div>
      </div>
    </header>

    <main class="main">
      <div v-if="page === 'docs'" class="panel docs-panel">
        <DocsViewer />
      </div>

      <template v-else>
      <p v-if="step !== 'step3'" class="app-intro">
        Build image lists for air-gapped Rancher: choose Rancher version(s), distros (K3s, RKE2), CNI, load balancer, and charts—then export one list to mirror or save.
      </p>
      <div v-if="step === 'step1'" class="panel step1-panel">
        <div class="step1-layout">
          <div class="step1-form">
            <Step1Form
              :available-rancher-versions="rancherVersions"
              v-model:rancher-version="genRequest.rancherVersion"
              v-model:rancher-versions="genRequest.rancherVersions"
              v-model:include-r-c="includeRC"
              v-model:include-git-hub-versions="includeGitHubVersions"
              v-model:include-deprecated-patches="genRequest.includeDeprecatedPatches"
              v-model:isRPMGC="genRequest.isRPMGC"
              v-model:include-community-image-lists="genRequest.includeCommunityImageLists"
              v-model:include-app-collection="genRequest.includeAppCollectionCharts"
              v-model:include-partner-charts="genRequest.includePartnerCharts"
              v-model:includeUIPluginCharts="genRequest.includeUIPluginCharts"
              v-model:app-user="genRequest.appCollectionAPIUser"
              v-model:app-password="genRequest.appCollectionAPIPassword"
              v-model:distros="genRequest.distros"
              v-model:arch="genRequest.arch"
              v-model:cni="genRequest.cni"
              v-model:lb-k3s-klipper="genRequest.lbK3sKlipper"
              v-model:lb-k3s-traefik="genRequest.lbK3sTraefik"
              v-model:lbRKE2Nginx="genRequest.lbRKE2Nginx"
              v-model:lbRKE2Traefik="genRequest.lbRKE2Traefik"
              v-model:include-windows="genRequest.includeWindows"
              v-model:k3s-versions="genRequest.k3sVersions"
              v-model:rke2-versions="genRequest.rke2Versions"
              :options="step1Options"
              :load-error="step1Error"
              :options-loading="optionsLoading"
              @generate="runGenerate"
            />
            <p v-if="genError" class="error">{{ genError }}</p>
          </div>
          <aside v-if="(genRequest.rancherVersions?.length || genRequest.rancherVersion) && step1Options" class="step1-details">
            <div class="details-section details-section-spaced">
              <h4 class="details-heading">Rancher</h4>
              <ul class="details-links">
                <li v-for="v in (genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion])" :key="v">
                  <a :href="rancherRelease(v)" target="_blank" rel="noopener noreferrer">Release {{ v }}</a>
                </li>
                <li><a href="https://ranchermanager.docs.rancher.com/releases" target="_blank" rel="noopener noreferrer">Rancher release notes</a></li>
              </ul>
            </div>
            <div v-if="genRequest.cni" class="details-section">
              <h4 class="details-heading">CNI — {{ genRequest.cni.replace('cni_', '').replace('_', ' ') }}</h4>
              <ul class="details-links">
                <li><a :href="cniDocs(genRequest.cni)" target="_blank" rel="noopener noreferrer">Documentation</a></li>
                <li v-if="cniRelease(genRequest.cni)"><a :href="cniRelease(genRequest.cni)" target="_blank" rel="noopener noreferrer">Upstream releases</a></li>
              </ul>
            </div>
            <div class="details-section">
              <h4 class="details-heading">Cluster components</h4>
              <ul class="details-links">
                <li><a :href="githubRelease(STACK_COMPONENTS.coredns.repo)" target="_blank" rel="noopener noreferrer">CoreDNS releases</a></li>
                <li><a :href="STACK_COMPONENTS.coredns.docs" target="_blank" rel="noopener noreferrer">CoreDNS docs</a></li>
                <li><a :href="githubRelease(STACK_COMPONENTS.fleet.repo)" target="_blank" rel="noopener noreferrer">Fleet releases</a></li>
                <li><a :href="STACK_COMPONENTS.fleet.docs" target="_blank" rel="noopener noreferrer">Fleet docs</a></li>
                <li><a :href="githubRelease(STACK_COMPONENTS.metricsServer.repo)" target="_blank" rel="noopener noreferrer">Metrics Server releases</a></li>
              </ul>
            </div>
            <div v-if="lbComponentLinks.length" class="details-section">
              <h4 class="details-heading">Ingress / Load balancer</h4>
              <ul class="details-links">
                <li v-for="lb in lbComponentLinks" :key="lb.label">
                  <a :href="lb.href" target="_blank" rel="noopener noreferrer">{{ lb.label }}</a>
                  <span v-if="lb.hint" class="details-hint"> — {{ lb.hint }}</span>
                </li>
              </ul>
            </div>
            <div class="details-section">
              <h4 class="details-heading">Lifecycle &amp; support</h4>
              <ul class="details-links">
                <li><a href="https://www.suse.com/lifecycle" target="_blank" rel="noopener noreferrer">SUSE Product Lifecycle</a></li>
                <li><a href="https://www.suse.com/suse-rancher/support-matrix/all-supported-versions" target="_blank" rel="noopener noreferrer">Rancher support matrix (all versions)</a></li>
                <li v-for="v in (genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion])" :key="'matrix-' + v">
                  <a :href="'https://www.suse.com/suse-rancher/support-matrix/all-supported-versions/rancher-v' + v.replace(/^v/, '').replace(/\./g, '-')" target="_blank" rel="noopener noreferrer">Support matrix {{ v }}</a>
                </li>
              </ul>
            </div>
            <div v-if="step1Options.details" class="details-section">
              <h4 class="details-heading">Data sources</h4>
              <dl class="details-dl">
                <dt>KDM</dt>
                <dd><a v-if="step1Options.details.kdmUrl" :href="step1Options.details.kdmUrl" target="_blank" rel="noopener noreferrer">KDM data</a></dd>
                <dt>Image lists</dt>
                <dd><code>{{ step1Options.details.imageListSource }}</code></dd>
                <dt>Charts</dt>
                <dd><code>rancher/charts (release-v{{ (genRequest.rancherVersions?.[0] || genRequest.rancherVersion)?.replace(/^v/,'').split('.').slice(0,2).join('.') }})</code></dd>
              </dl>
            </div>
            <div class="details-section">
              <h4 class="details-heading">Distro docs</h4>
              <ul class="details-links">
                <li v-if="genRequest.distros.includes('k3s')"><a href="https://docs.k3s.io/" target="_blank" rel="noopener noreferrer">K3s</a></li>
                <li v-if="genRequest.distros.includes('rke2')"><a href="https://docs.rke2.io/" target="_blank" rel="noopener noreferrer">RKE2</a></li>
              </ul>
            </div>
            <div class="details-section details-section-docs">
              <h4 class="details-heading">GenesisRK docs</h4>
              <ul class="details-links">
                <li><a href="#docs">Overview</a></li>
                <li><a href="#docs/getting-started">Getting started</a></li>
                <li><a href="#docs/cli">CLI reference</a></li>
                <li><a href="#docs/api">REST API</a></li>
              </ul>
            </div>
          </aside>
        </div>
      </div>

      <div v-else-if="step === 'loading'" class="panel loading-panel">
        <div class="loading-panel-inner">
          <LoadingShapes size="md" />
          <LoadingProgressBar
            class="loading-panel-progress"
            centered
            :percent="loadingProgress.percent"
            :phase="loadingProgress.phase"
          />
        </div>
        <ServerLogsPanel
          class="loading-logs"
          centered
          :entries="formattedServerLogs"
          title="Server activity"
          empty-text="Waiting for server activity…"
          collapsible
          :default-open="false"
        />
      </div>

      <div v-else-if="step === 'step3' && genResponse" class="panel panel-fullscreen">
        <Step3Tree
          :roots="genResponse.roots"
          :basic-charts="genResponse.basicCharts"
          :basic-image-component="genResponse.basicImageComponent"
          :past-selection="genResponse.pastSelection"
          :components="genRequest.distros.join(',')"
          :distros="genRequest.distros"
          :cni-for-standard="genRequest.cni"
          :rancher-versions="genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])"
          :rke2-versions="genRequest.distros.includes('rke2') ? genRequest.rke2Versions : []"
          :k3s-versions="genRequest.distros.includes('k3s') ? genRequest.k3sVersions : []"
          :available-rancher-versions="availableRancherVersionIds"
          :available-k3s-versions="availableK3sVersionIds"
          :available-rke2-versions="availableRke2VersionIds"
          :load-balancers="selectedLoadBalancers"
          :include-windows="genRequest.includeWindows"
          :is-prime="genRequest.isRPMGC"
          :arch="genRequest.arch"
          :include-partner-charts="genRequest.includePartnerCharts"
          :include-ui-plugin-charts="genRequest.includeUIPluginCharts"
          v-model:destination-registry="genRequest.destinationRegistry"
          @export-list="runExport"
          @back="backToStep1"
        />
        <p v-if="exportError" class="error">{{ exportError }}</p>
      </div>
      </template>
    </main>

    <footer class="footer">
      <a href="#" class="footer-brand" title="GenesisRK home" @click.prevent="goBackToApp">
        <span class="footer-logo-mark" aria-hidden="true">
          <GenesisRootMark />
        </span>
        <span>GenesisRK</span>
      </a>
      <span class="footer-sep">·</span>
      <a href="https://github.com/cnrancher/hangar" target="_blank" rel="noopener noreferrer">Hangar</a>
      <span class="footer-sep">·</span>
      <a href="https://ranchermanager.docs.rancher.com/" target="_blank" rel="noopener noreferrer">Rancher Manager docs</a>
      <span class="footer-sep">·</span>
      <a href="https://docs.rke2.io/" target="_blank" rel="noopener noreferrer">RKE2 docs</a>
      <span class="footer-sep">·</span>
      <a href="https://docs.k3s.io/" target="_blank" rel="noopener noreferrer">K3s docs</a>
      <span class="footer-sep">·</span>
      <a href="https://github.com/aeltai" target="_blank" rel="noopener noreferrer">@aeltai</a>
    </footer>
  </div>
</template>

<style scoped>
.app {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  background: var(--bg);
  color: var(--text);
}
.hero {
  padding: 0.65rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
}
.hero-inner {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1rem;
  flex-wrap: nowrap;
  min-height: 3.25rem;
}
.hero-brand {
  display: flex;
  align-items: center;
  gap: 0.65rem;
  flex-shrink: 0;
  text-decoration: none;
  color: inherit;
}
.hero-brand:hover .hero-name {
  color: var(--accent);
}
.hero-logo-mark {
  width: 2.45rem;
  height: 2.45rem;
  flex-shrink: 0;
}
.hero-logo-mark :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}
.hero-brand-lockup {
  display: flex;
  align-items: center;
  margin: 0;
  font-size: inherit;
  font-weight: inherit;
  line-height: 1;
}
.hero-name {
  font-size: 1.125rem;
  font-weight: 700;
  letter-spacing: -0.025em;
  color: var(--text);
  transition: color 0.15s;
}
.hero-title {
  transition: color 0.15s;
  font-size: 1.0625rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--text);
  margin: 0;
  line-height: 1.2;
}
.hero-version {
  font-size: 0.6875rem;
  font-weight: 500;
  opacity: 0.65;
  color: var(--text);
  padding: 0.1rem 0.35rem;
  border-radius: 3px;
  border: 1px solid var(--border);
  background: var(--bg);
  line-height: 1.3;
}
.hero-actions {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-shrink: 0;
}
.hero-link {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  border-radius: 4px;
  background: transparent;
  border: 1px solid transparent;
  color: color-mix(in srgb, var(--text) 75%, transparent);
  text-decoration: none;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.hero-link:hover {
  border-color: var(--border);
  background: var(--bg);
  color: var(--text);
}
.hero-link.active {
  border-color: var(--border-strong);
  background: var(--bg);
  color: var(--text);
}
.theme-toggle {
  padding: 0.25rem 0.5rem;
  font-size: 0.75rem;
  font-weight: 500;
  border-radius: 4px;
  background: transparent;
  border: 1px solid var(--border);
  color: color-mix(in srgb, var(--text) 75%, transparent);
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s, background 0.15s;
}
.theme-toggle:hover {
  border-color: color-mix(in srgb, var(--cyan) 50%, var(--border));
  color: var(--text);
  background: var(--bg);
}
.docs-main {
  padding-top: 1.25rem;
}
.docs-page-title {
  margin: 0 0 1rem;
  font-size: 1.125rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  color: var(--text);
}
.docs-panel {
  margin: 0;
  padding: 0;
  flex: 1;
  width: 100%;
  max-width: none;
  border: none;
  border-radius: 0;
  background: transparent;
  overflow: hidden;
  min-height: calc(100vh - 108px - var(--footer-h));
}
.docs-body {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 1.25rem 1.5rem;
  font-size: 0.875rem;
  line-height: 1.6;
}
.docs-body h3 {
  color: var(--text);
  margin: 1.25rem 0 0.5rem;
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: 0.01em;
}
.docs-body h3:first-child {
  margin-top: 0;
}
.docs-body p {
  margin: 0.5rem 0;
}
.docs-body ul {
  margin: 0.5rem 0;
  padding-left: 1.25rem;
}
.docs-body li {
  margin: 0.25rem 0;
}
.docs-body code {
  background: var(--bg);
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 0.85em;
}
.docs-body a {
  color: var(--cyan);
}
.docs-table {
  width: 100%;
  border-collapse: collapse;
  margin: 0.5rem 0;
}
.docs-table td {
  padding: 0.5rem 0.75rem;
  border-bottom: 1px solid var(--border);
  vertical-align: top;
}
.docs-table td:first-child {
  white-space: nowrap;
  width: 180px;
}
.app-intro {
  padding: 0.625rem 0 0.875rem;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--text-muted);
}
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 0 1.5rem calc(var(--footer-h) + 0.5rem);
  width: 100%;
  max-width: none;
  margin: 0;
}
.main:has(.panel-fullscreen),
.main:has(.docs-panel) {
  padding-left: 0.75rem;
  padding-right: 0.75rem;
  max-width: none;
}

@media (min-width: 1440px) {
  .main:has(.docs-panel) {
    padding-left: 0;
    padding-right: 0;
  }
}
.panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 1.25rem 1.5rem;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.step1-panel {
  padding: 0;
  overflow: hidden;
  width: 100%;
  align-self: stretch;
}
.details-section-docs {
  margin-top: 0.5rem;
  padding-top: 0.75rem;
  border-top: 1px dashed var(--border);
}
.details-section-docs a {
  color: var(--accent);
  font-weight: 500;
}
.step1-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(300px, 360px);
  min-height: calc(100vh - 148px - var(--footer-h));
  align-items: stretch;
  width: 100%;
}
.step1-layout:not(:has(.step1-details)) {
  grid-template-columns: 1fr;
}
.step1-form {
  min-width: 0;
  padding: 1.5rem 1.75rem;
  overflow-y: auto;
}
.step1-details {
  padding: 1.25rem 1.5rem;
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
  border-left: 1px solid var(--border);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}
.details-section-spaced {
  margin-top: 0.25rem;
  padding-top: 0.75rem;
  border-top: 1px solid var(--border);
}
.details-hint {
  font-size: 0.78rem;
  color: var(--text-muted);
}
.details-title {
  margin: 0 0 0.75rem;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}
.details-heading {
  font-size: 0.75rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
  margin: 0 0 0.5rem 0;
}
.details-section {
  margin-bottom: 1.25rem;
}
.details-links {
  list-style: none;
  margin: 0;
  padding: 0;
}
.details-links li {
  margin-bottom: 0.35rem;
}
.details-links a {
  font-size: 0.9rem;
}
.details-dl {
  margin: 0;
  font-size: 0.85rem;
}
.details-dl dt {
  font-weight: 600;
  margin-top: 0.5rem;
  color: var(--text);
  opacity: 0.9;
}
.details-dl dt:first-child {
  margin-top: 0;
}
.details-dl dd {
  margin: 0.2rem 0 0 0;
}
.details-dl code {
  font-size: 0.8rem;
  word-break: break-all;
}
@media (max-width: 900px) {
  .step1-layout {
    grid-template-columns: 1fr;
    min-height: auto;
  }
  .step1-details {
    border-left: none;
    border-top: 1px solid var(--border);
  }
}
.panel-fullscreen {
  min-height: calc(100vh - 160px);
  display: flex;
  flex-direction: column;
}
.loading-panel {
  text-align: center;
  padding: 2rem 1.5rem;
  min-height: calc(100vh - 10rem);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}
.loading-panel-inner {
  width: min(480px, 100%);
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.65rem;
}
.loading-panel :deep(.loading-shapes) {
  --loader-size: 44px;
}
.loading-panel-progress {
  width: 100%;
}
.loading-logs {
  width: min(900px, 100%);
  margin-top: 1.25rem;
}
.error {
  color: var(--red);
  margin-top: 1rem;
  font-size: 0.9rem;
}
.footer {
  position: fixed;
  bottom: 0;
  left: 0;
  right: 0;
  z-index: 200;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 0 0.35rem;
  min-height: var(--footer-h);
  padding: 0.15rem 1.5rem;
  border-top: 1px solid var(--border);
  background: color-mix(in srgb, var(--panel) 94%, var(--bg));
  backdrop-filter: blur(8px);
  font-size: 0.6875rem;
  line-height: 1.2;
  text-align: center;
  color: var(--text-muted);
}
.footer-brand {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  text-decoration: none;
  color: var(--accent);
  font-weight: 600;
  margin-right: 0.15rem;
}
.footer-logo-mark {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 0.95rem;
  height: 0.95rem;
  flex-shrink: 0;
  opacity: 0.92;
}
.footer-logo-mark :deep(svg) {
  width: 100%;
  height: 100%;
  display: block;
}
.footer-brand:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}
.footer a {
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
}
.footer a:hover {
  color: var(--accent-hover);
  text-decoration: underline;
}
.footer-sep {
  margin: 0 0.15rem;
  opacity: 0.45;
  user-select: none;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .hero {
    padding: 0.45rem 1rem;
  }
  .hero-title {
    font-size: 1rem;
  }
  .hero-name {
    font-size: 1rem;
  }
  .hero-logo-mark {
    width: 2.1rem;
    height: 2.1rem;
  }
  .hero-actions {
    margin-left: auto;
  }
  .app-intro {
    padding: 0.5rem 1rem 0;
    font-size: 0.75rem;
  }
  .main {
    padding: 1rem;
  }
  .main:has(.panel-fullscreen) {
    padding: 0.5rem 0.75rem;
  }
  .panel {
    padding: 1rem;
  }
  .step1-form {
    padding: 1rem;
  }
  .step1-details {
    padding: 1rem 1.25rem;
  }
  .docs-main {
    padding-top: 1rem;
  }
  .docs-body {
    padding: 1rem;
  }
  .docs-table td:first-child {
    width: 120px;
  }
  .footer {
    padding: 0.15rem 0.75rem;
    font-size: 0.625rem;
  }
  .footer-sep {
    margin: 0 0.35rem;
  }
  .loading-panel {
    padding: 2rem 1rem;
  }
}

@media (max-width: 480px) {
  .hero {
    padding: 0.4rem 0.75rem;
  }
  .hero-inner {
    gap: 0.4rem;
  }
  .hero-title {
    font-size: 0.9375rem;
  }
  .hero-name {
    font-size: 0.9375rem;
  }
  .hero-logo-mark {
    width: 1.9rem;
    height: 1.9rem;
  }
  .hero-actions {
    gap: 0.25rem;
  }
  .hero-link,
  .theme-toggle {
    padding: 0.2rem 0.4rem;
    font-size: 0.6875rem;
  }
  .app-intro {
    padding: 0.4rem 0.75rem 0;
    font-size: 0.6875rem;
  }
  .main {
    padding: 0.75rem;
  }
  .main:has(.panel-fullscreen) {
    padding: 0.35rem 0.5rem;
  }
  .panel {
    padding: 0.75rem;
    border-radius: 6px;
  }
  .step1-form {
    padding: 0.75rem;
  }
  .step1-details {
    padding: 0.75rem 1rem;
  }
  .docs-main {
    padding-top: 0.75rem;
  }
  .docs-page-title {
    font-size: 1rem;
  }
  .docs-body {
    padding: 0.75rem;
    font-size: 0.85rem;
  }
  .docs-table {
    font-size: 0.85rem;
  }
  .docs-table td {
    padding: 0.4rem 0.5rem;
    display: block;
  }
  .docs-table td:first-child {
    width: 100%;
    font-weight: 600;
    padding-bottom: 0.15rem;
  }
  .docs-table tr {
    display: block;
    margin-bottom: 0.75rem;
    padding-bottom: 0.75rem;
    border-bottom: 1px solid var(--border);
  }
  .docs-table tr:last-child {
    border-bottom: none;
  }
  .footer {
    padding: 0.15rem 0.5rem;
    font-size: 0.625rem;
  }
  .footer a {
    display: inline;
    margin: 0;
  }
  .footer-sep {
    display: inline;
  }
  .loading-panel {
    padding: 1.5rem 0.75rem;
  }
}
</style>
