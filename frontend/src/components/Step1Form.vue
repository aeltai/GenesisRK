<script setup lang="ts">
import { computed, ref, watch, onMounted, onUnmounted } from 'vue'
import type { Step1OptionsResponse } from '../types/genesis'
import type { RancherVersionInfo } from '../api/genesis'
import LoadingShapes from './LoadingShapes.vue'
import { CNI_CATALOG, githubRelease, k3sRelease, rke2Release, rancherRelease, LOAD_BALANCER_OPTIONS, cniIconUrl, type LoadBalancerOption } from '../utils/componentLinks'
import { brandIcon } from '../utils/brandIcons'
import {
  rancherEditionIcon,
  rancherEditionAvailability,
  rancherEditionAvailabilityLabel,
  rancherEditionAvailabilityHint,
  rancherEditionAvailabilityShort,
  type RancherEditionAvailability,
} from '../utils/rancherBrandIcons'
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
const includeCommunityImageLists = defineModel<boolean>('includeCommunityImageLists', { default: true })
const includeAppCollection = defineModel<boolean>('includeAppCollection', { default: false })
const includePartnerCharts = defineModel<boolean>('includePartnerCharts', { default: false })
const includeUIPluginCharts = defineModel<boolean>('includeUIPluginCharts', { default: false })
const includeCertManager = defineModel<boolean>('includeCertManager', { default: true })
const appUser = defineModel<string>('appUser', { default: '' })
const appPassword = defineModel<string>('appPassword', { default: '' })
const distros = defineModel<string[]>('distros', { default: () => ['rke2'] })
const cni = defineModel<string>('cni', { default: 'cni_calico' })
const arch = defineModel<string>('arch', { default: 'amd64' })
const lbK3sKlipper = defineModel<boolean>('lbK3sKlipper', { default: false })
const lbK3sTraefik = defineModel<boolean>('lbK3sTraefik', { default: false })
const lbRKE2Nginx = defineModel<boolean>('lbRKE2Nginx', { default: true })
const lbRKE2Traefik = defineModel<boolean>('lbRKE2Traefik', { default: false })
const includeRC = defineModel<boolean>('includeRC', { default: false })
const includeGitHubVersions = defineModel<boolean>('includeGitHubVersions', { default: false })
const includeDeprecatedPatches = defineModel<boolean>('includeDeprecatedPatches', { default: false })
const includeWindows = defineModel<boolean>('includeWindows', { default: false })
const k3sVersions = defineModel<string[]>('k3sVersions', { default: () => [] })
const rke2Versions = defineModel<string[]>('rke2Versions', { default: () => [] })

// CNI options by distro. The backend only offers Flannel for K3s
// ("Flannel is only available for K3s"); Canal/Calico/Cilium are available for both.
// K3s default is Flannel; RKE2 default is Canal.
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
    // Flannel is not offered for RKE2-only clusters.
    return [
      { id: 'cni_canal', label: 'Canal', hint: 'RKE2 default' },
      { id: 'cni_calico', label: 'Calico' },
      { id: 'cni_cilium', label: 'Cilium' },
      { id: 'cni', label: 'All CNI' },
      { id: '', label: 'None' },
    ]
  }
  // Both distros selected: Flannel is available because K3s is selected.
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

// CNI is multi-select only when BOTH K3s and RKE2 are selected (Flannel is K3s-only;
// Canal/Calico/Cilium apply to RKE2). With a single distro it stays single-select.
const cniMulti = computed(() => distros.value.length === 2)

const cniSelectedSet = computed(() => {
  const s = new Set<string>()
  for (const part of (cni.value || '').split(',')) {
    if (part) s.add(part)
  }
  return s
})

function isCniActive(id: string): boolean {
  if (cniMulti.value) {
    if (id === 'cni') return cni.value === 'cni'
    if (id === '') return cni.value === ''
    return cniSelectedSet.value.has(id)
  }
  return cni.value === id
}

function selectCni(id: string) {
  if (!cniMulti.value || id === 'cni' || id === '') {
    // single-select, or the special "All CNI" / "None" options (always exclusive)
    cni.value = id
    return
  }
  // multi-select: toggle membership, starting from the specific selection
  const set = new Set<string>(
    cni.value === 'cni' || cni.value === '' ? [] : cniSelectedSet.value
  )
  if (set.has(id)) set.delete(id)
  else set.add(id)
  cni.value = [...set].sort().join(',')
}

