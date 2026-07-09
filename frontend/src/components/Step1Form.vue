<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { Step1OptionsResponse } from '../types/genesis'
import type { RancherVersionInfo } from '../api/genesis'
import LoadingShapes from './LoadingShapes.vue'
import { CNI_CATALOG, githubRelease, k3sRelease, rke2Release, rancherRelease, LOAD_BALANCER_OPTIONS, cniIconUrl, type LoadBalancerOption } from '../utils/componentLinks'
import { brandIcon } from '../utils/brandIcons'
import {
  annotateDistroVersions,
  annotateRancherVersions,
  lifecycleStatusLabel,
  lifecycleStatusTitle,
  pickLatestOfficialDistroVersion,
} from '../utils/versionLifecycle'

const props = defineProps<{
  availableRancherVersions: RancherVersionInfo[]
  options: Step1OptionsResponse | null
  loadError: string
  optionsLoading: boolean
}>()

defineEmits<{
  generate: []
}>()

const rancherVersion = defineModel<string>('rancherVersion', { default: '' })
const rancherVersions = defineModel<string[]>('rancherVersions', { default: () => [] })
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
const k3sVersions = defineModel<string[]>('k3sVersions', { default: () => [] })
const rke2Versions = defineModel<string[]>('rke2Versions', { default: () => [] })

// CNI options by distro: K3s default is Flannel; RKE2 defaults to Canal and supports Calico, Cilium, Flannel
const cniOptions = computed(() => {
  const d = distros.value
  const hasK3s = d.includes('k3s')
  const hasRKE2 = d.includes('rke2')
  const onlyK3s = d.length === 1 && d[0] === 'k3s'
  const onlyRKE2 = d.length === 1 && d[0] === 'rke2'

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
  const base: { id: string; label: string; hint?: string }[] = []
  if (hasK3s) base.push({ id: 'cni_flannel', label: 'Flannel', hint: 'K3s default' })
  if (hasRKE2) base.push({ id: 'cni_canal', label: 'Canal', hint: 'RKE2 default' })
  if (hasRKE2) base.push({ id: 'cni_calico', label: 'Calico' })
  if (hasRKE2) base.push({ id: 'cni_cilium', label: 'Cilium' })
  base.push({ id: 'cni', label: 'All CNI' }, { id: '', label: 'None' })
  return base
})

const cniCards = computed(() =>
  cniOptions.value.map((o) => ({
    ...o,
    ...(CNI_CATALOG[o.id] ?? CNI_CATALOG['']),
  }))
)

function selectCni(id: string) {
  cni.value = id
}

const visibleLbOptions = computed(() =>
  LOAD_BALANCER_OPTIONS.filter((o) => distros.value.includes(o.distro))
)

const lbGroups = computed(() => {
  const groups: { distro: string; label: string; iconKey: 'k3s' | 'rancher'; items: LoadBalancerOption[] }[] = []
  if (distros.value.includes('k3s')) {
    groups.push({
      distro: 'k3s',
      label: 'K3s',
      iconKey: 'k3s',
      items: visibleLbOptions.value.filter((o) => o.distro === 'k3s'),
    })
  }
  if (distros.value.includes('rke2')) {
    groups.push({
      distro: 'rke2',
      label: 'RKE2',
      iconKey: 'rancher',
      items: visibleLbOptions.value.filter((o) => o.distro === 'rke2'),
    })
  }
  return groups
})

function isLbActive(id: LoadBalancerOption['id']): boolean {
  switch (id) {
    case 'lbK3sKlipper': return lbK3sKlipper.value
    case 'lbK3sTraefik': return lbK3sTraefik.value
    case 'lbRKE2Nginx': return lbRKE2Nginx.value
    case 'lbRKE2Traefik': return lbRKE2Traefik.value
    default: return false
  }
}

function toggleLb(id: LoadBalancerOption['id']) {
  switch (id) {
    case 'lbK3sKlipper': lbK3sKlipper.value = !lbK3sKlipper.value; break
    case 'lbK3sTraefik': lbK3sTraefik.value = !lbK3sTraefik.value; break
    case 'lbRKE2Nginx': lbRKE2Nginx.value = !lbRKE2Nginx.value; break
    case 'lbRKE2Traefik': lbRKE2Traefik.value = !lbRKE2Traefik.value; break
  }
}

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

