import fs from 'node:fs'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig, type Plugin } from 'vite'
import vue from '@vitejs/plugin-vue'

const rootDir = path.dirname(fileURLToPath(import.meta.url))
const openapiPath = path.resolve(rootDir, 'public/openapi.yaml')

const devSwaggerHtml = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <title>GenesisRK API — Swagger UI</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui.css" />
  <style>body { margin: 0; background: #fafafa; }</style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5.11.0/swagger-ui-bundle.js"></script>
  <script>
    window.onload = function () {
      window.ui = SwaggerUIBundle({
        url: '/api/openapi.yaml',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
        layout: 'StandaloneLayout',
        tryItOutEnabled: true,
        displayRequestDuration: true,
        filter: true,
      });
    };
  </script>
</body>
</html>`

/** Serve OpenAPI + Swagger UI in dev when the Go backend is stale or not running. */
function genesisDevApiPlugin(): Plugin {
  return {
    name: 'genesis-dev-api',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const pathname = (req.url ?? '').split('?')[0]
        const isOpenApi = pathname === '/api/openapi.yaml' || pathname === '/api/openapi'
        if (isOpenApi) {
          if (!fs.existsSync(openapiPath)) return next()
          res.statusCode = 200
          res.setHeader('Content-Type', 'application/yaml; charset=utf-8')
          if (req.method === 'HEAD') {
            res.setHeader('Content-Length', String(fs.statSync(openapiPath).size))
            res.end()
            return
          }
          if (req.method === 'GET') {
            fs.createReadStream(openapiPath).pipe(res)
            return
          }
          res.statusCode = 405
          res.end('Method Not Allowed')
          return
        }
        if (pathname === '/api/docs') {
          res.statusCode = 200
          res.setHeader('Content-Type', 'text/html; charset=utf-8')
          if (req.method === 'HEAD') {
            res.end()
            return
          }
          if (req.method === 'GET') {
            res.end(devSwaggerHtml)
            return
          }
          res.statusCode = 405
          res.end('Method Not Allowed')
          return
        }
        next()
      })
    },
  }
}

// https://vite.dev/config/
export default defineConfig({
  plugins: [vue(), genesisDevApiPlugin()],
  server: {
    proxy: {
      '/api': {
        target: 'http://localhost:8080',
        changeOrigin: true,
        bypass(req) {
          const pathname = (req.url ?? '').split('?')[0]
          if (
            pathname === '/api/openapi.yaml' ||
            pathname === '/api/openapi' ||
            pathname === '/api/docs'
          ) {
            return req.url
          }
        },
      },
    },
  },
})
