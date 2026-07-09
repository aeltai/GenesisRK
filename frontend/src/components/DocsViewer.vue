<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { marked } from 'marked'
import LoadingShapes from './LoadingShapes.vue'
import ApiSwaggerPanel from './ApiSwaggerPanel.vue'

const props = defineProps<{
  initialDoc?: string
}>()

const DOC_GROUPS = [
  { id: 'start', label: 'Start here' },
  { id: 'usage', label: 'Usage' },
  { id: 'operations', label: 'Operations' },
  { id: 'reference', label: 'Reference' },
] as const

type DocGroupId = (typeof DOC_GROUPS)[number]['id']

const DOC_PAGES = [
  { id: 'README', label: 'Overview', file: 'README.md' },
  {
    id: 'getting-started',
    label: 'Getting started',
    file: 'getting-started.md',
    group: 'start' as DocGroupId,
    description: 'Install GenesisRK, run your first image list (CLI, config, or web UI), and mirror for air-gap.',
  },
  {
    id: 'web-ui',
    label: 'Web UI',
    file: 'web-ui.md',
    group: 'start' as DocGroupId,
    description: 'Run the Vue frontend locally, walk through the three-step generator, and use the docs panel.',
  },
  {
    id: 'cli',
    label: 'CLI (imperative)',
    file: 'cli.md',
    group: 'usage' as DocGroupId,
    description: 'Flags and subcommands for `hangar genesis` / `genesisrk` — generate, serve, version, and TUI mode.',
  },
  {
    id: 'config',
    label: 'Config (YAML)',
    file: 'config.md',
    group: 'usage' as DocGroupId,
    description: 'Declarative YAML for CI/CD: distros, versions, chart groups, registry targets, and export options.',
  },
  {
    id: 'api',
    label: 'REST API',
    file: 'api.md',
    group: 'usage' as DocGroupId,
    description: 'HTTP endpoints for the web UI — step1 options, generate, export, scan, and interactive Swagger UI.',
  },
  {
    id: 'outputs',
    label: 'Outputs & exports',
    file: 'outputs.md',
    group: 'usage' as DocGroupId,
    description: 'What the generator produces: image lists, tree JSON, chart metadata, and export formats.',
  },
  {
    id: 'airgap-testing',
    label: 'Air-gap testing',
    file: 'airgap-testing.md',
    group: 'operations' as DocGroupId,
    description: 'Validate mirrored images offline, test registry access, and verify cluster pull behavior.',
  },
  {
    id: 'deploy',
    label: 'Deployment',
    file: 'deploy.md',
    group: 'operations' as DocGroupId,
    description: 'Docker, Azure Container Apps, custom domains, and serving the built frontend with the API.',
  },
  {
    id: 'IMAGE_LIST_DECISIONS',
    label: 'Image list decisions',
    file: 'IMAGE_LIST_DECISIONS.md',
    group: 'reference' as DocGroupId,
    description: 'How KDM, charts, and distro releases combine into the final mirror list shown to customers.',
  },
] as const

function pagesInGroup(groupId: DocGroupId) {
  return DOC_PAGES.filter((p) => p.id !== 'README' && 'group' in p && p.group === groupId)
}

function docHref(id: string) {
  return id === 'README' ? '#docs' : `#docs/${id}`
}

const activeId = ref(props.initialDoc ?? 'README')
const sidebarOpen = ref(false)
const html = ref('')
const loading = ref(false)
const error = ref('')

marked.setOptions({ gfm: true, breaks: true })

function rewriteAssetPaths(md: string): string {
  return md
    .replace(/\]\(assets\//g, '](/docs/assets/')
    .replace(/\]\(\.\.\/config\.example\.yaml\)/g, '](https://github.com/aeltai/GenesisRK/blob/main/generate-list-config.example.yaml)')
    .replace(/\]\(\.\.\/deploy\//g, '](https://github.com/aeltai/GenesisRK/blob/main/deploy/')
    .replace(/\]\(config\.example\.yaml\)/g, '](https://github.com/aeltai/GenesisRK/blob/main/generate-list-config.example.yaml)')
    .replace(/\]\(([^)]+\.md)\)/g, (_m, path: string) => {
      const name = path.replace(/^.*\//, '').replace('.md', '')
      const page = DOC_PAGES.find((p) => p.file === path || p.id === name || p.file === `${name}.md`)
      if (page) return `](#docs/${page.id})`
      return `](${path})`
    })
}

