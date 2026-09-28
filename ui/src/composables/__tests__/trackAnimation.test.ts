import { describe, it, expect } from 'vitest'
import type { Track } from '@/types/api'
import { computePixelHeading, createTrackAnimation } from '../canvas/trackAnimation'

function makeTrack(overrides: Partial<Track> = {}): Track {
    return {
        id: 1,
        bbox: [100, 100, 200, 200],
        confidence: 0.9,
        state: 'confirmed',
        history: [],
        configured: true,
        // top edge (0,1) above bottom edge (2,3) in image space → heading points -y
        corners: [
            [100, 100],
            [200, 100],
            [200, 200],
            [100, 200],
        ],
        ...overrides,
    } as Track
}

describe('computePixelHeading', () => {
    it('returns null without four corners', () => {
        expect(computePixelHeading(makeTrack({ corners: undefined }))).toBeNull()
    })

    it('adds heading_offset', () => {
        const base = computePixelHeading(makeTrack())!
        const shifted = computePixelHeading(makeTrack({ heading_offset: 0.5 } as Partial<Track>))!
        expect(shifted - base).toBeCloseTo(0.5)
    })
})

describe('createTrackAnimation', () => {
    it('spawns particles while moving forward and clears them when stopped', () => {
        const anim = createTrackAnimation()
        const moving = makeTrack({ motion_state: 'forward' } as Partial<Track>)
        anim.update(16, [moving])
        anim.update(16, [moving])
        expect(anim.animationState.get(1)!.particles).toHaveLength(2)

        anim.update(16, [makeTrack({ motion_state: 'stopped' } as Partial<Track>)])
        expect(anim.animationState.get(1)!.particles).toHaveLength(0)
    })

    it('expires particles after maxAge and caps the count at 20', () => {
        const anim = createTrackAnimation()
        const moving = makeTrack({ motion_state: 'forward' } as Partial<Track>)
        for (let i = 0; i < 30; i++) anim.update(1, [moving])
        expect(anim.animationState.get(1)!.particles.length).toBeLessThanOrEqual(20)

        // A long step ages every existing particle past 500ms; only the newly spawned one remains
        anim.update(600, [moving])
        expect(anim.animationState.get(1)!.particles).toHaveLength(1)
    })

    it('drops state for removed and unconfigured tracks', () => {
        const anim = createTrackAnimation()
        anim.update(16, [makeTrack()])
        expect(anim.animationState.has(1)).toBe(true)
        anim.update(16, [])
        expect(anim.animationState.has(1)).toBe(false)

        anim.update(16, [makeTrack({ configured: false })])
        expect(anim.animationState.size).toBe(0)
    })

    it('advances the path phase in seconds', () => {
        const anim = createTrackAnimation()
        anim.update(500, [])
        anim.update(500, [])
        expect(anim.getPathPhase()).toBeCloseTo(1)
    })
})
