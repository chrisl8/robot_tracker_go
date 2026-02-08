// Robot-related types

export type { Track, RobotStatus } from './api'

export interface TrackColor {
    id: number
    color: string
}

export const TRACK_COLORS = [
    '#4ecca3', '#e94560', '#ffc107', '#00bcd4',
    '#9c27b0', '#ff9800', '#00acc1', '#673ab7',
    '#f44336', '#3f51b5', '#009688', '#795548',
]

export function getTrackColor(trackId: number): string {
    return TRACK_COLORS[trackId % TRACK_COLORS.length]
}

export interface Destination {
    id: string
    x: number
    y: number
    label?: string
}

export interface ControlState {
    forward: boolean
    backward: boolean
    left: boolean
    right: boolean
    stop: boolean
}