async function loadDoc(id: string) {
  const page = DOC_PAGES.find((p) => p.id === id)
  if (!page) return
  loading.value = true
  error.value = ''
  html.value = ''
  activeId.value = id
  sidebarOpen.value = false
  if (typeof window !== 'undefined') {
    window.location.hash = id === 'README' ? 'docs' : `docs/${id}`
  }
  if (id === 'README') {
    loading.value = false
    return
  }
  try {
    const r = await fetch(`/docs/${page.file}`)
    if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
    const md = await r.text()
    html.value = marked.parse(rewriteAssetPaths(md)) as string
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function onDocClick(e: MouseEvent) {
  const target = (e.target as HTMLElement).closest('a')
  if (!target) return
  const href = target.getAttribute('href')
  if (href?.startsWith('#docs/')) {
    const id = href.slice(6)
    if (DOC_PAGES.some((p) => p.id === id)) {
      e.preventDefault()
      loadDoc(id)
    }
  } else if (href === '#docs') {
    e.preventDefault()
    loadDoc('README')
  }
}

function normalizeHash(raw: string): string {
  return raw.replace(/^\/+/, '')
}

function parseHash(): string | null {
  const h = normalizeHash(window.location.hash.slice(1))
  if (h.startsWith('docs/')) return h.slice(5) || 'README'
  if (h === 'docs') return 'README'
  return null
}

onMounted(() => {
  loadDoc(parseHash() ?? props.initialDoc ?? 'README')
  window.addEventListener('hashchange', onHashChange)
})
onUnmounted(() => {
  window.removeEventListener('hashchange', onHashChange)
})

function onHashChange() {
  const id = parseHash()
  if (id && id !== activeId.value) loadDoc(id)
}

function goToGenerator() {
  window.location.hash = ''
}

const activePageLabel = computed(() =>
  DOC_PAGES.find((p) => p.id === activeId.value)?.label ?? 'Documentation'
)
</script>

<template>
  <div class="docs-layout">
    <button
      type="button"
      class="docs-menu-toggle"
      :aria-expanded="sidebarOpen"
      aria-controls="docs-sidebar-nav"
      @click="sidebarOpen = !sidebarOpen"
    >
      <span class="docs-menu-icon" aria-hidden="true">
        <span /><span /><span />
      </span>
      <span class="docs-menu-label">{{ activePageLabel }}</span>
    </button>

    <div
      v-if="sidebarOpen"
      class="docs-sidebar-backdrop"
      aria-hidden="true"
      @click="sidebarOpen = false"
    />

    <aside
      id="docs-sidebar-nav"
      class="docs-sidebar"
      :class="{ open: sidebarOpen }"
    >
      <h3 class="sidebar-title">Documentation</h3>
      <p class="sidebar-desc">CLI, YAML config, REST API, deployment, and air-gap workflows.</p>

      <div class="sidebar-section">
        <h4 class="sidebar-heading">Guides</h4>
        <nav>
          <ul class="sidebar-links">
            <li v-for="page in DOC_PAGES" :key="page.id">
              <a
                :href="page.id === 'README' ? '#docs' : `#docs/${page.id}`"
                class="sidebar-link"
                :class="{ active: activeId === page.id }"
                @click.prevent="loadDoc(page.id)"
              >{{ page.label }}</a>
            </li>
          </ul>
        </nav>
      </div>

      <div class="sidebar-section sidebar-footer">
        <h4 class="sidebar-heading">Source</h4>
        <ul class="sidebar-links">
          <li>
            <a href="https://github.com/aeltai/GenesisRK/tree/main/docs" target="_blank" rel="noopener">docs/ on GitHub</a>
          </li>
        </ul>
      </div>
    </aside>

    <div class="docs-main">
      <div v-if="loading" class="docs-state">
        <LoadingShapes size="md" label="Loading documentation…" />
      </div>
      <div v-else-if="error" class="docs-state docs-error">
        <p class="docs-error-title">Could not load this page</p>
        <p class="docs-error-detail">{{ error }}</p>
        <p class="docs-error-hint">Run <code>npm run sync-docs</code> and ensure <code>/docs/*.md</code> is served.</p>
      </div>
      <div
        v-else
        class="docs-content"
        :class="{ 'docs-content-api': activeId === 'api', 'docs-content-overview': activeId === 'README' }"
      >
        <article v-if="activeId === 'README'" class="docs-article docs-overview">
          <h1>Table of contents</h1>
          <p class="docs-overview-intro">
            GenesisRK builds air-gapped Rancher image lists from KDM, charts, and distro releases.
            Pick a guide below — or use the sidebar to jump between sections.
          </p>
          <div class="docs-toc-grid">
            <section v-for="group in DOC_GROUPS" :key="group.id" class="docs-toc-section">
              <h2>{{ group.label }}</h2>
              <ul class="docs-toc-list">
                <li v-for="page in pagesInGroup(group.id)" :key="page.id">
                  <a
                    :href="docHref(page.id)"
                    class="docs-toc-link"
                    @click.prevent="loadDoc(page.id)"
                  >
                    <span class="docs-toc-title">{{ page.label }}</span>
                    <span v-if="'description' in page" class="docs-toc-desc">{{ page.description }}</span>
                  </a>
                </li>
              </ul>
            </section>
          </div>
          <p class="docs-overview-footer">
            Live app: <a href="#" @click.prevent="goToGenerator">Generator</a>
            · <a href="/api/docs" target="_blank" rel="noopener">Swagger UI</a>
            · <a href="https://github.com/aeltai/GenesisRK/tree/main/docs" target="_blank" rel="noopener">docs on GitHub</a>
          </p>
        </article>
        <template v-else>
          <ApiSwaggerPanel v-if="activeId === 'api'" class="docs-api-swagger" />
          <article
            class="docs-article markdown-body"
            :class="{ 'docs-api-reference': activeId === 'api' }"
            v-html="html"
            @click="onDocClick"
          />
        </template>
      </div>
    </div>
  </div>
</template>

<style scoped>
.docs-layout {
  display: grid;
  grid-template-columns: minmax(220px, 260px) minmax(0, 1fr);
  min-height: calc(100vh - 108px - var(--footer-h));
  align-items: stretch;
  width: 100%;
}

.docs-sidebar {
  padding: 1.25rem 1.25rem;
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
  border-right: 1px solid var(--border);
  overflow-y: auto;
  display: flex;
  flex-direction: column;
}

.sidebar-title {
  margin: 0 0 0.35rem;
  font-size: 0.8125rem;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.sidebar-desc {
  margin: 0 0 1.25rem;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.sidebar-section {
  margin-bottom: 1.25rem;
}

.sidebar-heading {
  margin: 0 0 0.5rem;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.sidebar-links {
  list-style: none;
  margin: 0;
  padding: 0;
}

.sidebar-links li {
  margin: 0 0 0.2rem;
}

.sidebar-link {
  display: block;
  padding: 0.35rem 0.5rem;
  margin-left: -0.5rem;
  border-radius: var(--radius-md);
  font-size: 0.8125rem;
  font-weight: 500;
  color: color-mix(in srgb, var(--text) 85%, transparent);
  text-decoration: none;
  border: 1px solid transparent;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}

.sidebar-link:hover {
  background: var(--panel-elevated);
  border-color: var(--border);
  color: var(--text);
  text-decoration: none;
}

.sidebar-link.active {
  background: var(--bg);
  border-color: var(--border-strong);
  color: var(--text);
  font-weight: 600;
}

.sidebar-footer {
  margin-top: auto;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
}

.sidebar-footer a {
  font-size: 0.8125rem;
  color: var(--accent);
}

.docs-main {
  min-width: 0;
  padding: 1.25rem 1.5rem;
  overflow-y: auto;
  background: var(--panel);
}

.docs-content {
  width: 100%;
  max-width: 920px;
}

.docs-content-api {
  max-width: none;
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.25rem;
  align-items: start;
}

.docs-api-reference :deep(h1) {
  margin-top: 0;
}

.docs-api-swagger {
  margin-bottom: 0;
}

.docs-content-overview {
  max-width: none;
}

.docs-overview h1 {
  font-size: 1.125rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  margin: 0 0 0.75rem;
  padding-bottom: 0.65rem;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}

.docs-overview-intro {
  margin: 0 0 1.5rem;
  font-size: 0.875rem;
  line-height: 1.6;
  color: color-mix(in srgb, var(--text) 88%, transparent);
  max-width: 72ch;
}

.docs-toc-grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: 1.5rem;
}

.docs-toc-section h2 {
  margin: 0 0 0.65rem;
  font-size: 0.75rem;
  font-weight: 600;
  letter-spacing: 0.05em;
  text-transform: uppercase;
  color: var(--text-muted);
}

.docs-toc-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: grid;
  gap: 0.5rem;
}

.docs-toc-link {
  display: block;
  padding: 0.75rem 0.875rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  text-decoration: none;
  transition: border-color 0.15s, background 0.15s;
}

.docs-toc-link:hover {
  border-color: var(--border-strong);
  background: var(--panel-elevated);
  text-decoration: none;
}

.docs-toc-title {
  display: block;
  font-size: 0.875rem;
  font-weight: 600;
  color: var(--text);
  margin-bottom: 0.25rem;
}

.docs-toc-desc {
  display: block;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--text-muted);
}

.docs-overview-footer {
  margin: 2rem 0 0;
  padding-top: 1rem;
  border-top: 1px solid var(--border);
  font-size: 0.8125rem;
  color: var(--text-muted);
}

.docs-overview-footer a {
  color: var(--accent);
  font-weight: 500;
}

.docs-state {
  padding: 2rem 0;
  font-size: 0.875rem;
  color: var(--text-muted);
}

.docs-error-title {
  margin: 0 0 0.35rem;
  font-weight: 600;
  color: var(--red);
}

.docs-error-detail {
  margin: 0.25rem 0;
  font-size: 0.8125rem;
}

.docs-error-hint {
  margin: 0.75rem 0 0;
  font-size: 0.8125rem;
  color: var(--text-muted);
}

.docs-error-hint code {
  background: var(--bg);
  padding: 0.1rem 0.35rem;
  border-radius: var(--radius-sm);
  font-family: var(--font-mono);
}

/* Markdown — matches generator panel typography */
.markdown-body :deep(h1) {
  font-size: 1.125rem;
  font-weight: 600;
  letter-spacing: -0.01em;
  margin: 0 0 1rem;
  padding-bottom: 0.65rem;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}

.markdown-body :deep(h2) {
  font-size: 0.9375rem;
  font-weight: 600;
  margin: 1.75rem 0 0.65rem;
  color: var(--text);
  letter-spacing: 0.01em;
}

.markdown-body :deep(h3) {
  font-size: 0.875rem;
  font-weight: 600;
  margin: 1.25rem 0 0.5rem;
  color: var(--text);
}

.markdown-body :deep(p),
.markdown-body :deep(li) {
  font-size: 0.8125rem;
  line-height: 1.6;
  margin: 0.45rem 0;
  color: color-mix(in srgb, var(--text) 92%, transparent);
}

.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.25rem;
  margin: 0.5rem 0;
}

.markdown-body :deep(code) {
  font-family: var(--font-mono);
  background: var(--bg);
  padding: 0.1rem 0.35rem;
  border-radius: var(--radius-sm);
  font-size: 0.85em;
  border: 1px solid var(--border);
}

.markdown-body :deep(pre) {
  background: var(--bg);
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  padding: 0.875rem 1rem;
  overflow-x: auto;
  margin: 0.75rem 0;
}

.markdown-body :deep(pre code) {
  background: none;
  border: none;
  padding: 0;
  font-size: 0.8125rem;
}

.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.8125rem;
  margin: 0.75rem 0;
  display: block;
  overflow-x: auto;
}

