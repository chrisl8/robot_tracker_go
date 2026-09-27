import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import {
    CAMERA_STALLED_WARN_AFTER_MS,
    FPS_CRITICAL_WARN_AFTER_MS,
    STATUS_SILENCE_MS,
    formatFps,
    isUsableFps,
    isWarmingUp,
    nextFpsLevel,
    type FpsLevel,
} from '@/composables/fpsHealth'
import { useRobotStore } from './robotStore'
import { useUIStore } from './uiStore'

export interface FpsReading {
    fps: number | null | undefined
    uptimeSec: number | null | undefined
    cameraStalled?: boolean
    frameAgeSec?: number
}

export type CriticalKind = 'slow' | 'stalled'

export const useFpsHealthStore = defineStore('fpsHealth', () => {
    const robotStore = useRobotStore()
    const uiStore = useUIStore()

    const level = ref<FpsLevel>('unknown')
    // Last frame rate accepted while warnings were active (null when suppressed/unknown).
    const fps = ref<number | null>(null)
    const connected = ref(false)
    // No video: the backend says the camera stalled, or status messages stopped arriving.
    const stalled = ref(false)
    // Whole seconds without a video frame, for the banner.
    const stalledSec = ref(0)
    // Set once the current critical episode has lasted long enough to warn.
    const warned = ref(false)
    // The user closed the banner; it stays closed until the episode ends.
    const dismissed = ref(false)

    // True when the websocket is up but the status feed has gone quiet.
    const silent = ref(false)
    let lastStatusAt = Date.now()
    let stalledSince = 0
    let sawStalled = false
    let warnTimer: ReturnType<typeof setTimeout> | null = null
    let silenceTimer: ReturnType<typeof setTimeout> | null = null
    let ticker: ReturnType<typeof setInterval> | null = null

    const kind = computed<CriticalKind>(() => (stalled.value ? 'stalled' : 'slow'))

    const bannerVisible = computed(
        () => level.value === 'critical' && warned.value && !dismissed.value
    )

    const levelClass = computed(() => {
        if (level.value === 'low') return 'fps-health-low'
        if (level.value === 'critical') return 'fps-health-critical'
        return ''
    })

    function clearWarnTimer(): void {
        if (warnTimer !== null) {
            clearTimeout(warnTimer)
            warnTimer = null
        }
    }

    function clearSilenceTimer(): void {
        if (silenceTimer !== null) {
            clearTimeout(silenceTimer)
            silenceTimer = null
        }
    }

    function stopTicker(): void {
        if (ticker !== null) {
            clearInterval(ticker)
            ticker = null
        }
    }

    function resetEpisode(): void {
        clearWarnTimer()
        warned.value = false
        dismissed.value = false
        sawStalled = false
    }

    function warnMessage(): string {
        if (stalled.value) {
            return `No video frames for ${stalledSec.value} s. The camera stream has stalled.`
        }
        return `Frame rate is ${formatFps(fps.value)} fps. Autonomous driving is unreliable below about 5 fps.`
    }

    function armWarnTimer(): void {
        clearWarnTimer()
        const delay = stalled.value ? CAMERA_STALLED_WARN_AFTER_MS : FPS_CRITICAL_WARN_AFTER_MS
        warnTimer = setTimeout(() => {
            warnTimer = null
            if (level.value !== 'critical') return
            warned.value = true
            uiStore.showToast(warnMessage(), 'warning', 6000)
        }, delay)
    }

    // While silent there are no status messages to refresh the count, so tick it.
    function startTicker(): void {
        if (ticker !== null) return
        ticker = setInterval(() => {
            stalledSec.value = Math.max(0, Math.round((Date.now() - lastStatusAt) / 1000))
        }, 1000)
    }

    function computeStalledSec(reading: FpsReading): number {
        if (typeof reading.frameAgeSec === 'number' && Number.isFinite(reading.frameAgeSec)) {
            return Math.max(0, Math.round(reading.frameAgeSec))
        }
        const since = silent.value ? lastStatusAt : stalledSince
        return Math.max(0, Math.round((Date.now() - since) / 1000))
    }

    function evaluate(reading: FpsReading): void {
        const suppressed = !connected.value || isWarmingUp(reading.uptimeSec)
        const wasStalled = stalled.value
        const wasCritical = level.value === 'critical'
        const isStalled = !suppressed && (reading.cameraStalled === true || silent.value)

        if (isStalled && !wasStalled) stalledSince = Date.now()
        stalled.value = isStalled
        if (isStalled) {
            sawStalled = true
            stalledSec.value = computeStalledSec(reading)
            if (silent.value && reading.cameraStalled !== true) startTicker()
        } else {
            stopTicker()
        }

        const next: FpsLevel = suppressed
            ? 'unknown'
            : isStalled
              ? 'critical'
              : nextFpsLevel(level.value, reading.fps)

        level.value = next
        fps.value = !suppressed && !isStalled && isUsableFps(reading.fps) ? reading.fps : null

        if (next === 'critical') {
            // A new episode, or the same one changing kind before it has warned.
            if (!wasCritical || (wasStalled !== isStalled && !warned.value)) armWarnTimer()
            return
        }

        if (wasCritical) {
            // Only announce a genuine recovery, not a reading that went away
            // (disconnect, restart) mid-episode.
            const announce = warned.value && (next === 'ok' || next === 'low')
            const fromStall = sawStalled
            resetEpisode()
            if (announce) {
                uiStore.showToast(
                    fromStall
                        ? 'Camera video recovered.'
                        : `Frame rate recovered (${formatFps(reading.fps)} fps).`,
                    'info'
                )
            }
        }
    }

    function currentReading(): FpsReading {
        const status = robotStore.status
        return {
            fps: status.fps,
            uptimeSec: status.uptimeSec,
            cameraStalled: status.cameraStalled,
            frameAgeSec: status.frameAgeSec,
        }
    }

    function armSilenceTimer(): void {
        clearSilenceTimer()
        if (!connected.value) return
        silenceTimer = setTimeout(() => {
            silenceTimer = null
            silent.value = true
            evaluate(currentReading())
        }, STATUS_SILENCE_MS)
    }

    // A status message arrived: the feed is alive.
    function recordStatus(): void {
        lastStatusAt = Date.now()
        silent.value = false
        armSilenceTimer()
        evaluate(currentReading())
    }

    function setConnection(isConnected: boolean): void {
        connected.value = isConnected
        silent.value = false
        lastStatusAt = Date.now()
        if (isConnected) {
            armSilenceTimer()
        } else {
            clearSilenceTimer()
        }
        evaluate(currentReading())
    }

    function dismiss(): void {
        dismissed.value = true
    }

    watch(
        () => robotStore.status,
        () => recordStatus()
    )

    return {
        level,
        fps,
        connected,
        stalled,
        stalledSec,
        kind,
        bannerVisible,
        levelClass,
        evaluate,
        recordStatus,
        setConnection,
        dismiss,
    }
})
