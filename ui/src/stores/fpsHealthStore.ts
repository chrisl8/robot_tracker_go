import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'
import {
    FPS_CRITICAL_WARN_AFTER_MS,
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
}

export const useFpsHealthStore = defineStore('fpsHealth', () => {
    const robotStore = useRobotStore()
    const uiStore = useUIStore()

    const level = ref<FpsLevel>('unknown')
    // Last frame rate accepted while warnings were active (null when suppressed/unknown).
    const fps = ref<number | null>(null)
    const connected = ref(false)
    // Set once the current critical episode has lasted long enough to warn.
    const warned = ref(false)
    // The user closed the banner; it stays closed until the episode ends.
    const dismissed = ref(false)

    let warnTimer: ReturnType<typeof setTimeout> | null = null

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

    function resetEpisode(): void {
        clearWarnTimer()
        warned.value = false
        dismissed.value = false
    }

    function startEpisode(): void {
        resetEpisode()
        warnTimer = setTimeout(() => {
            warnTimer = null
            if (level.value !== 'critical') return
            warned.value = true
            uiStore.showToast(
                `Frame rate is ${formatFps(fps.value)} fps. Autonomous driving is unreliable below about 5 fps.`,
                'warning',
                6000
            )
        }, FPS_CRITICAL_WARN_AFTER_MS)
    }

    function evaluate(reading: FpsReading): void {
        const suppressed = !connected.value || isWarmingUp(reading.uptimeSec)
        const previous = level.value
        const next: FpsLevel = suppressed ? 'unknown' : nextFpsLevel(previous, reading.fps)

        level.value = next
        fps.value = !suppressed && isUsableFps(reading.fps) ? reading.fps : null

        if (next === 'critical') {
            if (previous !== 'critical') startEpisode()
            return
        }

        if (previous === 'critical') {
            // Only announce a genuine recovery, not a reading that went away
            // (disconnect, restart) mid-episode.
            const announce = warned.value && (next === 'ok' || next === 'low')
            resetEpisode()
            if (announce) {
                uiStore.showToast(`Frame rate recovered (${formatFps(reading.fps)} fps).`, 'info')
            }
        }
    }

    function setConnection(isConnected: boolean): void {
        connected.value = isConnected
        evaluate({ fps: robotStore.status.fps, uptimeSec: robotStore.status.uptimeSec })
    }

    function dismiss(): void {
        dismissed.value = true
    }

    watch(
        () => [robotStore.status.fps, robotStore.status.uptimeSec] as const,
        ([currentFps, uptimeSec]) => evaluate({ fps: currentFps, uptimeSec })
    )

    return { level, fps, connected, bannerVisible, levelClass, evaluate, setConnection, dismiss }
})