.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--border);
  padding: 0.5rem 0.75rem;
  text-align: left;
  vertical-align: top;
}

.markdown-body :deep(th) {
  background: var(--panel-elevated);
  font-weight: 600;
  color: var(--text);
}

.markdown-body :deep(tr:nth-child(even) td) {
  background: color-mix(in srgb, var(--bg) 40%, transparent);
}

.markdown-body :deep(a) {
  color: var(--accent);
  font-weight: 500;
}

.markdown-body :deep(a:hover) {
  color: var(--accent-hover);
}

.markdown-body :deep(img) {
  max-width: 100%;
  border-radius: var(--radius-md);
  margin: 0.75rem 0;
  border: 1px solid var(--border);
}

.markdown-body :deep(blockquote) {
  margin: 0.75rem 0;
  padding: 0.65rem 0 0.65rem 0.875rem;
  border-left: 3px solid var(--accent);
  background: color-mix(in srgb, var(--bg) 50%, var(--panel));
  border-radius: 0 var(--radius-md) var(--radius-md) 0;
  color: var(--text-muted);
}

.markdown-body :deep(hr) {
  border: none;
  border-top: 1px solid var(--border);
  margin: 1.5rem 0;
}

@media (min-width: 1200px) {
  .docs-layout {
    grid-template-columns: minmax(240px, 280px) minmax(0, 1fr);
  }

  .docs-main {
    padding: 1.5rem 2rem 2rem;
  }

  .docs-content {
    max-width: 1040px;
  }

  .docs-toc-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
    gap: 1.75rem 2rem;
  }

  .docs-overview h1 {
    font-size: 1.25rem;
  }

  .markdown-body :deep(h1) {
    font-size: 1.25rem;
  }

  .markdown-body :deep(h2) {
    font-size: 1rem;
  }

  .markdown-body :deep(p),
  .markdown-body :deep(li) {
    font-size: 0.875rem;
  }
}