function applyLatestDistroVersion(distro: 'k3s' | 'rke2') {
  const cap = props.options?.capabilities?.[distro]
  const pick = pickLatestOfficialDistroVersion(cap?.versions ?? [], cap?.sources)
  if (!pick) return
  if (distro === 'k3s') k3sVersions.value = [pick]
  else rke2Versions.value = [pick]
}

function toggleDistroAll(distro: 'k3s' | 'rke2') {
  if (distro === 'k3s') {
    if (k3sVersions.value.includes('all')) applyLatestDistroVersion('k3s')
    else k3sVersions.value = ['all']
    return
  }
  if (rke2Versions.value.includes('all')) applyLatestDistroVersion('rke2')
  else rke2Versions.value = ['all']
}

function toggleDistro(d: string) {
  const i = distros.value.indexOf(d)
  if (i >= 0) {
    distros.value = distros.value.filter((x) => x !== d)
  } else {
    distros.value = [...distros.value, d]
    if (d === 'k3s') {
      lbK3sKlipper.value = true
      lbK3sTraefik.value = true
      if (!k3sVersions.value.length) {
        applyLatestDistroVersion('k3s')
      }
    }
    if (d === 'rke2') {
      lbRKE2Nginx.value = true
      lbRKE2Traefik.value = false
      if (!rke2Versions.value.length) {
        applyLatestDistroVersion('rke2')
      }
    }
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

const rancherVersionAnnotations = computed(() =>
  annotateRancherVersions(props.availableRancherVersions)
)

const k3sVersionAnnotations = computed(() =>
  annotateDistroVersions(props.options?.capabilities?.k3s?.versions ?? [])
)

const rke2VersionAnnotations = computed(() =>
  annotateDistroVersions(props.options?.capabilities?.rke2?.versions ?? [])
)

watch(
  () => props.options?.capabilities,
  (caps) => {
    if (!caps) return
    if (distros.value.includes('k3s') && !k3sVersions.value.length) {
      applyLatestDistroVersion('k3s')
    }
    if (distros.value.includes('rke2') && !rke2Versions.value.length) {
      applyLatestDistroVersion('rke2')
    }
  },
  { immediate: true }
)

onMounted(() => {
  document.addEventListener('click', closeRancherDropdown)
})
onUnmounted(() => {
  document.removeEventListener('click', closeRancherDropdown)
})
</script>

<template>
  <div class="step1">
    <h2 class="step-title">Step 1: Source &amp; options</h2>

    <div class="field rancher-version-field">
      <label>Rancher version(s)</label>
      <p class="field-hint">Select one or more; the image list will include images for all selected versions.</p>
      <div class="version-legend rancher-version-legend">
        <span class="version-legend-item"><span class="lifecycle-badge badge-current">Current</span> newest minor</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-latest">Latest patch</span> highest patch</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-eom">EOM</span> maintenance only</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-eol">EOL</span> end of life</span>
      </div>
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
                v-for="rv in rancherVersionAnnotations"
                :key="rv.version"
                class="rancher-version-option"
                :class="{ 'option-eol': rv.status === 'eol' }"
                :title="lifecycleStatusTitle(rv)"
              >
                <input
                  type="checkbox"
                  :checked="rancherVersions.includes(rv.version)"
                  @change="toggleRancherVersion(rv.version)"
                />
                <span class="option-version">{{ rv.version }}</span>
                <span v-if="rv.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
                <span v-else-if="rv.isLatestPatch" class="lifecycle-badge badge-latest">Latest patch</span>
                <span v-if="rv.status === 'maintenance'" class="lifecycle-badge badge-eom">{{ lifecycleStatusLabel(rv.status) }}</span>
                <span v-if="rv.status === 'eol'" class="lifecycle-badge badge-eol">{{ lifecycleStatusLabel(rv.status) }}</span>
                <span v-if="rv.releaseDate" class="option-date">{{ rv.releaseDate }}</span>
                <a
                  :href="rancherRelease(rv.version)"
                  target="_blank"
                  rel="noopener noreferrer"
                  class="option-release-link"
                  title="Open GitHub release"
                  @click.stop
                >↗</a>
              </label>
            </div>
          </div>
        </div>
      </template>
      <input
        v-else
        v-model="rancherVersion"
        type="text"
        placeholder="v2.x.x"
        class="input"
      />
      <LoadingShapes v-if="optionsLoading" size="sm" class="options-loader" />
      <label class="check rc-toggle">
        <input v-model="includeGitHubVersions" type="checkbox" />
        Include versions from GitHub (K3s/RKE2 release tags; shows newer than KDM)
      </label>
      <label v-if="includeGitHubVersions" class="check rc-toggle">
        <input v-model="includeRC" type="checkbox" />
        Include pre-release (RC/alpha/beta) versions from GitHub
      </label>
      <p v-if="loadError" class="error-msg">{{ loadError }}</p>
    </div>

    <div class="field source-field">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon" />
        Image list source
      </label>
      <div class="radio-group">
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="false" />
          <span>Community <span class="source-detail">image lists from GitHub releases (k3s-io/k3s, rancher/rke2)</span></span>
        </label>
        <label class="radio">
          <input v-model="isRPMGC" type="radio" :value="true" />
          <span>Rancher Prime <span class="source-detail">image lists from prime.ribs.rancher.io (curated/certified)</span></span>
        </label>
      </div>
      <p class="source-note">Both sources use the same KDM (releases.rancher.com) and chart repos (rancher/charts on GitHub).</p>
    </div>

    <div class="field">
      <label class="checkbox-label">
        <input v-model="includeAppCollection" type="checkbox" />
        Include Application Collection (dp.apps.rancher.io)
      </label>
      <template v-if="includeAppCollection">
        <input v-model="appUser" type="text" placeholder="API username" class="input inline" />
        <input v-model="appPassword" type="password" placeholder="API password/token" class="input inline" />
      </template>
    </div>

    <div class="field">
      <label>Distros</label>
      <div class="check-group distros-group">
        <label class="check">
          <input type="checkbox" :checked="distros.includes('k3s')" @change="toggleDistro('k3s')" />
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/k3s.svg" alt="" class="ctx-icon ctx-icon-sm" />
          K3s
        </label>
        <label class="check">
          <input type="checkbox" :checked="distros.includes('rke2')" @change="toggleDistro('rke2')" />
          <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon ctx-icon-sm" />
          RKE2
        </label>
      </div>
    </div>

    <div v-if="options?.capabilities" class="field versions">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/kubernetes.svg" alt="" class="ctx-icon" />
        Kubernetes versions
      </label>
      <p v-if="!includeGitHubVersions" class="version-gh-hint">
        Only KDM-supported versions are shown. Enable <strong>Include versions from GitHub</strong> above to add newer K3s/RKE2 versions from GitHub releases.
      </p>
      <div class="version-legend">
        <span class="version-legend-item"><span class="legend-swatch swatch-kdm"></span> KDM (Rancher supported)</span>
        <span v-if="includeGitHubVersions" class="version-legend-item"><span class="legend-swatch swatch-gh"></span> GitHub release (newer)</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-current">Current</span> newest minor line</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-latest">Latest patch</span> highest patch for minor</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-eom">EOM</span> maintenance only</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-eol">EOL</span> end of life</span>
      </div>
      <div v-if="distros.includes('k3s') && options?.capabilities?.k3s" class="version-block">
        <div class="version-header">
          <span class="version-label">K3s</span>
          <label class="check version-all">
            <input type="checkbox" :checked="k3sVersions.includes('all')" @change="toggleDistroAll('k3s')" />
            All
          </label>
        </div>
        <div v-if="!k3sVersions.includes('all')" class="version-chips">
          <label
            v-for="meta in k3sVersionAnnotations"
            :key="meta.version"
            class="version-chip"
            :class="{
              active: k3sVersions.includes(meta.version),
              'chip-github': versionSource('k3s', meta.version) === 'github',
              'chip-kdm': versionSource('k3s', meta.version) === 'kdm' || versionSource('k3s', meta.version) === 'both',
              'chip-eol': meta.status === 'eol',
            }"
            :title="lifecycleStatusTitle(meta)"
          >
            <input type="checkbox" :checked="k3sVersions.includes(meta.version)" @change="toggleVersion(k3sVersions, meta.version, val => k3sVersions = val)" hidden />
            {{ meta.version }}
            <span v-if="meta.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
            <span v-else-if="meta.isLatestPatch" class="lifecycle-badge badge-latest">Latest</span>
            <span v-if="meta.status === 'maintenance'" class="lifecycle-badge badge-eom">{{ lifecycleStatusLabel(meta.status) }}</span>
            <span v-if="meta.status === 'eol'" class="lifecycle-badge badge-eol">{{ lifecycleStatusLabel(meta.status) }}</span>
            <span v-if="versionSource('k3s', meta.version) === 'github'" class="chip-badge" title="From GitHub releases (not in KDM)">GH</span>
            <a :href="k3sRelease(meta.version)" target="_blank" rel="noopener noreferrer" class="chip-release-link" title="K3s release on GitHub" @click.stop>↗</a>
          </label>
        </div>
      </div>
      <div v-if="distros.includes('rke2') && options?.capabilities?.rke2" class="version-block">
        <div class="version-header">
          <span class="version-label">RKE2</span>
          <label class="check version-all">
            <input type="checkbox" :checked="rke2Versions.includes('all')" @change="toggleDistroAll('rke2')" />
            All
          </label>
        </div>
        <div v-if="!rke2Versions.includes('all')" class="version-chips">
          <label
            v-for="meta in rke2VersionAnnotations"
            :key="meta.version"
            class="version-chip"
            :class="{
              active: rke2Versions.includes(meta.version),
              'chip-github': versionSource('rke2', meta.version) === 'github',
              'chip-kdm': versionSource('rke2', meta.version) === 'kdm' || versionSource('rke2', meta.version) === 'both',
              'chip-eol': meta.status === 'eol',
            }"
            :title="lifecycleStatusTitle(meta)"
          >
            <input type="checkbox" :checked="rke2Versions.includes(meta.version)" @change="toggleVersion(rke2Versions, meta.version, val => rke2Versions = val)" hidden />
            {{ meta.version }}
            <span v-if="meta.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
            <span v-else-if="meta.isLatestPatch" class="lifecycle-badge badge-latest">Latest</span>
            <span v-if="meta.status === 'maintenance'" class="lifecycle-badge badge-eom">{{ lifecycleStatusLabel(meta.status) }}</span>
            <span v-if="meta.status === 'eol'" class="lifecycle-badge badge-eol">{{ lifecycleStatusLabel(meta.status) }}</span>
            <span v-if="versionSource('rke2', meta.version) === 'github'" class="chip-badge" title="From GitHub releases (not in KDM)">GH</span>
            <a :href="rke2Release(meta.version)" target="_blank" rel="noopener noreferrer" class="chip-release-link" title="RKE2 release on GitHub" @click.stop>↗</a>
          </label>
        </div>
      </div>
    </div>

    <div class="field platform-field">
      <label>Platform</label>
      <p class="field-note">Target node operating systems for container images.</p>
      <div class="option-cards platform-cards">
        <label class="option-card platform-card" :class="{ active: !includeWindows }">
          <input v-model="includeWindows" type="radio" :value="false" hidden />
          <span class="option-card-head">
            <span class="option-logo-wrap">
              <img :src="brandIcon('linux')" alt="" class="option-logo" />
            </span>
            <span class="option-card-meta">
              <span class="option-card-title">Linux only</span>
              <span class="option-card-desc">Standard amd64/arm64 node images</span>
            </span>
            <span class="option-check" :class="{ on: !includeWindows }" aria-hidden="true" />
          </span>
        </label>
        <label class="option-card platform-card" :class="{ active: includeWindows }">
          <input v-model="includeWindows" type="radio" :value="true" hidden />
          <span class="option-card-head">
            <span class="option-logo-wrap option-logo-wrap-dual">
              <img :src="brandIcon('linux')" alt="" class="option-logo" />
              <img :src="brandIcon('windows')" alt="" class="option-logo" />
            </span>
            <span class="option-card-meta">
              <span class="option-card-title">Linux + Windows</span>
              <span class="option-card-desc">Includes Windows node and hybrid workloads</span>
            </span>
            <span class="option-check" :class="{ on: includeWindows }" aria-hidden="true" />
          </span>
        </label>
      </div>
    </div>

    <div v-if="distros.length > 0" class="field cni-field">
      <label>Container Network (CNI)</label>
      <p class="field-note">Pod networking plugin for your cluster profile.</p>
      <div class="option-cards cni-cards">
        <button
          v-for="o in cniCards"
          :key="o.id || 'none'"
          type="button"
          class="option-card cni-card"
          :class="{ active: cni === o.id }"
          @click="selectCni(o.id)"
        >
          <span class="option-card-head">
            <span class="option-logo-wrap">
              <img :src="cniIconUrl(o)" alt="" class="option-logo" />
            </span>
            <span class="option-card-meta">
              <span class="option-card-title-row">
                <span class="option-card-title">{{ o.label }}</span>
                <span v-if="o.hint" class="option-badge">{{ o.hint }}</span>
              </span>
              <span class="option-card-desc">{{ o.description }}</span>
            </span>
            <span class="option-check" :class="{ on: cni === o.id }" aria-hidden="true" />
          </span>
          <span v-if="o.id" class="option-card-foot">
            <a v-if="o.docsUrl" :href="o.docsUrl" target="_blank" rel="noopener noreferrer" class="option-link" @click.stop>Documentation</a>
            <a v-if="o.upstreamRepo" :href="githubRelease(o.upstreamRepo)" target="_blank" rel="noopener noreferrer" class="option-link" @click.stop>Releases</a>
          </span>
        </button>
      </div>
    </div>

    <div v-if="distros.length > 0 && visibleLbOptions.length" class="field lb-field">
      <label>Load balancer / Ingress</label>
      <p class="field-note">Optional ingress controllers and service load balancers per distro.</p>
      <div v-for="group in lbGroups" :key="group.distro" class="lb-group">
        <div class="lb-group-head">
          <img :src="brandIcon(group.iconKey)" alt="" class="lb-group-logo" />
          <span class="lb-group-label">{{ group.label }}</span>
        </div>
        <div class="option-cards lb-cards">
          <button
            v-for="opt in group.items"
            :key="opt.id"
            type="button"
            class="option-card lb-card"
            :class="{ active: isLbActive(opt.id) }"
            @click="toggleLb(opt.id)"
          >
            <span class="option-card-head">
              <span class="option-logo-wrap">
                <img :src="brandIcon(opt.iconKey)" alt="" class="option-logo" />
              </span>
              <span class="option-card-meta">
                <span class="option-card-title-row">
                  <span class="option-card-title">{{ opt.label }}</span>
                </span>
                <span class="option-card-desc">{{ opt.subtitle }}</span>
              </span>
              <span class="option-check" :class="{ on: isLbActive(opt.id) }" aria-hidden="true" />
            </span>
            <span class="option-card-foot">
              <a :href="opt.docsUrl" target="_blank" rel="noopener noreferrer" class="option-link" @click.stop>Documentation</a>
              <a v-if="opt.releaseUrl" :href="opt.releaseUrl" target="_blank" rel="noopener noreferrer" class="option-link" @click.stop>Releases</a>
            </span>
          </button>
        </div>
      </div>
    </div>

    <div class="actions">
      <button type="button" class="btn btn-primary" @click="$emit('generate')">Generate</button>
    </div>
  </div>
