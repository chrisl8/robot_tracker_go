import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { effectScope, nextTick, ref } from 'vue'
import { useMaskPreview } from '../useMaskPreview'

function jpeg(): Response {
    return new Response(new Blob([new Uint8Array([0xff, 0xd8, 0xff])]), {
        status: 200,
        headers: { 'Content-Type': 'image/jpeg' },
    })
}

describe('useMaskPreview', () => {
    let fetchMock: ReturnType<typeof vi.fn>
    let created: string[]
    let revoked: string[]

    beforeEach(() => {
        vi.useFakeTimers()
        fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)
        created = []
        revoked = []
        let n = 0
        vi.stubGlobal('URL', {
            createObjectURL: () => {
                const u = `blob:mask-${++n}`
                created.push(u)
                return u
            },
            revokeObjectURL: (u: string) => revoked.push(u),
        })
    })

    afterEach(() => {
        vi.useRealTimers()
        vi.unstubAllGlobals()
    })

    function mount(active: boolean) {
        const flag = ref(active)
        const onError = vi.fn()
        const scope = effectScope()
        const api = scope.run(() => useMaskPreview(flag, onError))
        if (!api) throw new Error('scope failed')
        return { flag, onError, scope, ...api }
    }

    it('does nothing while inactive', async () => {
        mount(false)
        await vi.advanceTimersByTimeAsync(3000)
        expect(fetchMock).not.toHaveBeenCalled()
    })

    it('polls about twice a second while active and swaps blob URLs', async () => {
        fetchMock.mockImplementation(() => Promise.resolve(jpeg()))
        const { url } = mount(true)
        await vi.advanceTimersByTimeAsync(1100)

        expect(fetchMock).toHaveBeenCalledTimes(3)
        expect(fetchMock).toHaveBeenCalledWith('/api/foreground/debug.jpg')
        expect(url.value).toBe(created[created.length - 1])
        // Every replaced URL was revoked, only the latest is live.
        expect(revoked).toEqual(created.slice(0, -1))
    })

    it('ignores 204 (no image rendered yet)', async () => {
        fetchMock.mockImplementation(() => Promise.resolve(new Response(null, { status: 204 })))
        const { url, onError } = mount(true)
        await vi.advanceTimersByTimeAsync(1100)
        expect(url.value).toBeNull()
        expect(onError).not.toHaveBeenCalled()
    })

    it('stops polling and revokes the URL when turned off', async () => {
        fetchMock.mockImplementation(() => Promise.resolve(jpeg()))
        const { flag, url } = mount(true)
        await vi.advanceTimersByTimeAsync(600)
        expect(url.value).not.toBeNull()

        flag.value = false
        await nextTick()
        const calls = fetchMock.mock.calls.length
        await vi.advanceTimersByTimeAsync(3000)

        expect(fetchMock.mock.calls.length).toBe(calls)
        expect(url.value).toBeNull()
        expect(revoked).toContain(created[created.length - 1])
    })

    it('reports a failure once, not on every poll', async () => {
        fetchMock.mockImplementation(() =>
            Promise.resolve(new Response('nope', { status: 500, statusText: 'Server Error' }))
        )
        const { onError } = mount(true)
        await vi.advanceTimersByTimeAsync(3000)
        expect(onError).toHaveBeenCalledTimes(1)
        expect(onError).toHaveBeenCalledWith(expect.stringContaining('500'))
    })

    it('stops when its scope is disposed', async () => {
        fetchMock.mockImplementation(() => Promise.resolve(jpeg()))
        const { scope } = mount(true)
        await vi.advanceTimersByTimeAsync(600)
        scope.stop()
        const calls = fetchMock.mock.calls.length
        await vi.advanceTimersByTimeAsync(3000)
        expect(fetchMock.mock.calls.length).toBe(calls)
    })
})
