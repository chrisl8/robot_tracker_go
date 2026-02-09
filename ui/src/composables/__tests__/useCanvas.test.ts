import { describe, it, expect } from 'vitest'

describe('FOOTPRINT-001: Circle-Rectangle Collision Detection', () => {
    function isCircleInObstacle(cx: number, cy: number, radius: number, rectX1: number, rectY1: number, rectX2: number, rectY2: number): boolean {
        const closestX = Math.max(rectX1, Math.min(cx, rectX2))
        const closestY = Math.max(rectY1, Math.min(cy, rectY2))
        const distanceX = cx - closestX
        const distanceY = cy - closestY
        const distanceSquared = distanceX * distanceX + distanceY * distanceY

        if (distanceSquared < radius * radius) {
            return true
        }

        if (cx >= rectX1 && cx <= rectX2 && cy >= rectY1 && cy <= rectY2 &&
            cx - radius >= rectX1 && cx + radius <= rectX2 &&
            cy - radius >= rectY1 && cy + radius <= rectY2) {
            return true
        }
        return false
    }

    it('BUG-001: detects circle overlaps obstacle when center is outside but circle edge overlaps', () => {
        // OLD BUG: isPointInObstacle only checked center point
        // NEW: isCircleInObstacle checks entire circle area

        // Obstacle at [100, 100] to [200, 200]
        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }

        // Case 1: Center at [90, 150] with radius 50
        // Old behavior: center (90,150) is outside obstacle → would say valid (green)
        // New behavior: circle edge overlaps obstacle → should say invalid (red)
        // Circle extends x=[40,140], obstacle x=[100,200] → overlap in [100,140]
        expect(isCircleInObstacle(90, 150, 50, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(true)

        // Case 2: Center far away - no overlap
        expect(isCircleInObstacle(50, 50, 20, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(false)
    })

    it('detects circle completely inside obstacle', () => {
        const obs = { x1: 0, y1: 0, x2: 400, y2: 400 }
        expect(isCircleInObstacle(200, 200, 50, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(true)
    })

    it('detects circle overlaps corner of obstacle', () => {
        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }
        // Circle center at [80, 80], radius 30
        // Circle extends to [50,50] to [110,110], overlaps corner
        expect(isCircleInObstacle(80, 80, 30, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(true)
    })

    it('detects partial overlap - circle crosses top edge', () => {
        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }
        // Circle center at [150, 90], radius 30
        // Circle extends y=[60,120], obstacle y=[100,200] → overlap
        expect(isCircleInObstacle(150, 90, 30, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(true)
    })

    it('detects no overlap - circle on other side of obstacle', () => {
        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }
        // Circle center at [150, 50], radius 20
        // Circle extends y=[30,70], obstacle starts at y=100
        expect(isCircleInObstacle(150, 50, 20, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(false)
    })

    it('handles touching edge (not an overlap)', () => {
        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }
        // Circle center at [250, 150], radius 50
        // Circle extends x=[200,300], obstacle ends at x=200
        // They touch at exactly x=200 - this is not an overlap
        expect(isCircleInObstacle(250, 150, 50, obs.x1, obs.y1, obs.x2, obs.y2)).toBe(false)
    })

    it('compares old vs new behavior for edge case', () => {
        // This test demonstrates the bug fix

        const obs = { x1: 100, y1: 100, x2: 200, y2: 200 }

        // Circle center at [90, 150] with radius 50
        const centerX = 90
        const centerY = 150
        const radius = 50

        // OLD behavior (isPointInObstacle): only checks center point
        // Center (90, 150) is outside the obstacle [100,100]-[200,200]
        const oldBehaviorCenterInObstacle =
            centerX >= obs.x1 && centerX <= obs.x2 &&
            centerY >= obs.y1 && centerY <= obs.y2
        expect(oldBehaviorCenterInObstacle).toBe(false) // Center is outside

        // NEW behavior (isCircleInObstacle): checks entire circle
        // The circle extends into the obstacle area
        const newBehaviorCircleInObstacle =
            isCircleInObstacle(centerX, centerY, radius, obs.x1, obs.y1, obs.x2, obs.y2)
        expect(newBehaviorCircleInObstacle).toBe(true) // Circle overlaps

        // This is the key difference:
        // OLD: would show GREEN (center is outside obstacle)
        // NEW: shows RED (any part of circle overlaps obstacle)
    })
})
