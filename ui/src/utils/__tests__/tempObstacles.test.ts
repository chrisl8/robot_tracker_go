import { describe, it, expect } from 'vitest'
import type { TempObstacle } from '@/types/api'
import {
    hitTestTempObstacle,
    tempObstacleCanvasRect,
    tempObstacleLabel,
    tempObstacleStatusChip,
} from '../tempObstacles'

function obs(id: string, x1: number, y1: number, x2: number, y2: number): TempObstacle {
    return {
        id,
        pixel_top_left: [x1, y1],
        pixel_bottom_right: [x2, y2],
        world_top_left: [0, 0],
        world_bottom_right: [0, 0],
    }
}

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
