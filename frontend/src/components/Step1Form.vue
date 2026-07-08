<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { Step1OptionsResponse } from '../types/genesis'
import type { RancherVersionInfo } from '../api/genesis'

const props = defineProps<{
  availableRancherVersions: RancherVersionInfo[]
  options: Step1OptionsResponse | null
  loadError: string
  optionsLoading: boolean
}>()

defineEmits<{
  generate: []
}>()

const rancherVersion = defineModel<string>('rancherVersion', { default: 'v2.13.1' })
const rancherVersions = defineModel<string[]>('rancherVersions', { default: () => ['v2.13.1'] })
const isRPMGC = defineModel<boolean>('isRPMGC', { default: false })
const includeAppCollection = defineModel<boolean>('includeAppCollection', { default: false })
const appUser = defineModel<string>('appUser', { default: '' })
const appPassword = defineModel<string>('appPassword', { default: '' })
const distros = defineModel<string[]>('distros', { default: () => ['rke2'] })
const cni = defineModel<string>('cni', { default: 'cni_calico' })
const lbK3sKlipper = defineModel<boolean>('lbK3sKlipper', { default: false })
const lbK3sTraefik = defineModel<boolean>('lbK3sTraefik', { default: false })
const lbRKE2Nginx = defineModel<boolean>('lbRKE2Nginx', { default: true })
const lbRKE2Traefik = defineModel<boolean>('lbRKE2Traefik', { default: false })
const includeRC = defineModel<boolean>('includeRC', { default: false })
const includeGitHubVersions = defineModel<boolean>('includeGitHubVersions', { default: false })
const includeWindows = defineModel<boolean>('includeWindows', { default: false })
const k3sVersions = defineModel<string[]>('k3sVersions', { default: () => ['all'] })
const rke2Versions = defineModel<string[]>('rke2Versions', { default: () => ['all'] })
const rkeVersions = defineModel<string[]>('rkeVersions', { default: () => ['all'] })

