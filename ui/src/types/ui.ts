// UI state types

export type ToastType = 'success' | 'error' | 'warning' | 'info'

export interface Toast {
    id: string
    message: string
    type: ToastType
    duration?: number
}

export interface PanelState {
    obstacleOpen: boolean
    calibrationOpen: boolean
    leftPanelOpen: boolean
    rightPanelOpen: boolean
}

export interface KeyboardState {
    w: boolean
    a: boolean
    s: boolean
    d: boolean
    x: boolean
    e: boolean
    z: boolean
    c: boolean
}

export type LogEntryType = 'info' | 'success' | 'warning' | 'error' | 'system'

export interface LogEntry {
    id: string
    timestamp: Date
    type: LogEntryType
    message: string
}

export type CalibrationStep = 'print' | 'place' | 'result'
