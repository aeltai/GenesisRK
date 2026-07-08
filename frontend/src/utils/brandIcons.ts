/** Monochrome SVG icons — CSS inverts to white in dark mode, black in light. */
function svgData(svg: string): string {
  return `data:image/svg+xml,${encodeURIComponent(svg)}`
}

export const BRAND_ICONS = {
  calico: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#000"><path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 15v-4H7l5-7v4h4l-5 7z"/></svg>'
  ),
  canal: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="1.8"><circle cx="7" cy="12" r="3.5"/><circle cx="17" cy="12" r="3.5"/><path d="M10.5 12h3"/></svg>'
  ),
  cilium: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/cilium.svg',
  flannel: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="1.6"><rect x="4" y="6" width="16" height="4" rx="1"/><rect x="4" y="14" width="16" height="4" rx="1"/></svg>'
  ),
  allCni: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="1.6"><circle cx="12" cy="6" r="2.5"/><circle cx="6" cy="18" r="2.5"/><circle cx="18" cy="18" r="2.5"/><path d="M12 8.5L6 15.5M12 8.5l6 7"/></svg>'
  ),
  none: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="#000" stroke-width="1.6"><circle cx="12" cy="12" r="8"/><path d="M8 12h8"/></svg>'
  ),
  linux: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/linux.svg',
  windows: svgData(
    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="#000"><path d="M3 5.5 11 4v7.5H3V5.5zm0 9.5h8V21l-8-1.4V15zm9-10.6 9-1.5v9.1H12V4.4zm0 10.6h9V22l-9-1.6V15z"/></svg>'
  ),
  k3s: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/k3s.svg',
  rancher: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/rancher.svg',
  nginx: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/nginx.svg',
  traefik: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/traefikproxy.svg',
  kubernetes: 'https://cdn.jsdelivr.net/npm/simple-icons@v16/icons/kubernetes.svg',
} as const

export type BrandIconKey = keyof typeof BRAND_ICONS

export function brandIcon(key: BrandIconKey): string {
  return BRAND_ICONS[key]
}
