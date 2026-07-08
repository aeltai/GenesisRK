<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as THREE from 'three'
import { HELIX_GREY, HELIX_GREY_DIM, helixColor } from '../utils/helixColors'

export interface HelixSegment {
  id: string
  label: string
  detail: string
  active: boolean
  color?: string
}

const props = withDefaults(
  defineProps<{
    segments: HelixSegment[]
    completion: number
    isFull: boolean
    height?: number
  }>(),
  { height: 200 }
)

const host = ref<HTMLElement | null>(null)

let renderer: THREE.WebGLRenderer | null = null
let scene: THREE.Scene | null = null
let camera: THREE.PerspectiveCamera | null = null
let helixGroup: THREE.Group | null = null
let frameId = 0
let elapsed = 0
let resizeObs: ResizeObserver | null = null
let themeObs: MutationObserver | null = null

const HELIX_TURNS_BASE = 1.45
const HELIX_TURNS_STEP = 0.07
const HELIX_RADIUS = 0.38
const HELIX_LENGTH_MIN = 4.0
const HELIX_LENGTH_STEP = 0.52

let helixLength = HELIX_LENGTH_MIN
let helixTurns = HELIX_TURNS_BASE

function fitCamera() {
  if (!camera) return
  const fovRad = (camera.fov * Math.PI) / 180
  const dist = (helixLength * 0.62) / Math.tan(fovRad / 2) + 1.4
  camera.position.set(0, 0.15, dist)
  camera.lookAt(0, 0, 0)
}

function syncHelixScale(segmentCount: number) {
  const extra = Math.max(0, segmentCount - 5)
  helixLength = HELIX_LENGTH_MIN + extra * HELIX_LENGTH_STEP
  helixTurns = HELIX_TURNS_BASE + extra * HELIX_TURNS_STEP
  fitCamera()
}

function canvasSize(): { w: number; h: number } {
  const h = props.height
  if (!host.value) return { w: 320, h }
  const w =
    host.value.clientWidth ||
    host.value.parentElement?.clientWidth ||
    320
  return { w: Math.max(w, 200), h }
}

/** Helix runs left → right (X axis); strands wave in Y/Z. */
function helixPoint(t: number, phase: number): THREE.Vector3 {
  const angle = t * helixTurns * Math.PI * 2 + phase
  const x = t * helixLength - helixLength / 2
  return new THREE.Vector3(
    x,
    HELIX_RADIUS * Math.cos(angle),
    HELIX_RADIUS * Math.sin(angle)
  )
}

function segmentColor(seg: HelixSegment): THREE.Color {
  if (!seg.active && !props.isFull) return new THREE.Color(HELIX_GREY)
  return new THREE.Color(seg.color ?? helixColor(seg.id))
}

function disposeGroup(group: THREE.Group | null) {
  if (!group) return
  group.traverse((obj) => {
    if (obj instanceof THREE.Mesh || obj instanceof THREE.Line) {
      obj.geometry?.dispose()
      const mat = obj.material
      if (Array.isArray(mat)) mat.forEach((m) => m.dispose())
      else mat?.dispose()
    }
  })
  group.clear()
}

function addTubeSegment(
  group: THREE.Group,
  t0: number,
  t1: number,
  phase: number,
  color: THREE.Color,
  lit: boolean
) {
  const pts: THREE.Vector3[] = []
  const steps = 12
  for (let i = 0; i <= steps; i++) {
    const t = t0 + (t1 - t0) * (i / steps)
    pts.push(helixPoint(t, phase))
  }
  if (pts.length < 2) return
  const curve = new THREE.CatmullRomCurve3(pts)
  const geo = new THREE.TubeGeometry(curve, steps, 0.034, 8, false)
  const mat = new THREE.MeshStandardMaterial({
    color,
    emissive: color,
    emissiveIntensity: lit ? (props.isFull ? 0.42 : 0.28) : 0,
    metalness: 0.3,
    roughness: 0.45,
    transparent: true,
    opacity: lit ? 0.95 : 0.22,
  })
  group.add(new THREE.Mesh(geo, mat))
}

