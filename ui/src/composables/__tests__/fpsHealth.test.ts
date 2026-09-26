import { describe, it, expect } from 'vitest'
import {
    FPS_CRITICAL_BELOW,
    FPS_CRITICAL_RECOVER_AT,
    FPS_LOW_BELOW,
    FPS_LOW_RECOVER_AT,
    FPS_STARTUP_GRACE_SEC,
    formatFps,
    isUsableFps,
    isWarmingUp,
    nextFpsLevel,
    type FpsLevel,
} from '../fpsHealth'

describe('thresholds', () => {
    it('leave each level at a higher reading than the one that enters it', () => {
        expect(FPS_LOW_RECOVER_AT).toBeGreaterThan(FPS_LOW_BELOW)
        expect(FPS_CRITICAL_RECOVER_AT).toBeGreaterThan(FPS_CRITICAL_BELOW)
        expect(FPS_CRITICAL_BELOW).toBeLessThan(FPS_LOW_BELOW)
    })
})

describe('isUsableFps', () => {
    it.each([
        [12, true],
        [0.1, true],
        [0, false],
        [-3, false],
        [NaN, false],
        [Infinity, false],
        [undefined, false],
        [null, false],
    ])('%s -> %s', (value, expected) => {
        expect(isUsableFps(value)).toBe(expected)
    })
})

describe('nextFpsLevel', () => {
    it.each<[FpsLevel, number | null | undefined, FpsLevel]>([
        ['unknown', 12, 'ok'],
        ['unknown', 7.9, 'low'],
        ['unknown', 4.9, 'critical'],
        ['ok', 8, 'ok'],
        ['ok', 7.99, 'low'],
        ['ok', 5, 'low'],
        ['ok', 4.99, 'critical'],
        ['low', 8.5, 'low'],
        ['low', 9, 'ok'],
        ['low', 4.99, 'critical'],
        ['critical', 5.5, 'critical'],
        ['critical', 6, 'low'],
        ['critical', 8.9, 'low'],
        ['critical', 9, 'ok'],
        ['critical', 30, 'ok'],
    ])('from %s at %s -> %s', (previous, fps, expected) => {
        expect(nextFpsLevel(previous, fps)).toBe(expected)
    })

    it.each<[FpsLevel]>([['unknown'], ['ok'], ['low'], ['critical']])(
        'missing, zero or non-finite readings are unknown from %s',
        previous => {
            for (const fps of [undefined, null, 0, NaN, -1]) {
                expect(nextFpsLevel(previous, fps)).toBe('unknown')
            }
        }
    )

    it('does not flap while hovering around the low boundary', () => {
        let level: FpsLevel = 'ok'
        const seen: FpsLevel[] = []
        for (const fps of [8.4, 7.8, 8.2, 7.9, 8.6, 8.1, 8.9]) {
            level = nextFpsLevel(level, fps)
            seen.push(level)
        }
        expect(seen).toEqual(['ok', 'low', 'low', 'low', 'low', 'low', 'low'])
    })

    it('does not flap while hovering around the critical boundary', () => {
        let level: FpsLevel = 'low'
        const seen: FpsLevel[] = []
        for (const fps of [5.5, 4.8, 5.2, 4.9, 5.9, 5.1]) {
            level = nextFpsLevel(level, fps)
            seen.push(level)
        }
        expect(seen).toEqual(['low', 'critical', 'critical', 'critical', 'critical', 'critical'])
    })
})

describe('isWarmingUp', () => {
    it.each([
        [0, true],
        [FPS_STARTUP_GRACE_SEC - 0.1, true],
        [FPS_STARTUP_GRACE_SEC, false],
        [600, false],
        [undefined, false],
        [null, false],
        [NaN, false],
    ])('uptime %s -> %s', (uptime, expected) => {
        expect(isWarmingUp(uptime)).toBe(expected)
    })
})

describe('formatFps', () => {
    it('shows one decimal for usable values and dashes otherwise', () => {
        expect(formatFps(12.04)).toBe('12.0')
        expect(formatFps(2.34)).toBe('2.3')
        expect(formatFps(0)).toBe('--')
        expect(formatFps(undefined)).toBe('--')
    })
})