const visibleLbOptions = computed(() =>
  LOAD_BALANCER_OPTIONS.filter((o) => distros.value.includes(o.distro))
)

const lbGroups = computed(() => {
  const groups: { distro: string; label: string; iconKey: 'k3s' | 'rke2'; items: LoadBalancerOption[] }[] = []
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
      iconKey: 'rke2',
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

function setLbActive(id: LoadBalancerOption['id'], value: boolean) {
  switch (id) {
    case 'lbK3sKlipper': lbK3sKlipper.value = value; break
    case 'lbK3sTraefik': lbK3sTraefik.value = value; break
    case 'lbRKE2Nginx': lbRKE2Nginx.value = value; break
    case 'lbRKE2Traefik': lbRKE2Traefik.value = value; break
  }
}

// Independent toggles: each option adds/removes its images from the air-gap
// list. A live cluster usually runs only one RKE2 ingress, but packing both
// NGINX and Traefik images is valid when generating a combined list.
function toggleLb(id: LoadBalancerOption['id']) {
  if (!LOAD_BALANCER_OPTIONS.some((o) => o.id === id)) return
  setLbActive(id, !isLbActive(id))
}

// When distros change, reset CNI and LB options for deselected distros
watch(
  () => [distros.value, cniOptions.value] as const,
  () => {
    const opts = cniOptions.value
    const validIDs = new Set(opts.map((o) => o.id))
    const multi = distros.value.length === 2
    if (multi) {
      // Keep only still-valid specific selections; drop "All"/"None" pseudo-values.
      const kept: string[] = []
      for (const part of (cni.value || '').split(',')) {
        if (part && part !== 'cni' && validIDs.has(part) && !kept.includes(part)) kept.push(part)
      }
      cni.value = kept.sort().join(',')
      if (!kept.length && opts[0]) cni.value = opts[0].id
    } else {
      const valid = validIDs.has(cni.value)
      const first = opts[0]
      if (!valid && first) cni.value = first.id
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
  if (sel.length === 1) {
    const v = sel[0]
    const meta = rancherVersionAnnotations.value.find((r) => r.version === v)
    if (!meta) return v
    const mode = rancherEditionAvailability(meta.primeAvailable, meta.communityAvailable)
    return `${v} · ${rancherEditionAvailabilityLabel(mode)}`
  }
  return `${sel.length} versions`
})

function rancherEditionMode(rv: { primeAvailable?: boolean; communityAvailable?: boolean }): RancherEditionAvailability {
  return rancherEditionAvailability(rv.primeAvailable, rv.communityAvailable)
}

const rancherVersionAnnotations = computed(() =>
  annotateRancherVersions(
    props.availableRancherVersions.map((r) => ({
      version: r.version,
      date: r.date,
      primeAvailable: r.primeAvailable,
      communityAvailable: r.communityAvailable ?? true,
    }))
  )
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
      <p class="field-hint">Select release(s). Defaults to newest stable patch.</p>

      <div class="edition-explainer">
        <div class="edition-explainer-card edition-community" title="Official open-source release on GitHub (rancher-images.txt, K3s/RKE2 lists).">
          <img :src="rancherEditionIcon(false)" alt="" class="edition-explainer-icon" />
          <strong>Community</strong>
        </div>
        <div class="edition-explainer-card edition-prime" title="Curated SUSE registry (prime.ribs.rancher.io) — certified for air-gapped Prime installs.">
          <img :src="rancherEditionIcon(true)" alt="" class="edition-explainer-icon" />
          <strong>Rancher Prime</strong>
        </div>
      </div>

      <details class="rancher-legend-details">
        <summary>Lifecycle badges (Current, Latest, EOM…)</summary>
        <div class="version-legend rancher-version-legend">
          <span class="version-legend-item"><span class="lifecycle-badge badge-current">Current</span> newest stable minor</span>
          <span class="version-legend-item"><span class="lifecycle-badge badge-latest">Latest patch</span> highest patch for minor</span>
          <span class="version-legend-item"><span class="lifecycle-badge badge-deprecated">Deprecated</span> superseded patch</span>
          <span class="version-legend-item"><span class="lifecycle-badge badge-eom">EOM</span> / <span class="lifecycle-badge badge-eol">EOL</span></span>
        </div>
      </details>
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
                :class="{
                  'option-eol': rv.status === 'eol',
                  'option-deprecated': rv.isDeprecatedPatch,
                }"
                :title="lifecycleStatusTitle(rv)"
              >
                <input
                  type="checkbox"
                  :checked="rancherVersions.includes(rv.version)"
                  @change="toggleRancherVersion(rv.version)"
                />
                <span class="option-version">{{ rv.version }}</span>
                <span
                  class="edition-pill"
                  :class="`edition-pill-${rancherEditionMode(rv)}`"
                  :title="rancherEditionAvailabilityHint(rancherEditionMode(rv))"
                >
                  <span class="edition-pill-icons">
                    <img
                      v-if="rv.communityAvailable !== false"
                      :src="rancherEditionIcon(false)"
                      alt=""
                      class="edition-pill-icon"
                      title="Community (GitHub)"
                    />
                    <img
                      v-if="rv.primeAvailable"
                      :src="rancherEditionIcon(true)"
                      alt=""
                      class="edition-pill-icon"
                      title="Rancher Prime (prime.ribs)"
                    />
                  </span>
                  <span class="edition-pill-label">{{ rancherEditionAvailabilityLabel(rancherEditionMode(rv)) }}</span>
                  <span class="edition-pill-sub">{{ rancherEditionAvailabilityShort(rancherEditionMode(rv)) }}</span>
                </span>
                <span v-if="rv.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
                <span v-else-if="rv.isLatestPatch" class="lifecycle-badge badge-latest">Latest patch</span>
                <span v-else-if="rv.isDeprecatedPatch" class="lifecycle-badge badge-deprecated">Deprecated</span>
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
      <label class="check rc-toggle" title="All stable patches per Kubernetes minor; superseded ones marked Deprecated.">
        <input v-model="includeDeprecatedPatches" type="checkbox" />
        Include older KDM patches
      </label>
      <label class="check rc-toggle" title="K3s/RKE2 tags newer than KDM — not older stable patches.">
        <input v-model="includeGitHubVersions" type="checkbox" />
        Include GitHub versions
      </label>
      <label v-if="includeGitHubVersions" class="check rc-toggle" title="RC/alpha/beta K3s/RKE2 versions from GitHub.">
        <input v-model="includeRC" type="checkbox" />
        Include pre-releases
      </label>
      <p v-if="loadError" class="error-msg">{{ loadError }}</p>
    </div>

    <div class="field source-field">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg" alt="" class="ctx-icon" />
        Image list source (what to merge into your export)
      </label>
      <p class="field-hint source-field-hint">Pick registries to pull image lists from.</p>
      <div class="checkbox-group source-checkboxes">
        <label class="checkbox-label source-card" :class="{ active: includeCommunityImageLists }" title="GitHub releases: rancher-images.txt, K3s/RKE2 lists from k3s-io/k3s and rancher/rke2.">
          <input v-model="includeCommunityImageLists" type="checkbox" />
          <img :src="rancherEditionIcon(false)" alt="" class="source-edition-icon" />
          <span class="source-card-body">
            <span class="source-card-title">Community</span>
            <span class="source-detail">GitHub releases</span>
          </span>
        </label>
        <label class="checkbox-label source-card" :class="{ active: isRPMGC }" title="prime.ribs.rancher.io — K3s/RKE2 image lists for your selected versions + allowlisted Rancher core images (same policy as Community).">
          <input v-model="isRPMGC" type="checkbox" />
          <img :src="rancherEditionIcon(true)" alt="" class="source-edition-icon" />
          <span class="source-card-body">
            <span class="source-card-title">Rancher Prime</span>
            <span class="source-detail">prime.ribs.rancher.io</span>
          </span>
        </label>
      </div>
      <p v-if="!includeCommunityImageLists && !isRPMGC" class="source-warn">Select at least one image list source.</p>
      <p class="source-note" title="Both use the same allowlist for Rancher core images. Prime also pulls K3s/RKE2 lists from prime.ribs for the versions you selected — not the full Rancher matrix.">Enable both to merge Community + Prime sources.</p>
      <div v-if="isRPMGC && distros.includes('rke2')" class="prime-patcher-hint">
        <strong>rke2-patcher available</strong> — patched CVE-fixed images for Prime RKE2. Commands in Step 3.
      </div>
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

    <div class="field opt-in-repos">
      <p class="field-hint" title="Not part of the default rancher/charts bundle. Changing later requires generating again.">Optional chart repos — select before generating.</p>
      <label class="checkbox-label" title="TLS certificate controller from jetstack — install before Rancher Helm chart. Images: quay.io/jetstack/cert-manager-*.">
        <input v-model="includeCertManager" type="checkbox" />
        Include cert-manager (required for Rancher install)
      </label>
      <label class="checkbox-label">
        <input v-model="includePartnerCharts" type="checkbox" />
        Include Partner Charts (rancher/partner-charts)
      </label>
      <label class="checkbox-label" title="Rancher dashboard extension charts (e.g. Elemental UI), shown separately in Step 3 — distinct from ui-plugin-operator under Essentials.">
        <input v-model="includeUIPluginCharts" type="checkbox" />
        Include UI Plugins (rancher/ui-plugin-charts)
      </label>
    </div>

    <div class="field">
      <label>Distros</label>
      <div class="check-group distros-group">
        <label class="check">
          <input type="checkbox" :checked="distros.includes('k3s')" @change="toggleDistro('k3s')" />
          <img :src="brandIcon('k3s')" alt="" class="ctx-icon ctx-icon-sm" />
          K3s
        </label>
        <label class="check">
          <input type="checkbox" :checked="distros.includes('rke2')" @change="toggleDistro('rke2')" />
          <img :src="brandIcon('rke2')" alt="" class="ctx-icon ctx-icon-sm" />
          RKE2
        </label>
      </div>
    </div>

    <div v-if="options?.capabilities" class="field versions">
      <label class="label-with-icon">
        <img src="https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/kubernetes.svg" alt="" class="ctx-icon" />
        Kubernetes versions
      </label>
      <p
        v-if="!includeDeprecatedPatches && !includeGitHubVersions"
        class="version-gh-hint"
        title="Enable 'Include older KDM patches' for superseded releases, or 'Include GitHub versions' for tags ahead of KDM."
      >
        Showing latest KDM patch per minor.
      </p>
      <div class="version-legend">
        <span class="version-legend-item"><span class="legend-swatch swatch-kdm"></span> KDM (Rancher supported)</span>
        <span v-if="includeGitHubVersions" class="version-legend-item"><span class="legend-swatch swatch-gh"></span> GitHub release (newer)</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-current">Current</span> newest minor line</span>
        <span class="version-legend-item"><span class="lifecycle-badge badge-latest">Latest patch</span> highest patch for minor</span>
        <span v-if="includeDeprecatedPatches" class="version-legend-item"><span class="lifecycle-badge badge-deprecated">Deprecated</span> superseded patch</span>
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
              'chip-deprecated': meta.isDeprecatedPatch,
            }"
            :title="lifecycleStatusTitle(meta)"
          >
            <input type="checkbox" :checked="k3sVersions.includes(meta.version)" @change="toggleVersion(k3sVersions, meta.version, val => k3sVersions = val)" hidden />
            {{ meta.version }}
            <span v-if="meta.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
            <span v-else-if="meta.isLatestPatch" class="lifecycle-badge badge-latest">Latest</span>
            <span v-else-if="meta.isDeprecatedPatch" class="lifecycle-badge badge-deprecated">Deprecated</span>
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
              'chip-deprecated': meta.isDeprecatedPatch,
            }"
            :title="lifecycleStatusTitle(meta)"
          >
            <input type="checkbox" :checked="rke2Versions.includes(meta.version)" @change="toggleVersion(rke2Versions, meta.version, val => rke2Versions = val)" hidden />
            {{ meta.version }}
            <span v-if="meta.isCurrentMinor" class="lifecycle-badge badge-current">Current</span>
            <span v-else-if="meta.isLatestPatch" class="lifecycle-badge badge-latest">Latest</span>
            <span v-else-if="meta.isDeprecatedPatch" class="lifecycle-badge badge-deprecated">Deprecated</span>
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
      <div class="arch-field">
        <label>Node architecture</label>
        <div class="option-cards arch-cards">
          <label class="option-card arch-card" :class="{ active: arch === 'amd64' }">
            <input v-model="arch" type="radio" value="amd64" hidden />
            <span class="option-card-head">
              <span class="option-logo-wrap">
                <span class="arch-glyph">x86</span>
              </span>
              <span class="option-card-meta">
                <span class="option-card-title">amd64</span>
                <span class="option-card-desc">Intel/AMD 64-bit — default for most clusters</span>
              </span>
              <span class="option-check" :class="{ on: arch === 'amd64' }" aria-hidden="true" />
            </span>
          </label>
          <label class="option-card arch-card" :class="{ active: arch === 'arm64' }">
            <input v-model="arch" type="radio" value="arm64" hidden />
            <span class="option-card-head">
              <span class="option-logo-wrap">
                <span class="arch-glyph">ARM</span>
              </span>
              <span class="option-card-meta">
                <span class="option-card-title">arm64</span>
                <span class="option-card-desc">aarch64 — Graviton, Ampere, Apple Silicon nodes</span>
              </span>
              <span class="option-check" :class="{ on: arch === 'arm64' }" aria-hidden="true" />
            </span>
          </label>
        </div>
      </div>
    </div>

    <div v-if="distros.length > 0" class="field cni-field">
      <label>Container Network (CNI)</label>
      <p v-if="cniMulti" class="field-note">
        <span class="cni-multi-hint">Both distros selected — pick one or more CNIs.</span>
      </p>
      <div class="option-cards cni-cards">
        <button
          v-for="o in cniCards"
          :key="o.id || 'none'"
          type="button"
          class="option-card cni-card"
          :class="{ active: isCniActive(o.id) }"
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
            <span class="option-check" :class="{ on: isCniActive(o.id) }" aria-hidden="true" />
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
      <p class="field-hint">Select one or more to include their images. A cluster usually runs one ingress; packing both is fine for air-gap lists.</p>
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
.field-hint-sub {
  margin-top: 0.35rem;
  margin-bottom: 0;
}
.opt-in-repos {
  padding: 0.65rem 0.75rem;
  border: 1px dashed var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--panel) 90%, var(--bg));
}
.rancher-version-field {
  flex-direction: column;
  align-items: stretch;
}
.edition-explainer {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 0.55rem;
  margin: 0.5rem 0 0.35rem;
}
.edition-explainer-card {
  display: flex;
  gap: 0.5rem;
  align-items: center;
  padding: 0.45rem 0.65rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  font-size: 0.82rem;
  cursor: default;
}
.edition-explainer-card strong {
  font-weight: 600;
}
.edition-explainer-card.edition-community {
  background: color-mix(in srgb, var(--accent) 8%, var(--panel));
  border-color: color-mix(in srgb, var(--accent) 28%, var(--border));
}
.edition-explainer-card.edition-prime {
  background: color-mix(in srgb, #7c3aed 8%, var(--panel));
  border-color: color-mix(in srgb, #7c3aed 28%, var(--border));
}
.edition-explainer-icon {
  width: 1.75rem;
  height: 1.75rem;
  flex-shrink: 0;
  object-fit: contain;
}
.rancher-version-dropdown {
  position: relative;
  width: 100%;
  max-width: 640px;
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
  display: grid;
  grid-template-columns: auto minmax(5.5rem, 1fr) minmax(9rem, 1.4fr) auto auto auto;
  align-items: center;
  gap: 6px 8px;
  padding: 0.45rem 0.75rem;
  font-size: 0.88rem;
  cursor: pointer;
  user-select: none;
}
@media (max-width: 640px) {
  .rancher-version-option {
    grid-template-columns: auto 1fr;
  }
  .edition-pill {
    grid-column: 2 / -1;
  }
}
.edition-pill {
  display: flex;
  flex-direction: column;
  gap: 1px;
  min-width: 0;
  padding: 0.2rem 0.45rem;
  border-radius: var(--radius-sm);
  border: 1px solid var(--border);
  line-height: 1.2;
}
.edition-pill-icons {
  display: flex;
  align-items: center;
  gap: 3px;
}
.edition-pill-icon {
  width: 1rem;
  height: 1rem;
  object-fit: contain;
}
.edition-pill-label {
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.02em;
}
.edition-pill-sub {
  font-size: 0.65rem;
  opacity: 0.85;
}
.edition-pill-both {
  background: linear-gradient(
    90deg,
    color-mix(in srgb, var(--accent) 12%, var(--panel)) 0%,
    color-mix(in srgb, var(--accent) 12%, var(--panel)) 48%,
    color-mix(in srgb, #7c3aed 12%, var(--panel)) 52%,
    color-mix(in srgb, #7c3aed 12%, var(--panel)) 100%
  );
  border-color: color-mix(in srgb, #7c3aed 25%, var(--accent));
}
.edition-pill-both .edition-pill-label {
  color: var(--text);
}
.edition-pill-community-only {
  background: color-mix(in srgb, var(--accent) 10%, var(--panel));
  border-color: color-mix(in srgb, var(--accent) 35%, transparent);
}
.edition-pill-community-only .edition-pill-label {
  color: var(--accent);
}
.edition-pill-prime-only {
  background: color-mix(in srgb, #7c3aed 10%, var(--panel));
  border-color: color-mix(in srgb, #7c3aed 35%, transparent);
}
.edition-pill-prime-only .edition-pill-label {
  color: #6d28d9;
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
.rancher-legend-details {
  margin: 0.25rem 0 0.5rem;
  font-size: 0.82rem;
  color: var(--text-muted);
}
.rancher-legend-details summary {
  cursor: pointer;
  user-select: none;
}
.rancher-legend-details[open] summary {
  margin-bottom: 0.35rem;
}
.rancher-version-option.option-eol {
  opacity: 0.75;
}
.rancher-version-option.option-deprecated {
  opacity: 0.88;
}
.legend-edition-icon,
.badge-edition-icon {
  width: 0.85rem;
  height: 0.85rem;
  object-fit: contain;
  vertical-align: middle;
  margin-right: 2px;
}
.lifecycle-badge.badge-prime {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  background: color-mix(in srgb, #7c3aed 14%, var(--panel));
  color: #6d28d9;
  border: 1px solid color-mix(in srgb, #7c3aed 30%, transparent);
}
.lifecycle-badge.badge-community {
  display: inline-flex;
  align-items: center;
  gap: 2px;
  background: color-mix(in srgb, var(--accent) 12%, var(--panel));
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 28%, transparent);
}
.cni-field,
.lb-field,
.platform-field,
.arch-field {
  flex-direction: column;
  align-items: stretch;
}
.platform-cards,
.arch-cards {
  grid-template-columns: repeat(auto-fill, minmax(240px, 1fr));
}
.platform-card,
.arch-card {
  cursor: pointer;
}
.arch-glyph {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 2rem;
  height: 2rem;
  font-size: 0.65rem;
  font-weight: 700;
  letter-spacing: 0.04em;
  border-radius: 6px;
  background: color-mix(in srgb, var(--accent) 15%, var(--panel));
  color: var(--accent);
  border: 1px solid color-mix(in srgb, var(--accent) 30%, transparent);
}
.cni-multi-hint {
  color: var(--cyan);
  font-weight: 600;
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
.version-chip.chip-deprecated:not(.active) {
  opacity: 0.82;
  border-color: color-mix(in srgb, #b45309 35%, var(--border));
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
.badge-deprecated {
  background: color-mix(in srgb, #b45309 16%, var(--panel));
  color: #b45309;
  border: 1px solid color-mix(in srgb, #b45309 35%, transparent);
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
.source-warn {
  font-size: 0.78rem;
  color: var(--warn, #c9a227);
  margin: 0.35rem 0 0;
}
.source-field-hint {
  margin-bottom: 0.35rem;
}
.source-checkboxes {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(260px, 1fr));
  gap: 0.55rem;
  width: 100%;
}
.source-card {
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  padding: 0.55rem 0.65rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  cursor: pointer;
  transition: border-color 0.15s, background 0.15s;
}
.source-card.active {
  border-color: var(--border-strong);
  background: color-mix(in srgb, var(--accent) 6%, var(--panel));
}
.source-card:has(input:checked) {
  border-color: color-mix(in srgb, var(--accent) 40%, var(--border));
}
.source-edition-icon {
  width: 1.6rem;
  height: 1.6rem;
  flex-shrink: 0;
  object-fit: contain;
  margin-top: 0.1rem;
}
.source-card-body {
  display: flex;
  flex-direction: column;
  gap: 0.15rem;
  min-width: 0;
}
.source-card-title {
  font-weight: 700;
  font-size: 0.88rem;
}
.source-checkboxes .source-detail {
  opacity: 0.82;
  line-height: 1.35;
}
.prime-patcher-hint {
  margin-top: 0.5rem;
  padding: 0.5rem 0.7rem;
  font-size: 0.8rem;
  border: 1px solid color-mix(in srgb, var(--accent) 35%, transparent);
  border-radius: 6px;
  background: color-mix(in srgb, var(--accent) 10%, var(--panel));
  color: var(--text);
  line-height: 1.45;
}
.prime-patcher-hint code {
  background: var(--bg);
  padding: 1px 5px;
  border-radius: 3px;
  font-size: 0.85em;
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
