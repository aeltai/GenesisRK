<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'

const host = ref<HTMLElement | null>(null)
let swaggerLoaded = false

declare global {
  interface Window {
    SwaggerUIBundle?: {
      (config: Record<string, unknown>): void
      presets: { apis: unknown }
      SwaggerUIStandalonePreset?: unknown
    }
  }
}

function loadStylesheet(href: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`link[href="${href}"]`)) {
      resolve()
      return
    }
    const link = document.createElement('link')
    link.rel = 'stylesheet'
    link.href = href
    link.onload = () => resolve()
    link.onerror = () => reject(new Error(`Failed to load ${href}`))
    document.head.appendChild(link)
  })
}

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    if (document.querySelector(`script[src="${src}"]`)) {
      resolve()
      return
    }
    const script = document.createElement('script')
    script.src = src
    script.async = true
    script.onload = () => resolve()
    script.onerror = () => reject(new Error(`Failed to load ${src}`))
    document.head.appendChild(script)
  })
}

async function resolveOpenApiUrl(): Promise<string> {
  for (const url of ['/openapi.yaml', '/api/openapi.yaml', '/api/openapi']) {
    try {
      const r = await fetch(url, { method: 'HEAD' })
      if (r.ok) return url
    } catch {
      /* try next */
    }
  }
  return '/openapi.yaml'
}

onMounted(async () => {
  if (!host.value || swaggerLoaded) return
  try {
    const ver = '5.11.0'
    await loadStylesheet(`https://unpkg.com/swagger-ui-dist@${ver}/swagger-ui.css`)
    await loadScript(`https://unpkg.com/swagger-ui-dist@${ver}/swagger-ui-bundle.js`)
    if (!window.SwaggerUIBundle || !host.value) return
    const specUrl = await resolveOpenApiUrl()
    window.SwaggerUIBundle({
      url: specUrl,
      dom_id: '#swagger-ui-host',
      deepLinking: true,
      presets: [window.SwaggerUIBundle.presets.apis],
      layout: 'BaseLayout',
      tryItOutEnabled: true,
      persistAuthorization: false,
      displayRequestDuration: true,
      filter: true,
    })
    swaggerLoaded = true
  } catch (e) {
    console.error('Swagger UI failed to load', e)
  }
})

onUnmounted(() => {
  if (host.value) host.value.innerHTML = ''
})
</script>

<template>
  <section class="api-swagger-panel" aria-label="Interactive API explorer">
    <div class="api-swagger-header">
      <h2 class="api-swagger-title">Try the API</h2>
      <p class="api-swagger-desc">
        Interactive Swagger UI — requests use <strong>this server</strong> (same origin as the web UI).
        Open <a href="/api/docs" target="_blank" rel="noopener noreferrer">/api/docs</a> in a new tab for a full-page view.
      </p>
    </div>
    <div id="swagger-ui-host" ref="host" class="swagger-ui-host" />
  </section>
</template>

<style scoped>
.api-swagger-panel {
  margin-bottom: 1.5rem;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--bg);
  overflow: hidden;
}
.api-swagger-header {
  padding: 0.875rem 1rem;
  border-bottom: 1px solid var(--border);
  background: color-mix(in srgb, var(--panel) 70%, var(--bg));
}
.api-swagger-title {
  margin: 0 0 0.35rem;
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--text);
}
.api-swagger-desc {
  margin: 0;
  font-size: 0.8125rem;
  line-height: 1.5;
  color: var(--text-muted);
}
.api-swagger-desc code {
  font-family: var(--font-mono);
  font-size: 0.85em;
  background: var(--panel);
  padding: 0.1rem 0.35rem;
  border-radius: var(--radius-sm);
}
.swagger-ui-host {
  min-height: 420px;
}

@media (min-width: 1440px) {
  .swagger-ui-host {
    min-height: calc(100vh - 220px - var(--footer-h));
  }
}
.swagger-ui-host :deep(.swagger-ui) {
  font-family: var(--font-sans);
}
</style>
