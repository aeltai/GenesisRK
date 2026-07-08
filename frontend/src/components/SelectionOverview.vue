<script setup lang="ts">
import { computed, defineAsyncComponent } from 'vue'
import {
  githubRelease,
  rancherRelease,
  k3sRelease,
  rke2Release,
  selectedCniDefinition,
  STACK_COMPONENTS,
} from '../utils/componentLinks'
import { helixColor, helixColorShade } from '../utils/helixColors'
import { resolveHelixVersions } from '../utils/helixSegments'

const DnaHelixView = defineAsyncComponent(() => import('./DnaHelixView.vue'))

export interface HelixSegmentDef {
  id: string
  label: string
  detail: string
  active: boolean
  color: string
}

function expandVersionSegments(
  baseId: string,
  label: string,
  selected: string[],
  available: string[] | undefined,
  baseColor: string
): HelixSegmentDef[] {
  if (!selected?.length) {
    return [{ id: `${baseId}-pending`, label, detail: '—', active: false, color: baseColor }]
  }
  const versions = resolveHelixVersions(selected, available)
  if (selected.includes('all') && !versions.length) {
    return [{ id: `${baseId}-all`, label, detail: 'All versions', active: true, color: baseColor }]
  }
  return versions.map((v, i) => ({
    id: `${baseId}-${v}`,
    label: `${label} ${v}`,
    detail: v,
    active: true,
    color: helixColorShade(baseColor, i, versions.length),
  }))
}

function expandNamedSegments(
  baseId: string,
  label: string,
  names: string[],
  baseColor: string,
  emptyDetail = 'Not selected'
): HelixSegmentDef[] {
  if (!names.length) {
    return [{ id: `${baseId}-pending`, label, detail: emptyDetail, active: false, color: baseColor }]
  }
  return names.map((name, i) => ({
    id: `${baseId}-${name.replace(/\s+/g, '-').toLowerCase()}`,
    label,
    detail: name,
    active: true,
    color: helixColorShade(baseColor, i, names.length),
  }))
}

const props = withDefaults(
  defineProps<{
    rancherVersions: string[]
    distros: string[]
    cni: string
    k3sVersions?: string[]
    rke2Versions?: string[]
    availableRancherVersions?: string[]
    availableK3sVersions?: string[]
    availableRke2Versions?: string[]
    includeWindows?: boolean
    loadBalancers?: string[]
    chartCount?: number
    imageCount?: number
    compact?: boolean
  }>(),
  {
    k3sVersions: () => [],
    rke2Versions: () => [],
    availableRancherVersions: () => [],
    availableK3sVersions: () => [],
    availableRke2Versions: () => [],
    includeWindows: false,
    loadBalancers: () => [],
    compact: false,
  }
)

const cniDef = computed(() => selectedCniDefinition(props.cni))
const hasK3s = computed(() => props.distros.includes('k3s'))
const hasRke2 = computed(() => props.distros.includes('rke2'))

const resolvedRancherVersions = computed(() =>
  resolveHelixVersions(props.rancherVersions, props.availableRancherVersions)
)
const resolvedK3sVersions = computed(() =>
  hasK3s.value ? resolveHelixVersions(props.k3sVersions, props.availableK3sVersions) : []
)
const resolvedRke2Versions = computed(() =>
  hasRke2.value ? resolveHelixVersions(props.rke2Versions, props.availableRke2Versions) : []
)

const helixSegments = computed(() => {
  const segs: HelixSegmentDef[] = []

  segs.push(
    ...expandVersionSegments(
      'rancher',
      'Rancher',
      props.rancherVersions,
      props.availableRancherVersions,
      helixColor('rancher')
    )
  )

  if (hasK3s.value) {
    segs.push(
      ...expandVersionSegments(
        'k3s',
        'K3s',
        props.k3sVersions ?? [],
        props.availableK3sVersions,
        helixColor('k3s')
      )
    )
  }
  if (hasRke2.value) {
    segs.push(
      ...expandVersionSegments(
        'rke2',
        'RKE2',
        props.rke2Versions ?? [],
        props.availableRke2Versions,
        helixColor('rke2')
      )
    )
  }

  segs.push({
    id: 'platform',
    label: 'Platforms',
    detail: props.includeWindows ? 'Linux + Windows' : 'Linux',
    active: true,
    color: props.includeWindows
      ? helixColor('windows')
      : helixColor('platform'),
  })

  segs.push({
    id: 'cni',
    label: props.cni ? cniDef.value.label : 'CNI',
    detail: props.cni ? 'Pod network' : 'Not selected',
    active: !!props.cni,
    color: props.cni ? cniDef.value.color : helixColor('cni'),
  })

  segs.push(
    ...expandNamedSegments('lb', 'Ingress / LB', props.loadBalancers, helixColor('lb'))
  )

  if (props.chartCount != null) {
    segs.push({
      id: 'charts',
      label: 'Charts',
      detail: (props.chartCount ?? 0) > 0 ? String(props.chartCount) : 'None selected',
      active: (props.chartCount ?? 0) > 0,
      color: helixColor('charts'),
    })
  }
  if (props.imageCount != null) {
    segs.push({
      id: 'images',
      label: 'Images',
      detail: (props.imageCount ?? 0) > 0 ? String(props.imageCount) : 'None selected',
      active: (props.imageCount ?? 0) > 0,
      color: helixColor('images'),
    })
  }

  return segs
})

