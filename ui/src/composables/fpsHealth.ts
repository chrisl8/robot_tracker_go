// Frame-rate health thresholds.
//
// The path-following controller was tuned around ~5 fps. At ~2.3 fps its
// turn/wait bursts are far too long and navigation overshoots badly, so below
// FPS_CRITICAL_BELOW autonomous driving cannot be trusted. FPS_LOW_BELOW is an
// early-warning band. Each level has a higher recovery threshold (hysteresis) so
// a reading hovering near a boundary does not flap between levels.

export type FpsLevel = 'unknown' | 'ok' | 'low' | 'critical'

export const FPS_LOW_BELOW = 8
export const FPS_LOW_RECOVER_AT = 9
export const FPS_CRITICAL_BELOW = 5
export const FPS_CRITICAL_RECOVER_AT = 6

// Ignore the frame rate while the process is still ramping up.
export const FPS_STARTUP_GRACE_SEC = 15

// Critical must persist this long (wall clock) before the warning banner shows.
export const FPS_CRITICAL_WARN_AFTER_MS = 4000

export function isUsableFps(fps: number | null | undefined): fps is number {
    return typeof fps === 'number' && Number.isFinite(fps) && fps > 0
}

/** Next level for a reading, given the previous level (applies hysteresis). */
export function nextFpsLevel(previous: FpsLevel, fps: number | null | undefined): FpsLevel {
    if (!isUsableFps(fps)) return 'unknown'

    if (previous === 'critical') {
        if (fps >= FPS_LOW_RECOVER_AT) return 'ok'
        if (fps >= FPS_CRITICAL_RECOVER_AT) return 'low'
        return 'critical'
    }

    if (fps < FPS_CRITICAL_BELOW) return 'critical'

    if (previous === 'low') {
        return fps >= FPS_LOW_RECOVER_AT ? 'ok' : 'low'
    }

    return fps < FPS_LOW_BELOW ? 'low' : 'ok'
}

/** True while the process is young enough that its frame rate is not yet meaningful. */
export function isWarmingUp(uptimeSec: number | null | undefined): boolean {
    return (
        typeof uptimeSec === 'number' &&
        Number.isFinite(uptimeSec) &&
        uptimeSec < FPS_STARTUP_GRACE_SEC
    )
}

export function formatFps(fps: number | null | undefined): string {
    return isUsableFps(fps) ? fps.toFixed(1) : '--'
}