function rebuildHelix() {
  if (!scene || !helixGroup) return
  syncHelixScale(props.segments.length)
  const group = helixGroup
  disposeGroup(group)

  const n = props.segments.length
  const grey = new THREE.Color(HELIX_GREY_DIM)

  // Full ghost backbone — always visible, grey until complete
  for (const phase of [0, Math.PI]) {
    const pts: THREE.Vector3[] = []
    for (let i = 0; i <= 100; i++) pts.push(helixPoint(i / 100, phase))
    const geo = new THREE.BufferGeometry().setFromPoints(pts)
    const line = new THREE.Line(
      geo,
      new THREE.LineBasicMaterial({
        color: grey,
        transparent: true,
        opacity: props.isFull ? 0.22 : 0.55,
      })
    )
    group.add(line)
  }

  // Strand segments between rungs — colored per component when active
  for (const phase of [0, Math.PI]) {
    for (let i = 0; i < n; i++) {
      const seg = props.segments[i]
      if (!seg) continue
      const t0 = i / n
      const t1 = (i + 1) / n
      const lit = seg.active || props.isFull
      const col = segmentColor(seg)
      addTubeSegment(group, t0, t1, phase, col, lit)
    }
  }

  // Base pairs (rungs) + nodes
  props.segments.forEach((seg, i) => {
    const t = n <= 1 ? 0.5 : (i + 0.5) / n
    const p0 = helixPoint(t, 0)
    const p1 = helixPoint(t, Math.PI)
    const lit = seg.active || props.isFull
    const col = segmentColor(seg)

    const rung = new THREE.Line(
      new THREE.BufferGeometry().setFromPoints([p0, p1]),
      new THREE.LineBasicMaterial({
        color: col,
        transparent: true,
        opacity: lit ? 0.92 : 0.18,
      })
    )
    group.add(rung)

    for (const p of [p0, p1]) {
      const node = new THREE.Mesh(
        new THREE.SphereGeometry(lit ? 0.06 : 0.038, 10, 10),
        new THREE.MeshStandardMaterial({
          color: col,
          emissive: col,
          emissiveIntensity: lit ? (props.isFull ? 0.5 : 0.32) : 0,
          metalness: 0.45,
          roughness: 0.4,
          transparent: true,
          opacity: lit ? 1 : 0.25,
        })
      )
      node.position.copy(p)
      group.add(node)
    }
  })
}

function initScene() {
  if (!host.value) return
  const { w, h } = canvasSize()

  scene = new THREE.Scene()
  camera = new THREE.PerspectiveCamera(38, w / h, 0.1, 100)
  // Front view — helix reads left-to-right; distance set in rebuildHelix
  camera.position.set(0, 0.15, 6)
  camera.lookAt(0, 0, 0)

  renderer = new THREE.WebGLRenderer({ antialias: true, alpha: true })
  renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2))
  renderer.setSize(w, h)
  renderer.setClearColor(0x000000, 0)
  host.value.appendChild(renderer.domElement)

  scene.add(new THREE.AmbientLight(0xffffff, 0.6))
  const key = new THREE.DirectionalLight(0xffffff, 0.9)
  key.position.set(1, 2, 3)
  scene.add(key)

  helixGroup = new THREE.Group()
  scene.add(helixGroup)

  rebuildHelix()
  animate()
}

function animate() {
  frameId = requestAnimationFrame(animate)
  elapsed += 0.016
  if (helixGroup) {
    // Oscillate instead of spinning — horizontal helix vanishes when rotated edge-on
    const sway = props.isFull ? 0.42 : 0.28
    helixGroup.rotation.y = Math.sin(elapsed * 0.55) * sway
    helixGroup.rotation.x = Math.sin(elapsed * 0.38) * 0.08
    helixGroup.rotation.z = Math.sin(elapsed * 0.47) * 0.04
    helixGroup.position.y = Math.sin(elapsed * 0.9) * 0.05
  }
  renderer?.render(scene!, camera!)
}

