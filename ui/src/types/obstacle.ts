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