</template>

<style scoped>
.step1 {
  display: flex;
  flex-direction: column;
  gap: 1rem;
}
.step-title {
  font-size: 0.9375rem;
  font-weight: 600;
  letter-spacing: 0.01em;
  color: var(--text);
  margin: 0 0 0.75rem 0;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border);
}
.field {
  display: flex;
  flex-wrap: wrap;
  align-items: flex-start;
  gap: 0.5rem;
}
.field label:first-child {
  min-width: 140px;
  font-size: 0.8125rem;
  font-weight: 500;
  color: var(--text);
}
.input {
  padding: 0.4rem 0.6rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  color: var(--text);
}
.input.inline {
  margin-left: 0.5rem;
  width: 180px;
}
.select {
  min-width: 160px;
}
.select.narrow {
  min-width: 200px;
}
.radio-group,
.field-hint {
  margin: 0 0 0.5rem 0;
  font-size: 0.85rem;
  opacity: 0.88;
}
.rancher-version-field {
  flex-direction: column;
  align-items: stretch;
}
.rancher-version-dropdown {
  position: relative;
  width: 100%;
  max-width: 480px;
}
.rancher-version-trigger {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.5rem 0.75rem;
  font-size: 0.9rem;
  font-family: inherit;
  color: var(--text);
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  cursor: pointer;
  text-align: left;
}
.rancher-version-trigger:hover,
.rancher-version-trigger.open {
  border-color: var(--border-strong);
}
.trigger-arrow {
  font-size: 0.7rem;
  opacity: 0.8;
  margin-left: 0.5rem;
}
.rancher-version-panel {
  position: absolute;
  top: 100%;
  left: 0;
  margin-top: 4px;
  min-width: 100%;
  max-height: 280px;
  overflow-y: auto;
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.25);
  z-index: 50;
}
.rancher-version-list {
  padding: 0.35rem 0;
}
.rancher-version-option {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
  padding: 0.4rem 0.75rem;
  font-size: 0.88rem;
  cursor: pointer;
  user-select: none;
}
.rancher-version-option:hover {
  background: var(--panel-elevated);
}
.rancher-version-option input {
  flex-shrink: 0;
}
.option-version {
  font-weight: 600;
}
.option-date {
  font-size: 0.8rem;
  opacity: 0.85;
}
.option-release-link {
  margin-left: auto;
  font-size: 0.85rem;
  color: var(--accent);
  text-decoration: none;
  opacity: 0.85;
  padding: 0 4px;
}
.option-release-link:hover {
  opacity: 1;
}
.rancher-version-legend {
  margin: 0.35rem 0 0.5rem;
}
.rancher-version-option.option-eol {
  opacity: 0.75;
}
.cni-field,
.lb-field,
.platform-field {
  flex-direction: column;
  align-items: stretch;
}
.platform-cards {
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
}
.platform-card {
  cursor: pointer;
}
.option-logo-wrap-dual {
  gap: 2px;
  padding: 0 3px;
}
.option-logo-wrap-dual .option-logo {
  width: 13px;
  height: 13px;
}
.option-logo-wrap .option-logo,
.lb-group-logo {
  width: 16px;
  height: 16px;
  object-fit: contain;
  filter: brightness(0) invert(1);
}
[data-theme="light"] .option-logo-wrap .option-logo,
[data-theme="light"] .lb-group-logo {
  filter: brightness(0);
}
.lb-group-logo {
  width: 14px;
  height: 14px;
}
.field-note {
  width: 100%;
  margin: 0;
  font-size: 0.78rem;
  color: var(--text-muted);
  line-height: 1.45;
}
.option-cards {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
  gap: 0.45rem;
  width: 100%;
}
.option-card {
  display: flex;
  flex-direction: column;
  gap: 0.45rem;
  padding: 0.6rem 0.7rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--panel);
  color: var(--text);
  cursor: pointer;
  text-align: left;
  font-family: inherit;
  transition: border-color 0.15s, background 0.15s;
}
.option-card:hover {
  border-color: var(--border-strong);
  background: var(--panel-elevated);
}
.option-card.active {
  border-color: var(--border-strong);
  background: var(--panel-elevated);
  box-shadow: inset 2px 0 0 var(--accent);
}
.option-card-head {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  width: 100%;
}
.option-logo-wrap {
  flex-shrink: 0;
  width: 28px;
  height: 28px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg) 70%, transparent);
  border: 1px solid var(--border);
}
.option-card-meta {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
}
.option-card-title-row {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  flex-wrap: wrap;
}
.option-card-title {
  font-weight: 600;
  font-size: 0.8125rem;
  letter-spacing: 0.01em;
}
.option-badge {
  font-size: 0.62rem;
  font-weight: 500;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
  padding: 1px 5px;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  background: var(--bg);
}
.option-card-desc {
  font-size: 0.74rem;
  color: var(--text-muted);
  line-height: 1.4;
}
.option-check {
  flex-shrink: 0;
  width: 8px;
  height: 8px;
  margin-top: 6px;
  border-radius: 50%;
  border: 1.5px solid var(--border-strong);
  background: transparent;
  transition: border-color 0.15s, background 0.15s;
}
.option-check.on {
  border-color: var(--accent);
  background: var(--accent);
}
.option-card-foot {
  display: flex;
  flex-wrap: wrap;
  gap: 0.65rem;
  padding-left: calc(28px + 0.55rem);
  opacity: 0;
  max-height: 0;
  overflow: hidden;
  transition: opacity 0.15s, max-height 0.15s;
}
.option-card:hover .option-card-foot,
.option-card.active .option-card-foot {
  opacity: 1;
  max-height: 24px;
}
.option-link {
  font-size: 0.7rem;
  color: var(--text-muted);
  text-decoration: none;
}
.option-link:hover {
  color: var(--accent);
  text-decoration: underline;
}
.lb-group {
  width: 100%;
  margin-top: 0.35rem;
}
.lb-group-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  margin-bottom: 0.35rem;
  padding-bottom: 0.25rem;
  border-bottom: 1px solid var(--border);
}
.lb-group-logo {
  object-fit: contain;
  opacity: 0.9;
}
.lb-group-label {
  font-size: 0.72rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--text-muted);
}
.lb-cards {
  grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
}
.chip-release-link {
  margin-left: 2px;
  font-size: 0.72rem;
  color: inherit;
  opacity: 0.75;
  text-decoration: none;
}
.chip-release-link:hover {
  opacity: 1;
  color: var(--accent);
}
.version-chip.active .chip-release-link {
  color: #fff;
  opacity: 0.9;
}
.label-with-icon {
  display: inline-flex;
  align-items: center;
  gap: 6px;
}
.ctx-icon {
  width: 20px;
  height: 20px;
  object-fit: contain;
  flex-shrink: 0;
  filter: invert(1);
}
.ctx-icon-sm {
  width: 18px;
  height: 18px;
}
[data-theme="light"] .ctx-icon {
  filter: none;
}
.distros-group .check {
  display: inline-flex;
  align-items: center;
  gap: 5px;
}
.check-group {
  display: flex;
  flex-wrap: wrap;
  gap: 1rem;
}
.radio,
.check,
.checkbox-label {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  cursor: pointer;
}
.btn {
  padding: 0.5rem 1rem;
  border-radius: var(--radius-md);
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
  filter: none;
}
.versions {
  flex-direction: column;
  align-items: flex-start;
}
.rancher-version-field .rc-toggle {
  margin: 0.15rem 0 0;
  align-self: flex-start;
  max-width: 100%;
  font-size: 0.85rem;
  opacity: 0.85;
}
.rancher-version-field .error-msg {
  margin: 0.15rem 0 0;
  align-self: flex-start;
}
.loading-indicator,
.options-loader {
  margin-top: 0.25rem;
}
.version-block {
  margin-top: 0.25rem;
}
.version-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  margin-bottom: 0.35rem;
}
.version-label {
  font-weight: 600;
  min-width: 40px;
}
.version-all {
  font-size: 0.85rem;
  opacity: 0.9;
}
.version-chips {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  max-height: 168px;
  overflow-y: auto;
  padding: 2px;
}
.version-chip {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  padding: 2px 8px;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--text);
  font-size: 0.78rem;
  cursor: pointer;
  user-select: none;
  white-space: nowrap;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}
