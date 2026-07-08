<script setup lang="ts">
import { ref, reactive, onMounted, watch, onUnmounted } from 'vue'
import { fetchRancherVersions, fetchStep1OptionsMerged, generate, exportImageList, exportHaulerManifest, fetchLogs, type RancherVersionInfo } from './api/genesis'
import type { Step1OptionsResponse, GenerateRequest, GenerateResponse } from './types/genesis'
import Step1Form from './components/Step1Form.vue'
import Step3Tree from './components/Step3Tree.vue'
import DocsViewer from './components/DocsViewer.vue'

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
const page = ref<'app' | 'docs'>(
  typeof window !== 'undefined' && (window.location.hash.slice(1) === 'docs' || window.location.hash.startsWith('#docs/'))
    ? 'docs'
    : 'app'
)
function updatePageFromHash() {
  const h = window.location.hash.slice(1)
  page.value = h === 'docs' || h.startsWith('docs/') ? 'docs' : 'app'
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
  rancherVersion: 'v2.13.1',
  rancherVersions: ['v2.13.1'],
  isRPMGC: false,
  includeAppCollectionCharts: false,
  appCollectionAPIUser: '',
  appCollectionAPIPassword: '',
  distros: ['rke2'],
  cni: 'cni_calico',
  loadBalancer: true,
  lbK3sKlipper: false,
  lbK3sTraefik: false,
  lbRKE2Nginx: true,
  lbRKE2Traefik: false,
  includeWindows: false,
  k3sVersions: ['all'],
  rke2Versions: ['all'],
  rkeVersions: ['all'],
  destinationRegistry: '',
  destinationRegistryUser: '',
  destinationRegistryPassword: '',
})

const genResponse = ref<GenerateResponse | null>(null)
const genError = ref('')
const exportError = ref('')
const showLogs = ref(false)
const serverLogs = ref<string[]>([])
const logsContentRef = ref<HTMLElement | null>(null)
let logsPollTimer: ReturnType<typeof setInterval> | null = null

function startLogsPoll() {
  if (logsPollTimer) return
  async function poll() {
    try {
      serverLogs.value = await fetchLogs()
    } catch {
      // ignore
    }
  }
  poll()
  logsPollTimer = setInterval(poll, 1500)
}

function stopLogsPoll() {
  if (logsPollTimer) {
    clearInterval(logsPollTimer)
    logsPollTimer = null
  }
}

watch(
  () => [step.value, showLogs.value] as const,
  ([s, show]) => {
    if (s === 'loading' && show) startLogsPoll()
    else stopLogsPoll()
  }
)
watch(serverLogs, () => {
  if (logsContentRef.value) logsContentRef.value.scrollTop = logsContentRef.value.scrollHeight
})
onUnmounted(stopLogsPoll)

const optionsLoading = ref(false)
let loadAbort: AbortController | null = null

async function loadRancherVersions() {
  try {
    rancherVersions.value = await fetchRancherVersions(includeRC.value)
  } catch { /* ignore */ }
}

async function loadOptions() {
  if (loadAbort) loadAbort.abort()
  loadAbort = new AbortController()
  step1Error.value = ''
  optionsLoading.value = true
  const versions = genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion]
  try {
    step1Options.value = await fetchStep1OptionsMerged(versions, includeRC.value, includeGitHubVersions.value)
  } catch (e) {
    if ((e as Error).name === 'AbortError') return
    step1Error.value = e instanceof Error ? e.message : String(e)
  } finally {
    optionsLoading.value = false
  }
}

watch(() => [genRequest.rancherVersion, genRequest.rancherVersions], () => { loadOptions() }, { deep: true })
watch(includeRC, async () => {
  await loadRancherVersions()
  loadOptions()
})
watch(includeGitHubVersions, () => { loadOptions() })

onMounted(async () => {
  await loadRancherVersions()
  loadOptions()
})