@media (min-width: 1440px) {
  .docs-sidebar {
    position: sticky;
    top: 0;
    align-self: start;
    max-height: calc(100vh - 108px - var(--footer-h));
  }

  .docs-content {
    max-width: 1180px;
  }

  .docs-content-overview {
    max-width: none;
  }

  .docs-toc-grid {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }

  .docs-content-api {
    grid-template-columns: minmax(0, 1.15fr) minmax(320px, 0.85fr);
    gap: 1.5rem;
  }

  .docs-api-swagger {
    position: sticky;
    top: 0;
    max-height: calc(100vh - 140px - var(--footer-h));
    overflow: auto;
  }
}

@media (min-width: 1920px) {
  .docs-layout {
    grid-template-columns: 300px minmax(0, 1fr);
  }

  .docs-main {
    padding: 2rem 3rem 2.5rem;
  }

  .docs-content:not(.docs-content-api):not(.docs-content-overview) {
    max-width: 1320px;
  }

  .docs-toc-grid {
    grid-template-columns: repeat(4, minmax(0, 1fr));
    gap: 2rem;
  }

  .docs-content-api {
    grid-template-columns: minmax(0, 1.35fr) minmax(420px, 0.65fr);
    gap: 2.5rem;
  }

  .markdown-body :deep(pre) {
    font-size: 0.875rem;
  }

  .markdown-body :deep(table) {
    font-size: 0.875rem;
  }
}

