import type { Obstacle } from '@/types/api'

// Circle-vs-rectangle overlap test for destination placement.
// cx, cy, radius are all in NATURAL VIDEO coordinates (not canvas pixels).
export function isCircleInObstacle(
    obstacles: readonly Pick<Obstacle, 'pixel_top_left' | 'pixel_bottom_right'>[],
    cx: number,
    cy: number,
    radius: number
): boolean {
    for (const obs of obstacles) {
        const rectX1 = obs.pixel_top_left[0]
        const rectY1 = obs.pixel_top_left[1]
        const rectX2 = obs.pixel_bottom_right[0]
        const rectY2 = obs.pixel_bottom_right[1]

        // Find the closest point on the rectangle to the circle center
        const closestX = Math.max(rectX1, Math.min(cx, rectX2))
        const closestY = Math.max(rectY1, Math.min(cy, rectY2))

        // Calculate the distance from the closest point to the circle center
        const distanceX = cx - closestX
        const distanceY = cy - closestY
        const distanceSquared = distanceX * distanceX + distanceY * distanceY

        // If the distance is less than the circle radius, they intersect
        if (distanceSquared < radius * radius) {
            return true
        }

        // Also check if circle is completely inside rectangle
        if (
            cx >= rectX1 &&
            cx <= rectX2 &&
            cy >= rectY1 &&
            cy <= rectY2 &&
            cx - radius >= rectX1 &&
            cx + radius <= rectX2 &&
            cy - radius >= rectY1 &&
            cy + radius <= rectY2
        ) {
            return true
        }
    }
    return false
}