async function runGenerate() {
  genError.value = ''
  const versions = genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])
  if (!versions.length) {
    genError.value = 'Select at least one Rancher version.'
    return
  }
  step.value = 'loading'
  genRequest.loadBalancer = genRequest.lbK3sKlipper || genRequest.lbK3sTraefik || genRequest.lbRKE2Nginx || genRequest.lbRKE2Traefik
  try {
    genResponse.value = await generate(genRequest)
    step.value = 'step3'
  } catch (e) {
    genError.value = e instanceof Error ? e.message : String(e)
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

async function runExportHauler(
  selectedComponentIDs: string[],
  chartNames: string[],
  selectedImageRefs: string[]
) {
  if (!genResponse.value) return
  exportError.value = ''
  try {
    const blob = await exportHaulerManifest({
      jobId: genResponse.value.jobId,
      selectedComponentIDs,
      chartNames,
      selectedImageRefs,
    })
    const a = document.createElement('a')
    a.href = URL.createObjectURL(blob)
    a.download = 'hauler-manifest.yaml'
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
    <template v-if="page === 'docs'">
      <DocsViewer @back="goBackToApp" />
    </template>

    <template v-else>
    <header class="topbar">
      <div class="topbar-start">
        <div class="topbar-brand">
          <h1 class="topbar-title">GenesisRK</h1>
          <span class="topbar-version">v{{ VERSION }}</span>
        </div>
        <p class="topbar-tagline">
          Build image lists for air-gapped Rancher — select versions, distros, CNI, and charts, then export.
        </p>
      </div>
      <div class="topbar-actions">
        <a href="https://github.com/aeltai/Hangar-Genesis" target="_blank" rel="noopener noreferrer" class="topbar-btn" title="GitHub Repository">
          <svg viewBox="0 0 16 16" width="16" height="16" fill="currentColor"><path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.013 8.013 0 0016 8c0-4.42-3.58-8-8-8z"/></svg>
          GitHub
        </a>
        <a href="#docs" class="topbar-btn" title="Documentation">Docs</a>
        <button type="button" class="topbar-btn" :title="theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'" @click="toggleTheme">
          {{ theme === 'dark' ? 'Light' : 'Dark' }}
        </button>
      </div>
    </header>
    <main class="main">
      <div v-if="step === 'step1'" class="panel step1-panel">
        <div class="step1-layout">
          <div class="step1-form">
            <Step1Form
              :available-rancher-versions="rancherVersions"
              v-model:rancher-version="genRequest.rancherVersion"
              v-model:rancher-versions="genRequest.rancherVersions"
              v-model:include-r-c="includeRC"
              v-model:include-git-hub-versions="includeGitHubVersions"
              v-model:is-rpm-gc="genRequest.isRPMGC"
              v-model:include-app-collection="genRequest.includeAppCollectionCharts"
              v-model:app-user="genRequest.appCollectionAPIUser"
              v-model:app-password="genRequest.appCollectionAPIPassword"
              v-model:distros="genRequest.distros"
              v-model:cni="genRequest.cni"
              v-model:lb-k3s-klipper="genRequest.lbK3sKlipper"
              v-model:lb-k3s-traefik="genRequest.lbK3sTraefik"
              v-model:lb-rke2-nginx="genRequest.lbRKE2Nginx"
              v-model:lb-rke2-traefik="genRequest.lbRKE2Traefik"
              v-model:include-windows="genRequest.includeWindows"
              v-model:k3s-versions="genRequest.k3sVersions"
              v-model:rke2-versions="genRequest.rke2Versions"
              v-model:rke-versions="genRequest.rkeVersions"
              :options="step1Options"
              :load-error="step1Error"
              :options-loading="optionsLoading"
              @generate="runGenerate"
            />
            <p v-if="genError" class="error">{{ genError }}</p>
          </div>
          <aside v-if="(genRequest.rancherVersions?.length || genRequest.rancherVersion) && step1Options" class="step1-details">
            <h3 class="details-title">Details for this configuration</h3>
            <div class="details-section">
              <h4 class="details-heading">Rancher</h4>
              <ul class="details-links">
                <li v-for="v in (genRequest.rancherVersions?.length ? genRequest.rancherVersions : [genRequest.rancherVersion])" :key="v">
                  <a :href="'https://github.com/rancher/rancher/releases/tag/' + v" target="_blank" rel="noopener noreferrer">Release {{ v }}</a>
                </li>
                <li><a href="https://ranchermanager.docs.rancher.com/releases" target="_blank" rel="noopener noreferrer">Rancher release notes</a></li>
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
                <li v-if="genRequest.distros.includes('rke')"><a href="https://rke.docs.rancher.com/" target="_blank" rel="noopener noreferrer">RKE1</a></li>
              </ul>
            </div>
          </aside>
        </div>
      </div>

      <div v-else-if="step === 'loading'" class="panel loading-panel">
        <p class="loading-text">Generating tree from KDM and charts…</p>
        <p class="loading-hint">This may take a minute.</p>
        <div class="loading-logs">
          <button type="button" class="logs-toggle" @click="showLogs = !showLogs">
            {{ showLogs ? 'Hide logs' : 'Show logs' }}
          </button>
          <div v-show="showLogs" class="logs-viewer">
            <pre ref="logsContentRef" class="logs-content">{{ serverLogs.length ? serverLogs.join('\n') : 'Waiting for server logs…' }}</pre>
          </div>
        </div>
      </div>

      <div v-else-if="step === 'step3' && genResponse" class="panel panel-fullscreen">
        <Step3Tree
          :job-id="genResponse.jobId"
          :roots="genResponse.roots"
          :basic-charts="genResponse.basicCharts"
          :basic-image-component="genResponse.basicImageComponent"
          :past-selection="genResponse.pastSelection"
          :components="genRequest.distros.join(',')"
          :cni-for-standard="genRequest.cni"
          :rancher-versions="genRequest.rancherVersions?.length ? genRequest.rancherVersions : (genRequest.rancherVersion ? [genRequest.rancherVersion] : [])"
          :rke2-versions="genRequest.distros.includes('rke2') ? genRequest.rke2Versions : []"
          :k3s-versions="genRequest.distros.includes('k3s') ? genRequest.k3sVersions : []"
          v-model:destination-registry="genRequest.destinationRegistry"
          v-model:destination-registry-user="genRequest.destinationRegistryUser"
          v-model:destination-registry-password="genRequest.destinationRegistryPassword"
          @export-list="runExport"
          @export-hauler="runExportHauler"
          @back="backToStep1"
        />
        <p v-if="exportError" class="error">{{ exportError }}</p>
      </div>
    </main>

    <footer class="footer">
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
    </template>
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
.topbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 1.5rem;
  padding: 0.65rem 1.5rem;
  border-bottom: 1px solid var(--border);
  background: var(--panel);
  position: sticky;
  top: 0;
  z-index: 100;
  box-shadow: 0 1px 0 color-mix(in srgb, var(--border) 50%, transparent);
}
.topbar::before {
  content: '';
  position: absolute;
  left: 0;
  top: 0;
  bottom: 0;
  width: 3px;
  background: linear-gradient(180deg, var(--cyan), var(--green));
}
.topbar-start {
  display: flex;
  align-items: center;
  gap: 1.25rem;
  min-width: 0;
  flex: 1;
}
.topbar-brand {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-shrink: 0;
}
.topbar-title {
  font-size: 1.15rem;
  font-weight: 700;
  letter-spacing: -0.02em;
  color: var(--cyan);
  margin: 0;
}
.topbar-version {
  font-size: 0.7rem;
  font-weight: 600;
  opacity: 0.75;
  padding: 0.15rem 0.4rem;
  border-radius: 4px;
  background: color-mix(in srgb, var(--border) 35%, transparent);
}
.topbar-tagline {
  margin: 0;
  font-size: 0.82rem;
  opacity: 0.72;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.topbar-actions {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-shrink: 0;
}
.topbar-btn {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0.35rem 0.65rem;
  font-size: 0.8rem;
  font-weight: 600;
  border-radius: 6px;
  background: var(--bg);
  border: 1px solid var(--border);
  color: var(--text);
  text-decoration: none;
  cursor: pointer;
  transition: border-color 0.15s, color 0.15s;
}
.topbar-btn:hover {
  border-color: var(--cyan);
  color: var(--cyan);
}
.docs-page {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  padding: 0 2rem 2rem;
}
.docs-page-header {
  padding: 1.5rem 0 1rem;
  border-bottom: 1px solid var(--border);
  margin-bottom: 1.5rem;
}
.docs-back {
  display: inline-block;
  font-size: 0.9rem;
  color: var(--cyan);
  text-decoration: none;
  margin-bottom: 0.75rem;
}
.docs-back:hover {
  text-decoration: underline;
}
.docs-page-title {
  margin: 0;
  font-size: 1.75rem;
  font-weight: 700;
  color: var(--cyan);
}
.docs-panel {
  max-width: 900px;
  margin: 0 auto;
  padding: 0;
  flex: 1;
}
.docs-body {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1.5rem;
  font-size: 0.9rem;
  line-height: 1.6;
}
.docs-body h3 {
  color: var(--green, #22c55e);
  margin: 1.25rem 0 0.5rem;
  font-size: 1rem;
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
.main {
  flex: 1;
  display: flex;
  flex-direction: column;
  padding: 1rem 1.25rem 1.25rem;
  width: 100%;
  min-height: 0;
}
.main:has(.panel-fullscreen) {
  padding: 0.5rem 0.75rem;
}
.panel {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 10px;
  padding: 1.25rem;
  flex: 1;
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.step1-panel {
  padding: 0;
  overflow: hidden;
}
.step1-layout {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(260px, 320px);
  flex: 1;
  min-height: calc(100vh - 130px);
  align-items: stretch;
}
.step1-form {
  min-width: 0;
  padding: 1.25rem 1.5rem;
  overflow-y: auto;
}
.step1-details {
  padding: 1.25rem 1.25rem;
  background: color-mix(in srgb, var(--bg) 50%, var(--panel));
  border-left: 1px solid var(--border);
  overflow-y: auto;
}
.details-title {
  font-size: 0.85rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--cyan);
  margin: 0 0 1rem 0;
  padding-bottom: 0.65rem;
  border-bottom: 1px solid var(--border);
}
.details-section {
  margin-bottom: 1rem;
}
.details-heading {
  font-size: 0.72rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text);
  opacity: 0.65;
  margin: 0 0 0.4rem 0;
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
@media (max-width: 1100px) {
  .topbar-tagline {
    display: none;
  }
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
  padding: 3rem;
}
.loading-text {
  font-size: 1.1rem;
  margin-bottom: 0.5rem;
}
.loading-hint {
  opacity: 0.7;
  font-size: 0.9rem;
}
.loading-logs {
  margin-top: 1.5rem;
  text-align: left;
  max-width: 900px;
  margin-left: auto;
  margin-right: auto;
}
.logs-toggle {
  margin-bottom: 0.5rem;
}
.logs-viewer {
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  overflow: hidden;
}
.logs-content {
  margin: 0;
  padding: 0.75rem 1rem;
  font-size: 0.8rem;
  line-height: 1.4;
  white-space: pre-wrap;
  word-break: break-all;
  max-height: 320px;
  overflow: auto;
}
.error {
  color: var(--red);
  margin-top: 1rem;
  font-size: 0.9rem;
}
.footer {
  margin-top: auto;
  padding: 1rem 2rem;
  border-top: 1px solid var(--border);
  background: var(--panel);
  font-size: 0.85rem;
  text-align: center;
  color: var(--text);
  opacity: 0.9;
}
.footer a {
  color: var(--cyan);
  text-decoration: none;
}
.footer a:hover {
  text-decoration: underline;
}
.footer-sep {
  margin: 0 0.5rem;
  opacity: 0.6;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .topbar {
    flex-wrap: wrap;
    padding: 0.6rem 1rem;
  }
  .topbar-start {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.25rem;
  }
  .topbar-actions {
    width: 100%;
    justify-content: flex-end;
  }
  .main {
    padding: 0.75rem;
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
  .docs-page {
    padding: 0 1rem 1.5rem;
  }
  .docs-body {
    padding: 1rem;
  }
  .docs-table td:first-child {
    width: 120px;
  }
  .footer {
    padding: 0.75rem 1rem;
    font-size: 0.8rem;
  }
  .footer-sep {
    margin: 0 0.35rem;
  }
  .loading-panel {
    padding: 2rem 1rem;
  }
  .logs-content {
    max-height: 240px;
  }
}

@media (max-width: 480px) {
  .topbar {
    padding: 0.5rem 0.75rem;
  }
  .topbar-title {
    font-size: 1rem;
  }
  .topbar-btn {
    padding: 0.3rem 0.5rem;
    font-size: 0.75rem;
  }
  .main {
    padding: 0.5rem;
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
  .docs-page {
    padding: 0 0.75rem 1rem;
  }
  .docs-page-title {
    font-size: 1.4rem;
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
    padding: 0.5rem 0.75rem;
    font-size: 0.75rem;
  }
  .footer a {
    display: inline-block;
    margin: 0.1rem 0;
  }
  .footer-sep {
    display: inline;
  }
  .loading-panel {
    padding: 1.5rem 0.75rem;
  }
  .loading-text {
    font-size: 1rem;
  }
  .logs-content {
    max-height: 180px;
    font-size: 0.75rem;
  }
}
</style>