function resize() {
  if (!host.value || !renderer || !camera) return
  const { w, h } = canvasSize()
  camera.aspect = w / h
  camera.updateProjectionMatrix()
  renderer.setSize(w, h)
  fitCamera()
}

onMounted(() => {
  nextTick(() => {
    requestAnimationFrame(() => {
      initScene()
      resize()
      if (host.value) {
        resizeObs = new ResizeObserver(resize)
        resizeObs.observe(host.value)
      }
      // Async chunk + sidebar layout may settle one frame later
      requestAnimationFrame(resize)
    })
  })
  themeObs = new MutationObserver(() => rebuildHelix())
  themeObs.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] })
})

onUnmounted(() => {
  cancelAnimationFrame(frameId)
  resizeObs?.disconnect()
  themeObs?.disconnect()
  disposeGroup(helixGroup)
  renderer?.dispose()
  if (host.value && renderer?.domElement.parentElement === host.value) {
    host.value.removeChild(renderer.domElement)
  }
})

watch(
  () => [props.segments, props.completion, props.isFull] as const,
  () => rebuildHelix(),
  { deep: true }
)
</script>

<template>
  <div class="dna-helix-wrap" :class="{ 'helix-complete': isFull }">
    <div
      ref="host"
      class="dna-helix-canvas"
      :style="{ height: `${height}px` }"
      role="img"
      aria-label="Your Platform DNA — stack layer visualization"
    />
    <ul class="dna-helix-legend">
      <li
        v-for="seg in segments"
        :key="seg.id"
        class="legend-row"
        :class="{ active: seg.active || isFull, pending: !seg.active && !isFull }"
        :style="{ '--seg-color': seg.color ?? helixColor(seg.id) }"
      >
        <span class="legend-dot" />
        <span class="legend-text">
          <span class="legend-label">{{ seg.label }}</span>
          <span class="legend-detail">{{ seg.detail }}</span>
        </span>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.dna-helix-wrap {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: 0.5rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: color-mix(in srgb, var(--bg) 55%, var(--panel));
  padding: 0.4rem 0.5rem 0.55rem;
  overflow: hidden;
}
.dna-helix-canvas {
  width: 100%;
  min-height: 120px;
  position: relative;
  flex-shrink: 0;
}
.dna-helix-canvas :deep(canvas) {
  display: block;
  width: 100% !important;
  height: 100% !important;
}
.dna-helix-legend {
  list-style: none;
  margin: 0;
  padding: 0.4rem 0 0;
  display: flex;
  flex-direction: row;
  flex-wrap: wrap;
  gap: 0.35rem;
  border-top: 1px solid var(--border);
  max-height: 11rem;
  overflow-y: auto;
}
.legend-row {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  padding: 0.22rem 0.45rem;
  border-radius: var(--radius-md);
  border: 1px solid var(--border);
  background: var(--panel);
  transition: opacity 0.25s;
  flex: 1 1 calc(50% - 0.35rem);
  min-width: 0;
}
.legend-row.pending {
  opacity: 0.38;
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
}
.legend-row.pending .legend-label,
.legend-row.pending .legend-detail {
  color: var(--text-muted);
}
.legend-row.active {
  opacity: 1;
}
.legend-row.active .legend-dot {
  background: var(--seg-color);
  box-shadow: 0 0 8px color-mix(in srgb, var(--seg-color) 45%, transparent);
}
.legend-dot {
  flex-shrink: 0;
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: var(--border);
}
.legend-text {
  display: flex;
  flex-direction: column;
  gap: 0;
  min-width: 0;
  flex: 1;
  overflow: hidden;
}
.legend-label {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--text);
  line-height: 1.25;
}
.legend-detail {
  font-size: 0.64rem;
  color: var(--text-muted);
  line-height: 1.2;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.helix-complete {
  box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--accent) 25%, transparent);
}
</style>
