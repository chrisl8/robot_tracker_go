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

export interface KeyboardShortcut {
    key: string
    ctrl?: boolean
    shift?: boolean
    alt?: boolean
    action: () => void
    description: string
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