const completion = computed(() => {
  const segs = helixSegments.value
  if (!segs.length) return 0
  return segs.filter((s) => s.active).length / segs.length
})

const isFullHelix = computed(() => completion.value >= 1)

const helixHeight = computed(() => (props.compact ? 180 : 220))

const stats = computed(() =>
  helixSegments.value.filter((s) => s.active).map((s) => ({ label: s.label, value: s.detail }))
)
</script>

<template>
  <div class="selection-overview" :class="{ compact, 'helix-full': isFullHelix }">
    <div class="overview-header">
      <h3 class="overview-title">Your Platform DNA</h3>
      <div class="completion-bar">
        <div class="completion-fill" :style="{ width: `${Math.round(completion * 100)}%` }" />
        <span class="completion-label">{{ Math.round(completion * 100) }}% profile</span>
      </div>
    </div>

    <div class="overview-body">
      <DnaHelixView
        :segments="helixSegments"
        :completion="completion"
        :is-full="isFullHelix"
        :height="helixHeight"
      />

      <div class="overview-stats">
        <div v-for="s in stats" :key="s.label" class="stat-chip">
          <span class="stat-chip-label">{{ s.label }}</span>
          <span class="stat-chip-value">{{ s.value }}</span>
        </div>
      </div>

      <div v-if="!compact" class="overview-links">
        <a
          v-for="v in resolvedRancherVersions"
          :key="'rv-' + v"
          :href="rancherRelease(v)"
          target="_blank"
          rel="noopener noreferrer"
          class="link-chip"
        >Rancher {{ v }} ↗</a>
        <a v-if="cni" :href="cniDef.docsUrl" target="_blank" rel="noopener noreferrer" class="link-chip">{{ cniDef.label }} docs ↗</a>
        <a :href="githubRelease(STACK_COMPONENTS.coredns.repo)" target="_blank" rel="noopener noreferrer" class="link-chip">CoreDNS ↗</a>
        <a :href="githubRelease(STACK_COMPONENTS.fleet.repo)" target="_blank" rel="noopener noreferrer" class="link-chip">Fleet ↗</a>
        <template v-if="hasK3s">
          <a
            v-for="v in resolvedK3sVersions.slice(0, 8)"
            :key="'k3s-' + v"
            :href="k3sRelease(v)"
            target="_blank"
            rel="noopener noreferrer"
            class="link-chip"
          >K3s {{ v }} ↗</a>
        </template>
        <template v-if="hasRke2">
          <a
            v-for="v in resolvedRke2Versions.slice(0, 8)"
            :key="'rke2-' + v"
            :href="rke2Release(v)"
            target="_blank"
            rel="noopener noreferrer"
            class="link-chip"
          >RKE2 {{ v }} ↗</a>
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.selection-overview {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.overview-header {
  margin: 0;
}
.overview-title {
  margin: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}
.completion-bar {
  height: 4px;
  background: var(--border);
  border-radius: 2px;
  overflow: hidden;
  margin-top: 0.35rem;
}
.completion-fill {
  height: 100%;
  background: linear-gradient(90deg, var(--accent), var(--green));
  border-radius: 2px;
  transition: width 0.45s ease;
}
.completion-label {
  display: block;
  margin-top: 0.25rem;
  font-size: 0.68rem;
  color: var(--text-muted);
}
.helix-full .completion-fill {
  animation: helix-pulse 2.5s ease-in-out infinite;
}
@keyframes helix-pulse {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.78; }
}
.overview-body {
  display: flex;
  flex-direction: column;
  gap: 0.65rem;
}
.overview-stats {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.stat-chip {
  display: inline-flex;
  flex-direction: column;
  gap: 1px;
  padding: 0.28rem 0.5rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--panel);
  min-width: 0;
}
.stat-chip-label {
  font-size: 0.6rem;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  color: var(--text-muted);
}
.stat-chip-value {
  font-size: 0.76rem;
  font-weight: 600;
  color: var(--text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 120px;
}
.overview-links {
  display: flex;
  flex-wrap: wrap;
  gap: 0.35rem;
}
.link-chip {
  font-size: 0.7rem;
  padding: 0.18rem 0.4rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--bg);
  color: var(--accent);
  text-decoration: none;
  white-space: nowrap;
}
.link-chip:hover {
  border-color: var(--accent);
}
.compact .overview-links {
  display: none;
}
</style>