.version-chip.active {
  background: var(--accent);
  color: #fff;
  border-color: var(--accent);
  box-shadow: inset 0 0 0 1px color-mix(in srgb, #fff 12%, transparent);
}
.version-chip.chip-kdm {
  border-color: var(--border-strong);
}
.version-chip.chip-github {
  border-style: dashed;
}
.version-chip.chip-eol:not(.active) {
  opacity: 0.72;
  border-color: color-mix(in srgb, var(--text-muted) 50%, var(--border));
}
.lifecycle-badge {
  font-size: 0.58rem;
  font-weight: 700;
  padding: 0 4px;
  border-radius: var(--radius-sm);
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
.version-chip.active .lifecycle-badge.badge-latest {
  background: color-mix(in srgb, #fff 22%, transparent);
  color: #fff;
  border-color: color-mix(in srgb, #fff 35%, transparent);
}
.version-chip.active .lifecycle-badge.badge-eom,
.version-chip.active .lifecycle-badge.badge-eol {
  color: #000;
}
.chip-badge {
  font-size: 0.65rem;
  font-weight: 700;
  background: var(--yellow, #eab308);
  color: #000;
  padding: 0 3px;
  border-radius: var(--radius-sm);
  margin-left: 2px;
  line-height: 1.2;
}
.version-chip:hover {
  border-color: var(--border-strong);
}
.version-gh-hint {
  font-size: 0.8125rem;
  color: var(--text-muted);
  margin: 0 0 0.5rem 0;
  padding: 0.5rem 0.625rem;
  background: var(--panel-elevated);
  border-radius: var(--radius-md);
  border-left: 2px solid var(--accent);
}
.version-gh-hint strong {
  font-weight: 600;
  color: var(--text);
}
.version-legend {
  display: flex;
  flex-wrap: wrap;
  gap: 0.5rem 1rem;
  font-size: 0.78rem;
  opacity: 0.85;
  margin-bottom: 0.15rem;
}
.version-legend-item {
  display: inline-flex;
  align-items: center;
  gap: 4px;
}
.legend-swatch {
  display: inline-block;
  width: var(--control-size);
  height: var(--control-size);
  border-radius: var(--radius-md);
}
.swatch-kdm {
  background: var(--accent);
}
.swatch-gh {
  border: 1.5px dashed var(--yellow, #eab308);
  background: transparent;
}
.actions {
  margin-top: 0.5rem;
}
.rancher-select {
  min-width: 140px;
}
.source-field {
  flex-direction: column;
  align-items: flex-start;
}
.source-detail {
  font-size: 0.8rem;
  opacity: 0.7;
}
.source-note {
  font-size: 0.78rem;
  opacity: 0.6;
  margin: 0.25rem 0 0;
}
.data-sources {
  margin-top: 1rem;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}
.ds-title {
  font-size: 0.8125rem;
  color: var(--text-muted);
  margin: 0 0 0.5rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}
.ds-grid {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
}
.ds-item {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  font-size: 0.8rem;
}
.ds-label {
  min-width: 100px;
  font-weight: 600;
  opacity: 0.85;
}
.ds-value {
  background: var(--bg);
  padding: 1px 6px;
  border-radius: 3px;
  font-size: 0.78rem;
  word-break: break-all;
}
.error-msg {
  color: var(--red);
  font-size: 0.9rem;
  margin: 0;
}

/* Mobile & tablet */
@media (max-width: 768px) {
  .step1 {
    gap: 0.75rem;
  }
  .field {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.35rem;
  }
  .field label:first-child {
    min-width: 0;
  }
  .input.inline {
    margin-left: 0;
    width: 100%;
    max-width: 280px;
  }
  .select {
    width: 100%;
    max-width: 280px;
    min-width: 0;
  }
  .rancher-version-dropdown {
    max-width: 100%;
  }
  .check-group {
    gap: 0.75rem;
  }
  .version-chips {
    max-height: 100px;
  }
  .option-cards {
    grid-template-columns: 1fr;
  }
  .option-card-foot {
    opacity: 1;
    max-height: none;
  }
  .actions .btn {
    width: 100%;
    max-width: 200px;
  }
}

@media (max-width: 480px) {
  .step-title {
    font-size: 1.1rem;
  }
  .field label:first-child {
    font-size: 0.9rem;
  }
  .input,
  .input.inline,
  .select {
    width: 100%;
    max-width: none;
    box-sizing: border-box;
  }
  .radio-group {
    margin-left: 0;
  }
  .radio span,
  .check,
  .checkbox-label {
    font-size: 0.9rem;
  }
  .source-detail {
    display: block;
    margin-top: 0.2rem;
  }
  .version-header {
    flex-wrap: wrap;
    gap: 0.5rem;
  }
  .version-label {
    min-width: 0;
    width: 100%;
  }
  .version-chips {
    max-height: 90px;
    gap: 3px;
  }
  .version-chip {
    font-size: 0.72rem;
    padding: 2px 6px;
  }
  .version-legend {
    flex-direction: column;
    gap: 0.25rem;
  }
  .ds-grid {
    gap: 0.25rem;
  }
  .ds-item {
    flex-direction: column;
    align-items: flex-start;
    gap: 0.15rem;
  }
  .ds-label {
    min-width: 0;
  }
  .actions .btn {
    width: 100%;
    max-width: none;
  }
}
</style>
