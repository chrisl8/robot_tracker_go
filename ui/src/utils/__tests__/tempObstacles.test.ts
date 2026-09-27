import { describe, it, expect } from 'vitest'
import type { TempObstacle } from '@/types/api'
import {
    hitTestTempObstacle,
    tempObstacleCanvasPolygon,
    tempObstacleCanvasRect,
    tempObstacleLabel,
    tempObstacleStatusChip,
    topmostPoint,
} from '../tempObstacles'

function obs(
    id: string,
    x1: number,
    y1: number,
    x2: number,
    y2: number,
    quad?: [number, number][]
): TempObstacle {
    return {
        id,
        pixel_top_left: [x1, y1],
        pixel_bottom_right: [x2, y2],
        world_top_left: [0, 0],
        world_bottom_right: [0, 0],
        pixel_quad: quad,
    }
}

// A 100x20 stick (half-length 50, half-width 10) centred at (200, 200),
// rotated 45 degrees. Its AABB spans roughly (157.6, 157.6) to (242.4,
// 242.4), but the real quad leaves the AABB's own corners well clear of the
// stick itself.
const rotatedStick: [number, number][] = [
    [228.28, 242.43],
    [242.43, 228.28],
    [171.72, 157.57],
    [157.57, 171.72],
]

describe('tempObstacleCanvasRect', () => {
    it('scales natural pixels to canvas pixels', () => {
        const rect = tempObstacleCanvasRect(obs('a', 100, 50, 200, 150), (x, y) => ({
            x: x * 2 + 10,
            y: y * 2 + 20,
        }))
        expect(rect).toEqual({ x: 210, y: 120, w: 200, h: 200 })
    })

    it('normalises corners that arrive flipped', () => {
        const rect = tempObstacleCanvasRect(obs('a', 200, 150, 100, 50), (x, y) => ({ x, y }))
        expect(rect).toEqual({ x: 100, y: 50, w: 100, h: 100 })
    })
})

describe('tempObstacleCanvasPolygon', () => {
    it('projects each quad corner through toCanvas, in order', () => {
        const withQuad = obs('a', 150, 150, 250, 250, rotatedStick)
        const polygon = tempObstacleCanvasPolygon(withQuad, (x, y) => ({
            x: x * 2 + 1,
            y: y * 2 + 2,
        }))
        expect(polygon).toEqual(rotatedStick.map(([x, y]) => ({ x: x * 2 + 1, y: y * 2 + 2 })))
    })

    it('returns null when the obstacle has no quad', () => {
        const noQuad = obs('a', 150, 150, 250, 250)
        expect(tempObstacleCanvasPolygon(noQuad, (x, y) => ({ x, y }))).toBeNull()
    })
})

describe('topmostPoint', () => {
    it('picks the point with the smallest y', () => {
        expect(
            topmostPoint([
                { x: 0, y: 10 },
                { x: 5, y: 2 },
                { x: 9, y: 20 },
            ])
        ).toEqual({ x: 5, y: 2 })
    })
})

describe('hitTestTempObstacle', () => {
    const big = obs('big', 0, 0, 400, 400)
    const small = obs('small', 100, 100, 150, 150)
    const far = obs('far', 500, 500, 600, 600)

    it('finds the obstacle under a point', () => {
        expect(hitTestTempObstacle([big, far], 550, 550)?.id).toBe('far')
    })

    it('includes the edges', () => {
        expect(hitTestTempObstacle([small], 100, 150)?.id).toBe('small')
    })

    it('returns null when the point is outside every box', () => {
        expect(hitTestTempObstacle([small, far], 300, 300)).toBeNull()
    })

    it('prefers the smallest box when they overlap', () => {
        expect(hitTestTempObstacle([big, small], 120, 120)?.id).toBe('small')
        expect(hitTestTempObstacle([small, big], 120, 120)?.id).toBe('small')
    })

    it('handles an empty list', () => {
        expect(hitTestTempObstacle([], 1, 1)).toBeNull()
    })

    it('tests against the real quad, not just its bounding box, when one is present', () => {
        const stick = obs('stick', 150, 150, 250, 250, rotatedStick)
        // Centre of the stick: inside the quad.
        expect(hitTestTempObstacle([stick], 200, 200)?.id).toBe('stick')
        // A corner of the stick's own AABB: inside the box, but outside the
        // real rotated shape.
        expect(hitTestTempObstacle([stick], 157.6, 157.6)).toBeNull()
    })
})

describe('tempObstacleStatusChip', () => {
    it.each([
        [
            { enabled: true, warming: true, guarded: false },
            'Learning background: keep the floor clear',
        ],
        [{ enabled: true, warming: false, guarded: true }, 'Lighting change: holding obstacles'],
        [
            { enabled: true, warming: true, guarded: true },
            'Learning background: keep the floor clear',
        ],
        [{ enabled: true, warming: false, guarded: false }, null],
        [{ enabled: false, warming: true, guarded: true }, null],
    ])('%j -> %s', (flags, expected) => {
        expect(tempObstacleStatusChip(flags)).toBe(expected)
    })
})

describe('tempObstacleLabel', () => {
    it('marks obstacles the planner is ignoring', () => {
        expect(tempObstacleLabel(true)).toBe('temp')
        expect(tempObstacleLabel(false)).toBe('temp (not steering)')
    })
})
