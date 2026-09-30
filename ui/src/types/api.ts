// WebSocket API types - maps directly to Go backend types

// Track types (maps to internal/tracking/types.go)
export interface Track {
    id: number
    bbox: [number, number, number, number]
    confidence: number
    tag_id?: number
    state: 'pending' | 'confirmed' | 'lost'
    history: [number, number][]
    configured?: boolean
    name?: string
    pixel_radius?: number
    heading?: number
    heading_offset?: number
    corners?: [number, number][]
    motion_state?: 'forward' | 'backward' | 'rotating_left' | 'rotating_right' | 'stopped'
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
    // Whether the robot itself answers the gamepad's poll. 'silent' with the
    // Arduino Connected means the gamepad is fine but the robot is off/out of range.
    // Absent from an older backend.
    robotLink?: 'alive' | 'silent' | 'unknown'
    // From the robot's heartbeat, present only while it answers: whether its servos
    // are powered down (idle) and its own mode letter.
    robotServos?: 'asleep' | 'awake'
    robotMode?: string
    // How often the robot restarted (its uptime went backwards) since the service
    // started: usually a brown-out from a weak battery.
    robotReboots?: number
    hostMemoryMB?: number
    uptimeSec?: number
    // True when no camera frame has been processed for > 2 s (fps is then 0).
    cameraStalled?: boolean
    // Seconds since the last frame; present when cameraStalled.
    frameAgeSec?: number
}

// Calibration types
export interface CalibrationState {
    state: 'not_calibrated' | 'calibrating' | 'complete' | 'calibrated'
    message?: string
    resolutionMismatch?: boolean
    calibratedResolution?: [number, number]
}

export interface DetectedTagInfo {
    id: number
    center: [number, number]
    corners: [number, number][]
}

export type CalibrationTagRole = 'center' | 'corner'

export interface CalibrationTargetTag {
    id: number
    label: string
    role: CalibrationTagRole
    /** Where the tag's guide box sits, as a fraction (0-1) of the video frame. */
    guideX: number
    guideY: number
}

export interface CalibrationTarget {
    tagSize: number
    tags: CalibrationTargetTag[]
}

export interface CalibrationTagsResponse {
    tags: DetectedTagInfo[]
    frameWidth: number
    frameHeight: number
    target: CalibrationTarget
}

export type CalibrationRating = 'good' | 'ok' | 'poor'

export interface CalibrationTagResidual {
    id: number
    label: string
    rmsCm: number
    maxCm: number
}

export interface CalibrationCheck {
    fromId: number
    toId: number
    fromLabel: string
    toLabel: string
    meters: number
}

export interface CalibrationComputeRequest {
    tags: { id: number; corners: [number, number][] }[]
}

export interface CalibrationComputeResponse {
    state: string
    rmsCm?: number
    maxCm?: number
    qualityCm?: number
    rating?: CalibrationRating
    worstTagId?: number
    perTag?: CalibrationTagResidual[]
    checks?: CalibrationCheck[]
    message?: string
    error?: string
    filename?: string
}

// WebSocket message types (discriminated union)
export type WebSocketMessage =
    | TrackMessage
    | TracksMessage
    | ObstaclesMessage
    | StatusMessage
    | BBoxMessage
    | CalibrationMessage
    | DestinationMessage
    | DestinationClearMessage
    | PathsMessage
    | TempObstaclesMessage
    | ControlStateMessage

export interface ControlStateMessage {
    type: 'control_state'
    control: { mode: 'hold' | 'manual' | 'autonomous'; emergency_stopped: boolean }
}

export interface TrackMessage {
    type: 'track'
    track: Track
}

export interface TracksMessage {
    type: 'tracks'
    tracks: TracksNestedResponse
}

export interface TracksNestedResponse {
    tracks: Track[]
    count: number
}

export interface PathMessage {
    robot_id: number
    points: [number, number][]
    color: string
}

export interface PathsNestedResponse {
    paths: PathMessage[]
}

export interface PathsMessage {
    type: 'paths'
    paths: PathsNestedResponse
}

// Temporary obstacles found by the foreground (background-subtraction) detector
export interface TempObstacle {
    id: string
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    world_top_left: [number, number]
    world_bottom_right: [number, number]
    // The obstacle's exact (possibly rotated) footprint, four corners in
    // order; absent when the detector could not fit an oriented shape, in
    // which case the top-left/bottom-right box above is what's known.
    pixel_quad?: [number, number][]
    world_quad?: [number, number][]
}

export interface TempObstaclesPayload {
    obstacles: TempObstacle[]
    // The planner is steering around them
    applied: boolean
    // The background is still being learned
    warming: boolean
    // A lighting event: obstacles are being held
    guarded: boolean
    // The detector is on
    enabled: boolean
}

export interface TempObstaclesMessage {
    type: 'temp_obstacles'
    temp_obstacles: TempObstaclesPayload
}

export interface ForegroundState {
    enabled: boolean
    applied: boolean
    warming: boolean
    guarded: boolean
    count: number
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

// API Request types
export interface AddObstacleRequest {
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    name: string
    clearance: number
}

// Command types
export type RobotCommand = 'F' | 'B' | 'L' | 'R' | 'S'

export interface CommandRequest {
    command: RobotCommand
}

// Destination types
export interface Destination {
    id: string
    robot_id: number
    x: number
    y: number
}

export interface DestinationMessage {
    type: 'destination'
    destination: Destination & { valid: boolean }
}

export interface DestinationClearMessage {
    type: 'destination_clear'
    destination: { robot_id: number; valid: false }
}

// API Request types
export interface DestinationRequest {
    robot_id: number
    x: number
    y: number
}
