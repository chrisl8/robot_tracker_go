import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'

// One id per build, baked into the bundle (__BUILD_ID__) and written beside it
// as version.json, which the Go server serves at /api/version. A page whose id
// differs from the server's is running an older build and shows a refresh banner.
const buildId = new Date().toISOString()

export default defineConfig({
    define: { __BUILD_ID__: JSON.stringify(buildId) },
    plugins: [
        vue(),
        {
            name: 'emit-version',
            generateBundle() {
                this.emitFile({
                    type: 'asset',
                    fileName: 'version.json',
                    source: JSON.stringify({ id: buildId }),
                })
            },
        },
    ],
    resolve: {
        alias: {
            '@': resolve(__dirname, 'src'),
        },
    },
    server: {
        port: 5173,
        proxy: {
            '/api': {
                target: 'http://localhost:9086',
                changeOrigin: true,
            },
            '/ws': {
                target: 'ws://localhost:9086',
                ws: true,
            },
        },
    },
    build: {
        outDir: '../internal/ui/static',
        emptyOutDir: true,
        sourcemap: false,
        chunkSizeWarningLimit: 2000,
    },
})
