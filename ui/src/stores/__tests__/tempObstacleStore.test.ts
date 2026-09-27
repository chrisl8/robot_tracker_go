import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import type { TempObstaclesPayload } from '@/types/api'
import { useRobotStore } from '../robotStore'
import { useTempObstacleStore } from '../tempObstacleStore'
import { useUIStore } from '../uiStore'

function payload(overrides: Partial<TempObstaclesPayload> = {}): TempObstaclesPayload {
    return {
        obstacles: [
            {
                id: 'temp_1',
                pixel_top_left: [100, 100],
                pixel_bottom_right: [200, 200],
                world_top_left: [0, 0],
                world_bottom_right: [0.2, 0.2],
            },
        ],
        applied: true,
        warming: false,
        guarded: false,
        enabled: true,
        ...overrides,
    }
}

function jsonResponse(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })
}

function setup() {
    setActivePinia(createPinia())
    const store = useTempObstacleStore()
    const ui = useUIStore()
    const toast = vi.spyOn(ui, 'showToast')
    return { store, ui, toast }
}

describe('temp_obstacles WebSocket message', () => {
    it('is routed into the store by the robot store', () => {
        setActivePinia(createPinia())
        const robot = useRobotStore()
        const store = useTempObstacleStore()

        robot.handleWebSocketMessage({ type: 'temp_obstacles', temp_obstacles: payload() })

        expect(store.count).toBe(1)
        expect(store.obstacles[0]?.id).toBe('temp_1')
        expect(store.enabled).toBe(true)
        expect(store.applied).toBe(true)
    })

    it('carries the warming and guarded flags and treats missing data as empty', () => {
        const { store } = setup()
        store.setFromMessage(payload({ obstacles: [], warming: true, guarded: true }))
        expect(store.count).toBe(0)
        expect(store.warming).toBe(true)
        expect(store.guarded).toBe(true)

        store.setFromMessage({ ...payload(), obstacles: undefined as never })
        expect(store.obstacles).toEqual([])
    })

    it('clears the absorb prompt once its obstacle is gone', () => {
        const { store } = setup()
        store.setFromMessage(payload())
        store.promptAbsorb({ x: 150, y: 150, canvasX: 10, canvasY: 10 })
        store.setFromMessage(payload())
        expect(store.absorbPrompt).not.toBeNull()

        store.setFromMessage(payload({ obstacles: [] }))
        expect(store.absorbPrompt).toBeNull()
    })
})

describe('controls', () => {
    let fetchMock: ReturnType<typeof vi.fn>

    beforeEach(() => {
        fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)
    })

    afterEach(() => {
        vi.unstubAllGlobals()
    })

    it('loads the detector state at start-up', async () => {
        const { store } = setup()
        fetchMock.mockResolvedValue(
            jsonResponse(200, {
                enabled: true,
                applied: false,
                warming: true,
                guarded: false,
                count: 0,
            })
        )
        await store.loadState()
        expect(fetchMock).toHaveBeenCalledWith('/api/foreground/state')
        expect(store.enabled).toBe(true)
        expect(store.applied).toBe(false)
        expect(store.warming).toBe(true)
    })

    it('says why the state could not be loaded', async () => {
        const { store, toast } = setup()
        fetchMock.mockResolvedValue(jsonResponse(500, { error: 'detector crashed' }))
        await store.loadState()
        expect(toast).toHaveBeenCalledWith(expect.stringContaining('detector crashed'), 'error')

        fetchMock.mockRejectedValue(new Error('offline'))
        await store.loadState()
        expect(toast).toHaveBeenLastCalledWith(expect.stringContaining('Could not reach'), 'error')
    })

    it('toggles detection and posts the new value', async () => {
        const { store } = setup()
        fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))
        await store.setEnabled(true)
        expect(fetchMock).toHaveBeenCalledWith(
            '/api/foreground/enabled',
            expect.objectContaining({ method: 'POST', body: JSON.stringify({ enabled: true }) })
        )
        expect(store.enabled).toBe(true)
    })

    it('leaves the toggle unchanged and explains a rejected request', async () => {
        const { store, toast } = setup()
        store.setFromMessage(payload({ applied: false }))
        fetchMock.mockResolvedValue(jsonResponse(400, { error: 'not calibrated' }))
        await store.setApply(true)
        expect(store.applied).toBe(false)
        expect(toast).toHaveBeenCalledWith(
            'Changing obstacle steering failed: not calibrated',
            'error'
        )
    })

    it('reports a network failure with a specific message', async () => {
        const { store, toast } = setup()
        fetchMock.mockRejectedValue(new Error('offline'))
        await store.setEnabled(false)
        expect(toast).toHaveBeenCalledWith(
            expect.stringContaining('could not reach the tracker service'),
            'error'
        )
    })

    it('reset posts with no body and tells the user the floor should be clear', async () => {
        const { store, toast } = setup()
        fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))
        await store.resetBackground()
        expect(fetchMock).toHaveBeenCalledWith(
            '/api/foreground/reset',
            expect.objectContaining({ method: 'POST', body: undefined })
        )
        expect(store.warming).toBe(true)
        expect(toast).toHaveBeenCalledWith(expect.stringContaining('floor should be clear'), 'info')
    })

    it('absorb posts the natural-frame point and closes the prompt', async () => {
        const { store, toast } = setup()
        store.promptAbsorb({ x: 150, y: 160, canvasX: 5, canvasY: 6 })
        fetchMock.mockResolvedValue(jsonResponse(200, { status: 'ok' }))

        expect(await store.absorb(150, 160)).toBe(true)

        expect(fetchMock).toHaveBeenCalledWith(
            '/api/foreground/absorb',
            expect.objectContaining({ body: JSON.stringify({ x: 150, y: 160 }) })
        )
        expect(store.absorbPrompt).toBeNull()
        expect(toast).toHaveBeenCalledWith('Obstacle treated as floor', 'success')
    })

    it('absorb failure is reported and still closes the prompt', async () => {
        const { store, toast } = setup()
        store.promptAbsorb({ x: 1, y: 2, canvasX: 0, canvasY: 0 })
        fetchMock.mockResolvedValue(jsonResponse(404, { error: 'no obstacle there' }))
        expect(await store.absorb(1, 2)).toBe(false)
        expect(toast).toHaveBeenCalledWith(
            'Absorbing the obstacle failed: no obstacle there',
            'error'
        )
        expect(store.absorbPrompt).toBeNull()
    })

    it('toggles the mask preview', () => {
        const { store } = setup()
        expect(store.showMask).toBe(false)
        store.toggleMask()
        expect(store.showMask).toBe(true)
    })
})