@media (max-width: 900px) {
  .docs-layout {
    grid-template-columns: 1fr;
    min-height: auto;
    position: relative;
  }

  .docs-menu-toggle {
    display: flex;
    align-items: center;
    gap: 0.65rem;
    width: 100%;
    margin: 0;
    padding: 0.65rem 1rem;
    border: none;
    border-bottom: 1px solid var(--border);
    background: color-mix(in srgb, var(--bg) 60%, var(--panel));
    color: var(--text);
    font-size: 0.875rem;
    font-weight: 600;
    text-align: left;
    cursor: pointer;
  }

  .docs-menu-icon {
    display: flex;
    flex-direction: column;
    justify-content: center;
    gap: 3px;
    width: 1rem;
    flex-shrink: 0;
  }

  .docs-menu-icon span {
    display: block;
    height: 2px;
    border-radius: 1px;
    background: currentColor;
  }

  .docs-menu-label {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }

  .docs-sidebar-backdrop {
    position: fixed;
    inset: 0;
    z-index: 280;
    background: color-mix(in srgb, var(--bg) 35%, transparent);
    backdrop-filter: blur(2px);
  }

  .docs-sidebar {
    display: none;
    position: fixed;
    top: 0;
    left: 0;
    bottom: 0;
    z-index: 290;
    width: min(88vw, 320px);
    max-height: none;
    border-right: 1px solid var(--border);
    box-shadow: 4px 0 24px color-mix(in srgb, var(--bg) 50%, transparent);
  }

  .docs-sidebar.open {
    display: flex;
  }

  .sidebar-links {
    display: block;
  }

  .sidebar-link {
    margin-left: 0;
  }

  .docs-main {
    padding: 1.25rem 1rem;
  }
}

.docs-menu-toggle {
  display: none;
}

.docs-sidebar-backdrop {
  display: none;
}

@media (max-width: 900px) {
  .docs-sidebar-backdrop {
    display: block;
  }
}
</style>
