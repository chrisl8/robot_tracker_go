import type { TempObstacle } from '@/types/api'

export interface CanvasRect {
    x: number
    y: number
    w: number
    h: number
}

export interface CanvasPoint {
    x: number
    y: number
}

export interface TempObstacleFlags {
    enabled: boolean
    warming: boolean
    guarded: boolean
}

type ToCanvas = (x: number, y: number) => { x: number; y: number }

// The obstacle's box in canvas pixels, from its natural-frame pixel corners.
// Used as the fallback shape when the obstacle has no quad.
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

// The obstacle's exact footprint in canvas pixels, four corners in order, or
// null when it has no quad (draw the rectangle fallback instead).
export function tempObstacleCanvasPolygon(
    obs: TempObstacle,
    toCanvas: ToCanvas
): CanvasPoint[] | null {
    if (!obs.pixel_quad || obs.pixel_quad.length !== 4) return null
    return obs.pixel_quad.map(([x, y]) => toCanvas(x, y))
}

// The topmost point of a polygon (smallest y), for label placement near the
// shape's top edge, matching where the rectangle label sits.
export function topmostPoint(points: readonly CanvasPoint[]): CanvasPoint {
    return points.reduce((top, p) => (p.y < top.y ? p : top), points[0])
}

function area(obs: TempObstacle): number {
    return (
        Math.abs(obs.pixel_bottom_right[0] - obs.pixel_top_left[0]) *
        Math.abs(obs.pixel_bottom_right[1] - obs.pixel_top_left[1])
    )
}

// Even-odd point-in-polygon test in natural-frame pixel coordinates.
function pointInQuad(quad: readonly [number, number][], x: number, y: number): boolean {
    let inside = false
    for (let i = 0, j = quad.length - 1; i < quad.length; j = i++) {
        const [xi, yi] = quad[i]
        const [xj, yj] = quad[j]
        const intersects = yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi) + xi
        if (intersects) inside = !inside
    }
    return inside
}

// Which temporary obstacle (if any) contains this natural-frame pixel; the
// smallest (by its bounding box's area) wins so a small object inside a
// larger box can still be picked. Tests against the obstacle's real quad
// when it has one, falling back to its axis-aligned box otherwise.
export function hitTestTempObstacle(
    obstacles: readonly TempObstacle[],
    x: number,
    y: number
): TempObstacle | null {
    let best: TempObstacle | null = null
    for (const obs of obstacles) {
        const inside = obs.pixel_quad
            ? pointInQuad(obs.pixel_quad, x, y)
            : x >= obs.pixel_top_left[0] &&
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
