/// <reference types="vite/client" />

declare module '*.vue' {
    import type { DefineComponent } from 'vue'
    const component: DefineComponent<object, object, any>
    export default component
}

declare module 'element-plus/dist/index.css' {
    const content: string
    export default content
}

// Injected by vite.config.ts: the id of the build this bundle came from.
declare const __BUILD_ID__: string
