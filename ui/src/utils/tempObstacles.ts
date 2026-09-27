import type { TempObstacle } from '@/types/api'

export interface CanvasRect {
    x: number
    y: number
    w: number
    h: number
}

export interface TempObstacleFlags {
    enabled: boolean
    warming: boolean
    guarded: boolean
}

type ToCanvas = (x: number, y: number) => { x: number; y: number }

// The obstacle's box in canvas pixels, from its natural-frame pixel corners.
export function tempObstacleCanvasRect(obs: TempObstacle, toCanvas: ToCanvas): CanvasRect {
    const a = toCanvas(obs.pixel_top_left[0], obs.pixel_top_left[1])
    const b = toCanvas(obs.pixel_bottom_right[0], obs.pixel_bottom_right[1])
    return {
        x: Math.min(a.x, b.x),
        y: Math.min(a.y, b.y),
        w: Math.abs(b.x - a.x),
        h: Math.abs(b.y - a.y),
    }
}

function area(obs: TempObstacle): number {
    return (
        Math.abs(obs.pixel_bottom_right[0] - obs.pixel_top_left[0]) *
        Math.abs(obs.pixel_bottom_right[1] - obs.pixel_top_left[1])
    )
}

// Which temporary obstacle (if any) contains this natural-frame pixel; the
// smallest wins so a small object inside a larger box can still be picked.
export function hitTestTempObstacle(
    obstacles: readonly TempObstacle[],
    x: number,
    y: number
): TempObstacle | null {
    let best: TempObstacle | null = null
    for (const obs of obstacles) {
        const inside =
            x >= obs.pixel_top_left[0] &&
            x <= obs.pixel_bottom_right[0] &&
            y >= obs.pixel_top_left[1] &&
            y <= obs.pixel_bottom_right[1]
        if (inside && (best === null || area(obs) < area(best))) {
            best = obs
        }
    }
    return best
}

// Text for the corner chip, or null when there is nothing to say.
export function tempObstacleStatusChip(flags: TempObstacleFlags): string | null {
    if (!flags.enabled) return null
    if (flags.warming) return 'Learning background: keep the floor clear'
    if (flags.guarded) return 'Lighting change: holding obstacles'
    return null
}

export function tempObstacleLabel(applied: boolean): string {
    return applied ? 'temp' : 'temp (not steering)'
}
