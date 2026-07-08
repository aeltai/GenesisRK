<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { marked } from 'marked'

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
    const processed = rewriteAssetPaths(md)
    html.value = marked.parse(processed) as string
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

function parseHash(): string | null {
  const h = window.location.hash.slice(1)
  if (h.startsWith('docs/')) return h.slice(5) || 'README'
  if (h === 'docs') return 'README'
  return null
}

onMounted(() => {
  const fromHash = parseHash()
  loadDoc(fromHash ?? props.initialDoc ?? 'README')
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
      <p class="sidebar-desc">Guides for CLI, YAML config, REST API, and deployment.</p>
      <nav class="docs-nav">
        <ul class="docs-nav-list">
          <li v-for="page in DOC_PAGES" :key="page.id">
            <a
              :href="page.id === 'README' ? '#docs' : `#docs/${page.id}`"
              class="docs-nav-link"
              :class="{ active: activeId === page.id }"
              @click.prevent="loadDoc(page.id)"
            >{{ page.label }}</a>
          </li>
        </ul>
      </nav>
      <p class="sidebar-footer">
        <a href="https://github.com/aeltai/Hangar-Genesis/tree/main/docs" target="_blank" rel="noopener">View on GitHub</a>
      </p>
    </aside>

    <div class="docs-content">
      <div v-if="loading" class="docs-state">Loading…</div>
      <div v-else-if="error" class="docs-state docs-error">
        <p>Could not load documentation.</p>
        <p class="docs-error-detail">{{ error }}</p>
        <p class="docs-error-hint">Ensure <code>/docs/*.md</code> is served from the frontend build.</p>
      </div>
      <article
        v-else
        class="docs-body markdown-body"
        v-html="html"
        @click="onDocClick"
      />
    </div>
  </div>
</template>

<style scoped>
.docs-layout {
  display: grid;
  grid-template-columns: minmax(240px, 280px) minmax(0, 1fr);
  min-height: calc(100vh - 130px);
  align-items: stretch;
}
.docs-sidebar {
  padding: 1.25rem 1rem;
  background: color-mix(in srgb, var(--bg) 50%, var(--panel));
  border-right: 1px solid var(--border);
  overflow-y: auto;
}
.sidebar-title {
  font-size: 0.85rem;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  color: var(--cyan);
  margin: 0 0 0.35rem 0;
}
.sidebar-desc {
  margin: 0 0 1rem 0;
  font-size: 0.78rem;
  opacity: 0.65;
  line-height: 1.4;
}
.docs-nav-list {
  list-style: none;
  margin: 0;
  padding: 0;
}
.docs-nav-list li {
  margin: 0;
}
.docs-nav-link {
  display: block;
  padding: 0.45rem 0.65rem;
  margin-bottom: 2px;
  border-radius: 6px;
  color: var(--text);
  text-decoration: none;
  font-size: 0.85rem;
  border: 1px solid transparent;
  transition: background 0.15s, border-color 0.15s, color 0.15s;
}
.docs-nav-link:hover {
  background: color-mix(in srgb, var(--cyan) 8%, var(--panel));
  border-color: color-mix(in srgb, var(--cyan) 30%, var(--border));
  color: var(--cyan);
  text-decoration: none;
}
.docs-nav-link.active {
  background: color-mix(in srgb, var(--cyan) 14%, var(--panel));
  border-color: var(--cyan);
  color: var(--cyan);
  font-weight: 600;
}
.sidebar-footer {
  margin: 1.25rem 0 0;
  padding-top: 0.75rem;
  border-top: 1px solid var(--border);
  font-size: 0.75rem;
  opacity: 0.65;
}
.sidebar-footer a {
  color: var(--cyan);
}
.docs-content {
  min-width: 0;
  padding: 1.25rem 1.75rem;
  overflow-y: auto;
}
.docs-state {
  padding: 2rem 0;
  opacity: 0.75;
  font-size: 0.9rem;
}
.docs-error {
  color: var(--red);
}
.docs-error-detail {
  font-size: 0.85rem;
  opacity: 0.9;
}
.docs-error-hint {
  font-size: 0.8rem;
  opacity: 0.7;
  color: var(--text);
}
.docs-error-hint code {
  background: var(--bg);
  padding: 0.1rem 0.35rem;
  border-radius: 3px;
}

.markdown-body :deep(h1) {
  font-size: 1.5rem;
  margin: 0 0 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border);
  color: var(--text);
}
.markdown-body :deep(h2) {
  font-size: 1.2rem;
  margin: 1.75rem 0 0.75rem;
  color: var(--cyan);
}
.markdown-body :deep(h3) {
  font-size: 1.02rem;
  margin: 1.25rem 0 0.5rem;
}
.markdown-body :deep(p),
.markdown-body :deep(li) {
  line-height: 1.65;
  margin: 0.5rem 0;
  font-size: 0.9rem;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.5rem;
}
.markdown-body :deep(code) {
  background: color-mix(in srgb, var(--bg) 80%, var(--panel));
  padding: 0.12em 0.4em;
  border-radius: 4px;
  font-size: 0.88em;
}
.markdown-body :deep(pre) {
  background: var(--bg);
  border: 1px solid var(--border);
  padding: 1rem;
  border-radius: 6px;
  overflow-x: auto;
  font-size: 0.82rem;
}
.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
  border: none;
}
.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.88rem;
  margin: 1rem 0;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--border);
  padding: 0.5rem 0.75rem;
  text-align: left;
}
.markdown-body :deep(th) {
  background: color-mix(in srgb, var(--bg) 60%, var(--panel));
}
.markdown-body :deep(a) {
  color: var(--cyan);
}
.markdown-body :deep(img) {
  max-width: 100%;
  border-radius: 6px;
  margin: 0.75rem 0;
  border: 1px solid var(--border);
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid var(--cyan);
  margin: 1rem 0;
  padding-left: 1rem;
  opacity: 0.9;
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
  .docs-nav-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(140px, 1fr));
    gap: 0.25rem;
  }
  .docs-content {
    padding: 1rem;
  }
}
</style>
