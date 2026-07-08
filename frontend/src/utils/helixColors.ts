/** Distinct colors per stack layer in the selection helix. */
export const HELIX_SEGMENT_COLORS: Record<string, string> = {
  rancher: '#2f6fed',
  k3s: '#0ea5e9',
  rke2: '#2a9d6a',
  platform: '#22c55e',
  linux: '#22c55e',
  windows: '#8b5cf6',
  cni: '#f59e0b',
  lb: '#64748b',
  charts: '#eab308',
  images: '#14b8a6',
}

export function helixColor(id: string): string {
  return HELIX_SEGMENT_COLORS[id] ?? '#9aa3b2'
}

function hexToRgb(hex: string): [number, number, number] {
  const h = hex.replace('#', '')
  const n = parseInt(h.length === 3 ? h.replace(/./g, '$&$&') : h, 16)
  return [(n >> 16) & 255, (n >> 8) & 255, n & 255]
}

function rgbToHex(r: number, g: number, b: number): string {
  return `#${[r, g, b].map((v) => Math.round(v).toString(16).padStart(2, '0')).join('')}`
}

function mixRgb(a: [number, number, number], b: [number, number, number], t: number): [number, number, number] {
  return [
    a[0] + (b[0] - a[0]) * t,
    a[1] + (b[1] - a[1]) * t,
    a[2] + (b[2] - a[2]) * t,
  ]
}

/** Lighter/darker variants so multiple versions of the same component stay distinguishable. */
export function helixColorShade(baseHex: string, index: number, total: number): string {
  if (total <= 1) return baseHex
  const base = hexToRgb(baseHex)
  const white: [number, number, number] = [255, 255, 255]
  const black: [number, number, number] = [0, 0, 0]
  const t = index / (total - 1)
  const mid = 0.5
  const mixed =
    t < mid
      ? mixRgb(base, white, (mid - t) * 0.35)
      : mixRgb(base, black, (t - mid) * 0.28)
  return rgbToHex(mixed[0], mixed[1], mixed[2])
}

export const HELIX_GREY = '#4a5568'
export const HELIX_GREY_DIM = '#2d333d'
