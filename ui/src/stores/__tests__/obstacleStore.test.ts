import { describe, it, expect, beforeEach } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useObstacleStore } from '../obstacleStore'
import type { Obstacle } from '@/types/api'

function obstacle(id: string): Obstacle {
    return {
        id,
        name: id,
        pixel_top_left: [100, 100],
        pixel_bottom_right: [200, 200],
        world_top_left: [0, 0],
        world_bottom_right: [1, 1],
        clearance: 0.05,
    }
}

describe('obstacleStore obstacles', () => {
    beforeEach(() => setActivePinia(createPinia()))

    it('setObstacles replaces the list and counts it', () => {
        const store = useObstacleStore()
        store.setObstacles([obstacle('a'), obstacle('b')])
        expect(store.obstacleCount).toBe(2)

        store.setObstacles([obstacle('c')])
        expect(store.obstacles.map(o => o.id)).toEqual(['c'])
    })

    it('removing or clearing marks the list unsaved', () => {
        const store = useObstacleStore()
        store.setObstacles([obstacle('a'), obstacle('b')])
        store.setSaved(true)

        store.removeObstacle('a')
        expect(store.obstacles.map(o => o.id)).toEqual(['b'])
        expect(store.isSaved).toBe(false)

        store.setSaved(true)
        store.clearObstacles()
        expect(store.obstacles).toHaveLength(0)
        expect(store.isSaved).toBe(false)
    })

    it('removing an unknown id changes nothing', () => {
        const store = useObstacleStore()
        store.setObstacles([obstacle('a')])
        store.setSaved(true)

        store.removeObstacle('nope')

        expect(store.obstacles).toHaveLength(1)
        expect(store.isSaved).toBe(true)
    })
})

describe('obstacleStore drawing', () => {
    beforeEach(() => setActivePinia(createPinia()))

    // startDrawing silently does nothing unless drawing mode is on.
    it('ignores startDrawing while drawing mode is off', () => {
        const store = useObstacleStore()
        store.startDrawing({ x: 10, y: 10 })
        expect(store.drawing.active).toBe(false)
        expect(store.drawRect).toBeNull()
    })

    it('tracks a drag and exposes its rectangle', () => {
        const store = useObstacleStore()
        store.setDrawingMode(true)
        store.startDrawing({ x: 50, y: 50 })
        store.updateDrawing({ x: 150, y: 120 })

        expect(store.drawing.active).toBe(true)
        expect(store.drawRect).toEqual({ x1: 50, y1: 50, x2: 150, y2: 120 })
    })

    it('normalizes a drag made up and to the left', () => {
        const store = useObstacleStore()
        store.setDrawingMode(true)
        store.startDrawing({ x: 150, y: 120 })
        store.updateDrawing({ x: 50, y: 50 })

        expect(store.drawRect).toEqual({ x1: 50, y1: 50, x2: 150, y2: 120 })
    })

    it('turning drawing mode off cancels a drag in progress', () => {
        const store = useObstacleStore()
        store.setDrawingMode(true)
        store.startDrawing({ x: 10, y: 10 })

        store.setDrawingMode(false)

        expect(store.drawing.active).toBe(false)
        expect(store.drawRect).toBeNull()
    })

    it('finishDrawing returns the start point and resets', () => {
        const store = useObstacleStore()
        store.setDrawingMode(true)
        store.startDrawing({ x: 5, y: 6 })

        expect(store.finishDrawing()).toEqual({ x: 5, y: 6 })
        expect(store.drawing.active).toBe(false)
    })
})
