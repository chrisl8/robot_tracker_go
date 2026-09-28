import type { Track } from '@/types/api'

// Per-track particle/phase animation state for the movement indicators (thrust
// particles, pulsing chevron, rotation arc). This owns the *state and its update
// step* only; drawing lives in renderTracks.ts. One instance per useCanvas().

export interface Particle {
    x: number
    y: number
    vx: number
    vy: number
    age: number
    maxAge: number
}

export interface TrackAnimState {
    particles: Particle[]
    phase: number
}

// Pixel-space heading of a track from its tag corners (top edge midpoint minus bottom
// edge midpoint), plus the robot's configured heading offset. Null without 4 corners.
export function computePixelHeading(track: Track): number | null {
    if (!track.corners || track.corners.length !== 4) return null
    const botMidX = (track.corners[2][0] + track.corners[3][0]) / 2
    const botMidY = (track.corners[2][1] + track.corners[3][1]) / 2
    const topMidX = (track.corners[0][0] + track.corners[1][0]) / 2
    const topMidY = (track.corners[0][1] + track.corners[1][1]) / 2
    return Math.atan2(topMidY - botMidY, topMidX - botMidX) + (track.heading_offset || 0)
}

export function createTrackAnimation() {
    // Animation state for movement indicators, keyed by track ID
    const animationState = new Map<number, TrackAnimState>()
    // Seconds accumulated for the path-pulse animation (independent of any track)
    let pathPhase = 0

    // Advance all animation state by dt milliseconds.
    function update(dt: number, tracks: readonly Track[]): void {
        pathPhase += dt / 1000
        updateAnimationState(dt, tracks)
    }

    function updateAnimationState(dt: number, tracks: readonly Track[]): void {
        const activeTrackIds = new Set<number>()

        for (const track of tracks) {
            if (!track.configured || !track.corners || track.corners.length !== 4) continue
            activeTrackIds.add(track.id)

            let state = animationState.get(track.id)
            if (!state) {
                state = { particles: [], phase: 0 }
                animationState.set(track.id, state)
            }

            state.phase += dt / 1000

            // Update existing particles
            for (let i = state.particles.length - 1; i >= 0; i--) {
                const p = state.particles[i]
                p.age += dt
                p.x += p.vx * (dt / 1000)
                p.y += p.vy * (dt / 1000)
                if (p.age >= p.maxAge) {
                    state.particles.splice(i, 1)
                }
            }

            // Spawn new particles for forward/backward motion
            const motionState = track.motion_state || 'stopped'
            if (motionState === 'forward' || motionState === 'backward') {
                const heading = computePixelHeading(track)
                if (heading === null) continue

                const centerX = (track.bbox[0] + track.bbox[2]) / 2
                const centerY = (track.bbox[1] + track.bbox[3]) / 2
                const dir = motionState === 'backward' ? heading + Math.PI : heading
                const speed = 80

                state.particles.push({
                    x: centerX,
                    y: centerY,
                    vx: Math.cos(dir) * speed,
                    vy: Math.sin(dir) * speed,
                    age: 0,
                    maxAge: 500,
                })

                if (state.particles.length > 20) {
                    state.particles.shift()
                }
            } else {
                // Clear particles when not moving linearly
                state.particles.length = 0
            }
        }

        // Clean up state for removed tracks
        for (const id of animationState.keys()) {
            if (!activeTrackIds.has(id)) {
                animationState.delete(id)
            }
        }
    }

    return {
        animationState,
        update,
        getPathPhase: () => pathPhase,
    }
}

export type TrackAnimation = ReturnType<typeof createTrackAnimation>
