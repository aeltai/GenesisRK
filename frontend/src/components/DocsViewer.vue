<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { marked } from 'marked'

const props = defineProps<{
  initialDoc?: string
}>()

const emit = defineEmits<{ back: [] }>()

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
  // getting-started.md references assets/ — serve from /docs/assets/
  return md
    .replace(/\]\(assets\//g, '](/docs/assets/')
    .replace(/\]\(\.\.\/config\.example\.yaml\)/g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/config.example.yaml)')
    .replace(/\]\(\.\.\/deploy\//g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/deploy/')
    .replace(/\]\(\.\.\/config\.example\.yaml\)/g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/config.example.yaml)')
    .replace(/\]\(config\.example\.yaml\)/g, '](https://github.com/aeltai/Hangar-Genesis/blob/main/config.example.yaml)')
    .replace(/\]\(([^)]+\.md)\)/g, (_m, path: string) => {
      const name = path.replace(/^.*\//, '').replace('.md', '')
      const page = DOC_PAGES.find((p) => p.file === path || p.id === name || p.file === `${name}.md`)
      if (page) return `](#${page.id})`
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
      window.location.hash = `docs/${id}`
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
  if (href?.startsWith('#') && !href.startsWith('#docs')) {
    const id = href.slice(1)
    if (DOC_PAGES.some((p) => p.id === id)) {
      e.preventDefault()
      loadDoc(id)
    }
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
  <div class="docs-viewer">
    <div class="docs-page-header">
      <a href="#" class="docs-back" @click.prevent="emit('back')">← Back to GenesisRK</a>
      <h1 class="docs-page-title">Documentation</h1>
    </div>
    <div class="docs-layout">
      <nav class="docs-nav">
        <p class="docs-nav-title">Guides</p>
        <ul>
          <li v-for="page in DOC_PAGES" :key="page.id">
            <a
              href="#"
              :class="{ active: activeId === page.id }"
              @click.prevent="loadDoc(page.id)"
            >{{ page.label }}</a>
          </li>
        </ul>
        <p class="docs-nav-note">
          Source: <a href="https://github.com/aeltai/Hangar-Genesis/tree/main/docs" target="_blank" rel="noopener">docs/ on GitHub</a>
        </p>
      </nav>
      <div class="docs-panel">
        <div v-if="loading" class="docs-loading">Loading…</div>
        <div v-else-if="error" class="docs-error">{{ error }}</div>
        <div
          v-else
          class="docs-body markdown-body"
          v-html="html"
          @click="onDocClick"
        />
      </div>
    </div>
  </div>
</template>

<style scoped>
.docs-viewer {
  max-width: 1200px;
  margin: 0 auto;
  padding: 1rem 1.5rem 3rem;
}
.docs-page-header {
  margin-bottom: 1.5rem;
}
.docs-back {
  color: var(--accent, #7eb8da);
  text-decoration: none;
  font-size: 0.9rem;
}
.docs-back:hover {
  text-decoration: underline;
}
.docs-page-title {
  margin: 0.5rem 0 0;
  font-size: 1.75rem;
  font-weight: 600;
}
.docs-layout {
  display: grid;
  grid-template-columns: 220px 1fr;
  gap: 2rem;
  align-items: start;
}
.docs-nav {
  position: sticky;
  top: 1rem;
  background: var(--panel-bg, rgba(255, 255, 255, 0.04));
  border: 1px solid var(--border, rgba(255, 255, 255, 0.08));
  border-radius: 8px;
  padding: 1rem;
}
.docs-nav-title {
  margin: 0 0 0.75rem;
  font-size: 0.75rem;
  text-transform: uppercase;
  letter-spacing: 0.05em;
  opacity: 0.6;
}
.docs-nav ul {
  list-style: none;
  margin: 0;
  padding: 0;
}
.docs-nav li {
  margin: 0.25rem 0;
}
.docs-nav a {
  display: block;
  padding: 0.35rem 0.5rem;
  border-radius: 4px;
  color: var(--text, #e0e0e0);
  text-decoration: none;
  font-size: 0.9rem;
}
.docs-nav a:hover {
  background: rgba(255, 255, 255, 0.06);
}
.docs-nav a.active {
  background: rgba(126, 184, 218, 0.15);
  color: var(--accent, #7eb8da);
}
.docs-nav-note {
  margin: 1rem 0 0;
  font-size: 0.75rem;
  opacity: 0.55;
}
.docs-panel {
  min-width: 0;
  background: var(--panel-bg, rgba(255, 255, 255, 0.04));
  border: 1px solid var(--border, rgba(255, 255, 255, 0.08));
  border-radius: 8px;
  padding: 1.5rem 2rem;
}
.docs-loading,
.docs-error {
  padding: 2rem;
  opacity: 0.7;
}
.docs-error {
  color: #f87171;
}

/* Markdown body — global-ish within component via :deep */
.markdown-body :deep(h1) {
  font-size: 1.6rem;
  margin: 0 0 1rem;
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--border, rgba(255, 255, 255, 0.1));
}
.markdown-body :deep(h2) {
  font-size: 1.25rem;
  margin: 1.75rem 0 0.75rem;
}
.markdown-body :deep(h3) {
  font-size: 1.05rem;
  margin: 1.25rem 0 0.5rem;
}
.markdown-body :deep(p),
.markdown-body :deep(li) {
  line-height: 1.6;
  margin: 0.5rem 0;
}
.markdown-body :deep(ul),
.markdown-body :deep(ol) {
  padding-left: 1.5rem;
}
.markdown-body :deep(code) {
  background: rgba(0, 0, 0, 0.25);
  padding: 0.15em 0.4em;
  border-radius: 4px;
  font-size: 0.88em;
}
.markdown-body :deep(pre) {
  background: rgba(0, 0, 0, 0.35);
  padding: 1rem;
  border-radius: 6px;
  overflow-x: auto;
  font-size: 0.85rem;
}
.markdown-body :deep(pre code) {
  background: none;
  padding: 0;
}
.markdown-body :deep(table) {
  width: 100%;
  border-collapse: collapse;
  font-size: 0.9rem;
  margin: 1rem 0;
}
.markdown-body :deep(th),
.markdown-body :deep(td) {
  border: 1px solid var(--border, rgba(255, 255, 255, 0.12));
  padding: 0.5rem 0.75rem;
  text-align: left;
}
.markdown-body :deep(th) {
  background: rgba(255, 255, 255, 0.05);
}
.markdown-body :deep(a) {
  color: var(--accent, #7eb8da);
}
.markdown-body :deep(img) {
  max-width: 100%;
  border-radius: 6px;
  margin: 0.75rem 0;
}
.markdown-body :deep(blockquote) {
  border-left: 3px solid var(--accent, #7eb8da);
  margin: 1rem 0;
  padding-left: 1rem;
  opacity: 0.85;
}

@media (max-width: 768px) {
  .docs-layout {
    grid-template-columns: 1fr;
  }
  .docs-nav {
    position: static;
  }
  .docs-panel {
    padding: 1rem;
  }
}
</style>