// CNI options by distro: K3s default is Flannel; RKE2 defaults to Canal and supports Calico, Cilium, Flannel; RKE1 uses Canal/Calico/Flannel
const cniOptions = computed(() => {
  const d = distros.value
  const hasK3s = d.includes('k3s')
  const hasRKE2 = d.includes('rke2')
  const hasRKE1 = d.includes('rke')
  const onlyK3s = d.length === 1 && d[0] === 'k3s'
  const onlyRKE2 = d.length === 1 && d[0] === 'rke2'
  const onlyRKE1 = d.length === 1 && d[0] === 'rke'

  if (onlyK3s) {
    return [
      { id: 'cni_flannel', label: 'Flannel', hint: 'K3s default' },
      { id: 'cni_canal', label: 'Canal', hint: 'custom' },
      { id: 'cni_calico', label: 'Calico', hint: 'custom' },
      { id: 'cni_cilium', label: 'Cilium', hint: 'custom' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  if (onlyRKE2) {
    return [
      { id: 'cni_canal', label: 'Canal', hint: 'RKE2 default' },
      { id: 'cni_calico', label: 'Calico' },
      { id: 'cni_cilium', label: 'Cilium' },
      { id: 'cni_flannel', label: 'Flannel' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  if (onlyRKE1) {
    return [
      { id: 'cni_canal', label: 'Canal', hint: 'RKE1 common' },
      { id: 'cni_calico', label: 'Calico' },
      { id: 'cni_flannel', label: 'Flannel' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  const base: { id: string; label: string; hint?: string }[] = []
  if (hasK3s) base.push({ id: 'cni_flannel', label: 'Flannel', hint: 'K3s default' })
  if (hasRKE2 || hasRKE1) base.push({ id: 'cni_canal', label: 'Canal', hint: hasRKE2 ? 'RKE2 default' : undefined })
  if (hasRKE2 || hasRKE1) base.push({ id: 'cni_calico', label: 'Calico' })
  if (hasRKE2) base.push({ id: 'cni_cilium', label: 'Cilium' })
  if (hasRKE1 && !base.some((x) => x.id === 'cni_flannel')) base.push({ id: 'cni_flannel', label: 'Flannel' })
  base.push({ id: 'cni', label: 'All CNI' }, { id: '', label: 'None' })
  return base
})

// When distros change, reset CNI and LB options for deselected distros
watch(
  () => [distros.value, cniOptions.value] as const,
  () => {
    const opts = cniOptions.value
    const valid = opts.some((o) => o.id === cni.value)
    const first = opts[0]
    if (!valid && first) {
      cni.value = first.id
    }
    const d = distros.value
    if (!d.includes('k3s')) {
      lbK3sKlipper.value = false
      lbK3sTraefik.value = false
    }
    if (!d.includes('rke2')) {
      lbRKE2Nginx.value = false
      lbRKE2Traefik.value = false
    }
  },
  { immediate: true }
)

function toggleVersion(arr: string[], v: string, setter: (val: string[]) => void) {
  const idx = arr.indexOf(v)
  if (idx >= 0) setter(arr.filter(x => x !== v))
  else setter([...arr, v])
}

function versionSource(distro: string, v: string): string {
  const cap = props.options?.capabilities?.[distro]
  return cap?.sources?.[v] || 'kdm'
}

function toggleDistro(d: string) {
  const i = distros.value.indexOf(d)
  if (i >= 0) {
    distros.value = distros.value.filter((x) => x !== d)
  } else {
    distros.value = [...distros.value, d]
    if (d === 'k3s') { lbK3sKlipper.value = true; lbK3sTraefik.value = true }
    if (d === 'rke2') { lbRKE2Nginx.value = true; lbRKE2Traefik.value = false }
  }
}

const rancherVersionDropdownOpen = ref(false)

function toggleRancherVersion(version: string) {
  const arr = rancherVersions.value ?? []
  const i = arr.indexOf(version)
  if (i >= 0) {
    rancherVersions.value = arr.filter((x) => x !== version)
  } else {
    rancherVersions.value = [...arr, version].sort()
  }
  rancherVersion.value = rancherVersions.value[0] ?? ''
}

function closeRancherDropdown(e: Event) {
  const target = e.target as Node
  if (rancherVersionDropdownOpen.value && !(document.querySelector('.rancher-version-dropdown')?.contains(target))) {
    rancherVersionDropdownOpen.value = false
  }
}

const rancherVersionSummary = computed(() => {
  const sel = rancherVersions.value
  if (!sel?.length) return 'Select version(s)'
  if (sel.length === 1) return sel[0]
  return `${sel.length} versions`
})

onMounted(() => {
  document.addEventListener('click', closeRancherDropdown)
})
onUnmounted(() => {
  document.removeEventListener('click', closeRancherDropdown)
})
</script>

<template>
  <div class="step1">
    <header class="step-header">
      <h2 class="step-title">Step 1: Source &amp; options</h2>
      <p class="step-desc">Configure Rancher versions, distros, and network options before generating the image tree.</p>
    </header>

    <div class="form-grid">
      <!-- Rancher versions -->
      <section class="form-section">
        <h3 class="section-title">Rancher versions</h3>
        <div class="field-stack">
          <div class="field rancher-version-field">
            <label>Version(s)</label>
            <p class="field-hint">Select one or more; the image list includes all selected versions.</p>
            <template v-if="availableRancherVersions?.length > 0">
              <div class="rancher-version-dropdown">
                <button
                  type="button"
                  class="rancher-version-trigger"
                  :class="{ open: rancherVersionDropdownOpen }"
                  @click.stop="rancherVersionDropdownOpen = !rancherVersionDropdownOpen"
                >
                  <span class="trigger-text">{{ rancherVersionSummary }}</span>
                  <span class="trigger-arrow">{{ rancherVersionDropdownOpen ? '▲' : '▼' }}</span>
                </button>
                <div v-show="rancherVersionDropdownOpen" class="rancher-version-panel">
                  <div class="rancher-version-list">
                    <label
                      v-for="rv in availableRancherVersions"
                      :key="rv.version"
                      class="rancher-version-option"
                    >
                      <input
                        type="checkbox"
                        :checked="rancherVersions.includes(rv.version)"
                        @change="toggleRancherVersion(rv.version)"
                      />
                      <span class="option-version">{{ rv.version }}</span>
                      <span v-if="rv.date" class="option-date">{{ rv.date }}</span>
                    </label>
                  </div>
                </div>
              </div>
            </template>
            <input
              v-else
              v-model="rancherVersion"
              type="text"
              placeholder="v2.13.1"
              class="input"
            />
            <span v-if="optionsLoading" class="loading-indicator">Loading…</span>
          </div>
          <label class="check option-toggle">
            <input v-model="includeGitHubVersions" type="checkbox" />
            Include versions from GitHub (K3s/RKE2 release tags)
          </label>
          <label v-if="includeGitHubVersions" class="check option-toggle nested">
            <input v-model="includeRC" type="checkbox" />
            Include pre-release (RC/alpha/beta) versions
          </label>
          <p v-if="loadError" class="error-msg">{{ loadError }}</p>
        </div>
      </section>

      <!-- Image source -->
      <section class="form-section">
        <h3 class="section-title">
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon" />
          Image source
        </h3>
        <div class="field-stack">
          <div class="radio-group stacked">
            <label class="radio">
              <input v-model="isRPMGC" type="radio" :value="false" />
              <span>
                <strong>Community</strong>
                <span class="source-detail">GitHub releases (k3s-io/k3s, rancher/rke2)</span>
              </span>
            </label>
            <label class="radio">
              <input v-model="isRPMGC" type="radio" :value="true" />
              <span>
                <strong>Rancher Prime</strong>
                <span class="source-detail">prime.ribs.rancher.io (curated/certified)</span>
              </span>
            </label>
          </div>
          <p class="source-note">Both use KDM (releases.rancher.com) and rancher/charts on GitHub.</p>
          <label class="check option-toggle">
            <input v-model="includeAppCollection" type="checkbox" />
            Include Application Collection (dp.apps.rancher.io)
          </label>
          <div v-if="includeAppCollection" class="inline-inputs">
            <input v-model="appUser" type="text" placeholder="API username" class="input" />
            <input v-model="appPassword" type="password" placeholder="API password/token" class="input" />
          </div>
        </div>
      </section>

      <!-- Distros & platform -->
      <section class="form-section">
        <h3 class="section-title">Distros &amp; platform</h3>
        <div class="field-stack">
          <div class="check-group distros-group">
            <label class="check chip-check">
              <input type="checkbox" :checked="distros.includes('k3s')" @change="toggleDistro('k3s')" />
              <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/k3s.svg" alt="" class="ctx-icon ctx-icon-sm" />
              K3s
            </label>
            <label class="check chip-check">
              <input type="checkbox" :checked="distros.includes('rke2')" @change="toggleDistro('rke2')" />
              <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon ctx-icon-sm" />
              RKE2
            </label>
            <label v-if="options?.hasRKE1" class="check chip-check">
              <input type="checkbox" :checked="distros.includes('rke')" @change="toggleDistro('rke')" />
              RKE1
            </label>
          </div>
          <div class="sub-field">
            <label class="sub-label">Platform</label>
            <div class="radio-group inline">
              <label class="radio"><input v-model="includeWindows" type="radio" :value="false" /> Linux only</label>
              <label class="radio"><input v-model="includeWindows" type="radio" :value="true" /> Linux + Windows</label>
            </div>
          </div>
        </div>
      </section>

      <!-- Network -->
      <section v-if="distros.length > 0" class="form-section">
        <h3 class="section-title">
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/nginx.svg" alt="" class="ctx-icon" />
          Network
        </h3>
        <div class="field-stack">
          <div class="sub-field">
            <label class="sub-label">CNI</label>
            <select v-model="cni" class="input select">
              <option v-for="o in cniOptions" :key="o.id" :value="o.id">{{ o.label }}{{ o.hint ? ' (' + o.hint + ')' : '' }}</option>
            </select>
          </div>
          <div class="sub-field">
            <label class="sub-label">Load balancer / Ingress</label>
            <div class="check-group lb-options">
              <template v-if="distros.includes('k3s')">
                <label class="check"><input v-model="lbK3sKlipper" type="checkbox" /> K3s Klipper</label>
                <label class="check"><input v-model="lbK3sTraefik" type="checkbox" /> K3s Traefik</label>
              </template>
              <template v-if="distros.includes('rke2')">
                <label class="check"><input v-model="lbRKE2Nginx" type="checkbox" /> RKE2 NGINX</label>
                <label class="check"><input v-model="lbRKE2Traefik" type="checkbox" /> RKE2 Traefik</label>
              </template>
            </div>
          </div>
        </div>
      </section>

      <!-- Kubernetes versions (full width) -->
      <section v-if="options?.capabilities" class="form-section form-section-wide">
        <h3 class="section-title">
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/kubernetes.svg" alt="" class="ctx-icon" />
          Kubernetes versions
        </h3>
        <div class="field-stack">
          <p v-if="!includeGitHubVersions" class="version-gh-hint">
            Only KDM-supported versions shown. Enable <strong>Include versions from GitHub</strong> to add newer K3s/RKE2 releases.
          </p>
          <div class="version-legend">
            <span class="version-legend-item"><span class="legend-swatch swatch-kdm"></span> KDM (Rancher supported)</span>
            <span v-if="includeGitHubVersions" class="version-legend-item"><span class="legend-swatch swatch-gh"></span> GitHub release (newer)</span>
          </div>
          <div class="version-grid">
            <div v-if="distros.includes('k3s') && options?.capabilities?.k3s" class="version-block">
              <div class="version-header">
                <span class="version-label">K3s</span>
                <label class="check version-all">
                  <input type="checkbox" :checked="k3sVersions.includes('all')" @change="k3sVersions = k3sVersions.includes('all') ? [] : ['all']" />
                  All
                </label>
              </div>
              <div v-if="!k3sVersions.includes('all')" class="version-chips">
                <label v-for="v in (options?.capabilities?.k3s?.versions ?? [])" :key="v" class="version-chip" :class="{ active: k3sVersions.includes(v), 'chip-github': versionSource('k3s', v) === 'github', 'chip-kdm': versionSource('k3s', v) === 'kdm' || versionSource('k3s', v) === 'both' }">
                  <input type="checkbox" :checked="k3sVersions.includes(v)" @change="toggleVersion(k3sVersions, v, val => k3sVersions = val)" hidden />
                  {{ v }}
                  <span v-if="versionSource('k3s', v) === 'github'" class="chip-badge" title="From GitHub releases">GH</span>
                </label>
              </div>
            </div>
            <div v-if="distros.includes('rke2') && options?.capabilities?.rke2" class="version-block">
              <div class="version-header">
                <span class="version-label">RKE2</span>
                <label class="check version-all">
                  <input type="checkbox" :checked="rke2Versions.includes('all')" @change="rke2Versions = rke2Versions.includes('all') ? [] : ['all']" />
                  All
                </label>
              </div>
              <div v-if="!rke2Versions.includes('all')" class="version-chips">
                <label v-for="v in (options?.capabilities?.rke2?.versions ?? [])" :key="v" class="version-chip" :class="{ active: rke2Versions.includes(v), 'chip-github': versionSource('rke2', v) === 'github', 'chip-kdm': versionSource('rke2', v) === 'kdm' || versionSource('rke2', v) === 'both' }">
                  <input type="checkbox" :checked="rke2Versions.includes(v)" @change="toggleVersion(rke2Versions, v, val => rke2Versions = val)" hidden />
                  {{ v }}
                  <span v-if="versionSource('rke2', v) === 'github'" class="chip-badge" title="From GitHub releases">GH</span>
                </label>
              </div>
            </div>
            <div v-if="distros.includes('rke') && options?.capabilities?.rke" class="version-block">
              <div class="version-header">
                <span class="version-label">RKE1</span>
                <label class="check version-all">
                  <input type="checkbox" :checked="rkeVersions.includes('all')" @change="rkeVersions = rkeVersions.includes('all') ? [] : ['all']" />
                  All
                </label>
              </div>
              <div v-if="!rkeVersions.includes('all')" class="version-chips">
                <label v-for="v in (options?.capabilities?.rke?.versions ?? [])" :key="v" class="version-chip" :class="{ active: rkeVersions.includes(v) }">
                  <input type="checkbox" :checked="rkeVersions.includes(v)" @change="toggleVersion(rkeVersions, v, val => rkeVersions = val)" hidden />
                  {{ v }}
                </label>
              </div>
            </div>
          </div>
        </div>
      </section>
    </div>

    <div class="actions">
      <button type="button" class="btn btn-primary" @click="$emit('generate')">Generate image tree</button>
    </div>
  </div>
</template>

<style scoped>
.step1 {
  display: flex;
  flex-direction: column;
  gap: 1rem;
  height: 100%;
}
.step-header {
  margin-bottom: 0.25rem;
}
.step-title {
  font-size: 1.1rem;
  font-weight: 700;
  color: var(--cyan);
  margin: 0 0 0.25rem 0;
}
.step-desc {
  margin: 0;
  font-size: 0.82rem;
  opacity: 0.7;
}
.form-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 0.85rem;
  flex: 1;
}
.form-section {
  background: color-mix(in srgb, var(--bg) 40%, var(--panel));
  border: 1px solid var(--border);
  border-radius: 8px;
  padding: 1rem 1.1rem;
  min-width: 0;
}
.form-section-wide {
  grid-column: 1 / -1;
}
.section-title {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 0.78rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text);
  opacity: 0.85;
  margin: 0 0 0.75rem 0;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 70%, transparent);
}
.field-stack {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}
.field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.field label {
  font-size: 0.85rem;
  font-weight: 600;
}
.field-hint {
  margin: 0;
  font-size: 0.78rem;
  opacity: 0.65;
}
.input {
  padding: 0.45rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  color: var(--text);
  font-size: 0.88rem;
  width: 100%;
}
.select {
  max-width: 280px;
}
.inline-inputs {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.5rem;
}
.rancher-version-dropdown {
  position: relative;
  width: 100%;
  max-width: 360px;
}
.rancher-version-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 0.88rem;
  font-family: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  cursor: pointer;
  text-align: left;
}
.rancher-version-trigger:hover,
.rancher-version-trigger.open {
  border-color: var(--cyan);
}
.trigger-arrow {
  font-size: 0.65rem;
  opacity: 0.7;
  margin-left: 0.5rem;
}
.rancher-version-panel {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  margin-top: 4px;
  max-height: 260px;
  overflow-y: auto;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 6px;
  box-shadow: 0 6px 20px rgba(0, 0, 0, 0.2);
  z-index: 50;
}
.rancher-version-list {
  padding: 0.3rem 0;
}
.rancher-version-option {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 0.4rem 0.75rem;
  font-size: 0.85rem;
  cursor: pointer;
  user-select: none;
}
.rancher-version-option:hover {
  background: color-mix(in srgb, var(--cyan) 12%, transparent);
}
.option-version {
  font-weight: 600;
}
.option-date {
  font-size: 0.75rem;
  opacity: 0.7;
  margin-left: auto;
}
.ctx-icon {
  width: 18px;
  height: 18px;
  object-fit: contain;
  flex-shrink: 0;
  filter: invert(1);
}
.ctx-icon-sm {
  width: 16px;
  height: 16px;
}
[data-theme="light"] .ctx-icon {
  filter: none;
}
.option-toggle {
  font-size: 0.82rem;
  opacity: 0.88;
}
.option-toggle.nested {
  margin-left: 1.25rem;
  opacity: 0.8;
}
.check-group {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
}
.chip-check {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  padding: 0.35rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: 6px;
  background: var(--bg);
  font-size: 0.85rem;
  cursor: pointer;
  transition: border-color 0.15s;
}
.chip-check:has(input:checked) {
  border-color: var(--cyan);
  background: color-mix(in srgb, var(--cyan) 10%, var(--bg));
}
.radio-group.stacked {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.radio-group.inline {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.radio,
.check {
  display: inline-flex;
  align-items: flex-start;
  gap: 0.45rem;
  cursor: pointer;
  font-size: 0.85rem;
}
.radio span {
  display: flex;
  flex-direction: column;
  gap: 0.1rem;
}
.sub-field {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.sub-label {
  font-size: 0.78rem;
  font-weight: 600;
  opacity: 0.75;
  text-transform: uppercase;
  letter-spacing: 0.03em;
}
.source-detail {
  font-size: 0.78rem;
  opacity: 0.65;
  font-weight: 400;
}
.source-note {
  font-size: 0.75rem;
  opacity: 0.55;
  margin: 0;
}
.loading-indicator {
  font-size: 0.8rem;
  opacity: 0.65;
}
.version-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(280px, 1fr));
  gap: 0.75rem;
}
.version-block {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: 6px;
  padding: 0.65rem 0.75rem;
}
.version-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.4rem;
}
.version-label {
  font-weight: 700;
  font-size: 0.85rem;
}
.version-all {
  font-size: 0.8rem;
  opacity: 0.85;
  margin-left: auto;
}
.version-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  max-height: 140px;
  overflow-y: auto;
}
.version-chip {
  padding: 2px 8px;
  border-radius: 4px;
  border: 1px solid var(--border);
  background: var(--panel);
  color: var(--text);
  font-size: 0.75rem;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: background 0.15s, border-color 0.15s;
}
.version-chip:hover {
  border-color: var(--cyan);
}
.version-chip.active {
  background: var(--cyan);
  color: var(--bg);
  border-color: var(--cyan);
}
.version-chip.chip-kdm {
  border-color: color-mix(in srgb, var(--cyan) 60%, var(--border));
}
.version-chip.chip-github {
  border-style: dashed;
}
.chip-badge {
  font-size: 0.6rem;
  font-weight: 700;
  background: var(--yellow, #eab308);
  color: #000;
  padding: 0 3px;
  border-radius: 2px;
  margin-left: 2px;
}
.version-gh-hint {
  font-size: 0.8rem;
  opacity: 0.9;
  margin: 0;
  padding: 0.45rem 0.65rem;
  background: color-mix(in srgb, var(--cyan) 10%, transparent);
  border-radius: 6px;
  border-left: 3px solid var(--cyan);
}
.version-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
  font-size: 0.75rem;
  opacity: 0.8;
}
.version-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.legend-swatch {
  display: inline-block;
  width: 10px;
  height: 10px;
  border-radius: 2px;
}
.swatch-kdm {
  background: var(--cyan);
}
.swatch-gh {
  border: 1.5px dashed var(--yellow, #eab308);
  background: transparent;
}
.actions {
  display: flex;
  justify-content: flex-end;
  padding-top: 0.5rem;
  border-top: 1px solid var(--border);
  margin-top: auto;
}
.btn {
  padding: 0.55rem 1.25rem;
  border-radius: 6px;
  border: 1px solid var(--border);
  cursor: pointer;
  font-size: 0.9rem;
  font-weight: 600;
}
.btn-primary {
  background: var(--cyan);
  color: var(--bg);
  border-color: var(--cyan);
}
.btn-primary:hover {
  filter: brightness(1.08);
}
.error-msg {
  color: var(--red);
  font-size: 0.85rem;
  margin: 0;
}

@media (max-width: 900px) {
  .form-grid {
    grid-template-columns: 1fr;
  }
  .inline-inputs {
    grid-template-columns: 1fr;
  }
}

@media (max-width: 480px) {
  .step-title {
    font-size: 1rem;
  }
  .form-section {
    padding: 0.85rem;
  }
  .version-grid {
    grid-template-columns: 1fr;
  }
  .actions .btn {
    width: 100%;
  }
}
</style>
