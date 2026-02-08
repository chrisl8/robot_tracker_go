// WebSocket API types - maps directly to Go backend types

// Track types (maps to internal/tracking/types.go)
export interface Track {
    id: number
    bbox: [number, number, number, number]  // [x1, y1, x2, y2]
    confidence: number
    tag_id?: number
    state: 'pending' | 'confirmed' | 'lost'
    history: [number, number][]  // [x, y] coordinates
}

// Obstacle types (maps to internal/planning/static_obstacle.go)
export interface Obstacle {
    id: string
    name: string
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    world_top_left: [number, number]
    world_bottom_right: [number, number]
    clearance: number
}

export interface ObstaclesResponse {
    obstacles: Obstacle[]
    count: number
    saved: boolean
}

// Robot Status types
export interface RobotStatus {
    connected: boolean
    fps: number
    robotCount: number
    arduinoState: 'Connected' | 'Disconnected' | 'Searching' | 'Error'
}

// Calibration types
export interface CalibrationState {
    state: 'not_calibrated' | 'calibrating' | 'complete' | 'calibrated'
    message?: string
}

export interface DetectedTagInfo {
    id: number
    center: [number, number]
    corners: [number, number][]
}

export interface CalibrationTagsResponse {
    tags: DetectedTagInfo[]
    count: number
}

export interface CalibrationComputeResponse {
    state: string
    tag_id?: number
    computed_width?: number
    computed_height?: number
    pixels_per_meter?: number
    message?: string
    error?: string
    filename?: string
    homography?: number[][]
}

// WebSocket message types (discriminated union)
export type WebSocketMessage =
    | TrackMessage
    | TracksMessage
    | ObstaclesMessage
    | StatusMessage
    | BBoxMessage
    | CalibrationMessage
    | CalibrationTagsMessage

export interface TrackMessage {
    type: 'track'
    track: Track
}

export interface TracksMessage {
    type: 'tracks'
    tracks: Track[]
}

export interface ObstaclesMessage {
    type: 'obstacles'
    obstacles: ObstaclesResponse
}

export interface StatusMessage {
    type: 'status'
    status: RobotStatus
}

export interface BBoxMessage {
    type: 'bbox'
    bbox: {
        id: number
        x: number
        y: number
        w: number
        h: number
        confidence: number
        class_name: string
    }
}

export interface CalibrationMessage {
    type: 'calibration'
    calibration: CalibrationState
}

export interface CalibrationTagsMessage {
    type: 'calibration_tags'
    tags: DetectedTagInfo[]
    count: number
}

// API Request types
export interface AddObstacleRequest {
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    name: string
    clearance: number
}

export interface CalibrationComputeRequest {
    tag_id: number
    tag_size: number
    corners: [number, number][]
}

// Command types
export type RobotCommand = 'F' | 'B' | 'L' | 'R' | 'S'

export interface CommandRequest {
    command: RobotCommand
}
