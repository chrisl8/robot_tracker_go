import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createApp, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import ObstaclePanel from '../ObstaclePanel.vue'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import type { Obstacle } from '@/types/api'

function obstacle(id: string): Obstacle {
    return {
        id,
        name: id,
        pixel_top_left: [0, 0],
        pixel_bottom_right: [10, 10],
        world_top_left: [0, 0],
        world_bottom_right: [0.1, 0.1],
        clearance: 0.02,
    }
}

describe('ObstaclePanel delete', () => {
    let app: App
    let root: HTMLElement
    let fetchMock: ReturnType<typeof vi.fn>

    beforeEach(() => {
        // Shape of the real DELETE /api/obstacles/:id response.
        fetchMock = vi.fn().mockResolvedValue({
            ok: true,
            json: async () => ({ status: 'ok', id: 'a', count: 1 }),
        })
        vi.stubGlobal('fetch', fetchMock)
        vi.spyOn(console, 'error').mockImplementation(() => {})

        const pinia = createPinia()
        setActivePinia(pinia)
        useObstacleStore().setObstacles([obstacle('a'), obstacle('b')])

        root = document.createElement('div')
        document.body.appendChild(root)
        app = createApp(ObstaclePanel)
        app.use(pinia)
        app.mount(root)
    })

    afterEach(() => {
        app.unmount()
        root.remove()
        vi.unstubAllGlobals()
        vi.restoreAllMocks()
    })

    it('reports success (not a false failure) and drops the deleted row', async () => {
        const deleteBtn = root.querySelector('.obstacle-item button')!
        deleteBtn.dispatchEvent(new MouseEvent('click'))
        await vi.waitFor(() => expect(useUIStore().toasts.length).toBeGreaterThan(0))

        const toasts = useUIStore().toasts
        expect(toasts.map(t => t.type)).toEqual(['success'])
        expect(useObstacleStore().obstacles.map(o => o.id)).toEqual(['b'])
    })
})
