// Obstacle-related types

export type { Obstacle, ObstaclesResponse, AddObstacleRequest } from './api'

export interface DrawState {
    active: boolean
    startPoint: Point | null
    currentPoint: Point | null
    startTime: number | null
}

export interface Point {
    x: number
    y: number
}

export interface DrawRect {
    x1: number
    y1: number
    x2: number
    y2: number
}

export const MIN_OBSTACLE_SIZE = 10

export function isValidObstacle(rect: DrawRect): boolean {
    const width = rect.x2 - rect.x1
    const height = rect.y2 - rect.y1
    return width > MIN_OBSTACLE_SIZE && height > MIN_OBSTACLE_SIZE
}

export function calculateObstacleFromPoints(
    start: Point,
    end: Point
): { topLeft: [number, number]; bottomRight: [number, number] } {
    return {
        topLeft: [
            Math.round(Math.min(start.x, end.x)),
            Math.round(Math.min(start.y, end.y))
        ],
        bottomRight: [
            Math.round(Math.max(start.x, end.x)),
            Math.round(Math.max(start.y, end.y))
        ]
    }
}
