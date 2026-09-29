import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createApp, defineComponent, h } from 'vue'
import { createPinia } from 'pinia'
import { useWebSocket } from '../useWebSocket'

class FakeWebSocket {
    static instances: FakeWebSocket[] = []
    static OPEN = 1
    readyState = 0 // CONNECTING
    onopen: (() => void) | null = null
    onclose: (() => void) | null = null
    onerror: ((e: unknown) => void) | null = null
    onmessage: ((e: { data: string }) => void) | null = null
    closed = false
    constructor(public url: string) {
        FakeWebSocket.instances.push(this)
    }
    close(): void {
        this.closed = true
        // Real sockets deliver onclose asynchronously after close().
        setTimeout(() => this.onclose?.(), 0)
    }
}

describe('useWebSocket', () => {
    beforeEach(() => {
        FakeWebSocket.instances = []
        vi.stubGlobal('WebSocket', FakeWebSocket)
        vi.useFakeTimers()
    })
    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    function mountComposable() {
        let api!: ReturnType<typeof useWebSocket>
        const Comp = defineComponent({
            setup() {
                api = useWebSocket('ws://test/ws')
                return () => h('div')
            },
        })
        const root = document.createElement('div')
        const app = createApp(Comp)
        app.use(createPinia())
        app.mount(root)
        return { app, api }
    }

    it('reconnects after an unexpected close', async () => {
        const { app } = mountComposable()
        expect(FakeWebSocket.instances).toHaveLength(1)

        FakeWebSocket.instances[0].onclose?.() // server went away

        await vi.advanceTimersByTimeAsync(60_000)
        expect(FakeWebSocket.instances.length).toBeGreaterThan(1)
        app.unmount()
    })

    it('does not reconnect after the app unmounts', async () => {
        const { app } = mountComposable()
        expect(FakeWebSocket.instances).toHaveLength(1)

        app.unmount() // closes the socket on purpose
        await vi.advanceTimersByTimeAsync(120_000)

        expect(FakeWebSocket.instances).toHaveLength(1)
    })
})
