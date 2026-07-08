<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { marked } from 'marked'
import LoadingShapes from './LoadingShapes.vue'

const props = defineProps<{
  initialDoc?: string
}>()

const DOC_PAGES = [
  { id: 'README', label: 'Overview', file: 'README.md' },
  { id: 'getting-started', label: 'Getting started', file: 'getting-started.md' },
  { id: 'cli', label: 'CLI (imperative)', file: 'cli.md' },
  { id: 'config', label: 'Config (YAML)', file: 'config.md' },
  { id: 'api', label: 'REST API', file: 'api.md' },
  { id: 'web-ui', label: 'Web UI', file: 'web-ui.md' },
  { id: 'outputs', label: 'Outputs & exports', file: 'outputs.md' },
  { id: 'airgap-testing', label: 'Air-gap testing', file: 'airgap-testing.md' },
  { id: 'deploy', label: 'Deployment', file: 'deploy.md' },
  { id: 'IMAGE_LIST_DECISIONS', label: 'Image list decisions', file: 'IMAGE_LIST_DECISIONS.md' },
] as const

const activeId = ref(props.initialDoc ?? 'README')
const html = ref('')
const loading = ref(false)
const error = ref('')

marked.setOptions({ gfm: true, breaks: true })

function rewriteAssetPaths(md: string): string {
  return md
    .replace(/\]\(assets\//g, '](/docs/assets/')
    .replace(/\]\(\.\.\/config\.example\.yaml\)/g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/config.example.yaml)')
    .replace(/\]\(\.\.\/deploy\//g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/deploy/')
    .replace(/\]\(config\.example\.yaml\)/g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/config.example.yaml)')
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
  try {
    const r = await fetch(`/docs/${page.file}`)
    if (!r.ok) throw new Error(`${r.status} ${r.statusText}`)
    const md = await r.text()
    html.value = marked.parse(rewriteAssetPaths(md)) as string
    activeId.value = id
    if (typeof window !== 'undefined') {
      window.location.hash = id === 'README' ? 'docs' : `docs/${id}`
    }
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
</script>

<template>
  <div class="docs-layout">
    <aside class="docs-sidebar">
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
            <a href="https://github.com/aeltai/Hangar-Genesis/tree/main/docs" target="_blank" rel="noopener">docs/ on GitHub</a>
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
      <article
        v-else
        class="docs-article markdown-body"
        v-html="html"
        @click="onDocClick"
      />
    </div>
  </div>
</template>

<style scoped>
.docs-layout {
  display: grid;
  grid-template-columns: minmax(280px, 320px) minmax(0, 1fr);
  min-height: calc(100vh - 108px - var(--footer-h));
  align-items: stretch;
}

.docs-sidebar {
  padding: 1.25rem 1.5rem;
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
  border-right: 1px solid var(--border);
  overflow-y: auto;
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
  padding: 1.5rem 2rem;
  overflow-y: auto;
  background: var(--panel);
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
  max-width: min(100%, 960px);
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

@media (max-width: 900px) {
  .docs-layout {
    grid-template-columns: 1fr;
    min-height: auto;
  }

  .docs-sidebar {
    border-right: none;
    border-bottom: 1px solid var(--border);
  }

  .sidebar-links {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(160px, 1fr));
    gap: 0.15rem;
  }

  .docs-main {
    padding: 1.25rem 1.5rem;
  }
}
</style>
