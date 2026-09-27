import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import {
    CAMERA_STALLED_WARN_AFTER_MS,
    FPS_CRITICAL_WARN_AFTER_MS,
    STATUS_SILENCE_MS,
} from '@/composables/fpsHealth'
import type { RobotStatus } from '@/types/api'
import { useFpsHealthStore } from '../fpsHealthStore'
import { useRobotStore } from '../robotStore'
import { useUIStore } from '../uiStore'

interface Feed {
    fps: number
    uptimeSec?: number
    cameraStalled?: boolean
    frameAgeSec?: number
}

function setup() {
    setActivePinia(createPinia())
    const health = useFpsHealthStore()
    const ui = useUIStore()
    const robot = useRobotStore()
    const toast = vi.spyOn(ui, 'showToast')
    health.setConnection(true)

    let last: Feed = { fps: 12, uptimeSec: 60 }

    // One status message, as the backend sends them.
    function feed(next: Feed): void {
        last = { uptimeSec: 60, ...next }
        const status: RobotStatus = {
            connected: true,
            robotCount: 1,
            arduinoState: 'Connected',
            ...last,
        }
        robot.setStatus(status)
        health.recordStatus()
    }

    // Advance wall-clock time while the backend keeps sending status each second.
    function advance(ms: number): void {
        for (let left = ms; left > 0; left -= 1000) {
            vi.advanceTimersByTime(Math.min(1000, left))
            feed(last)
        }
    }

    return { health, ui, robot, toast, feed, advance }
}

describe('fpsHealthStore', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('is neutral until it has a reading', () => {
        const { health } = setup()
        expect(health.level).toBe('unknown')
        expect(health.levelClass).toBe('')
        expect(health.bannerVisible).toBe(false)
        expect(health.stalled).toBe(false)
    })

    it('maps levels to the glow classes', () => {
        const { health, feed } = setup()
        feed({ fps: 12 })
        expect(health.levelClass).toBe('')
        feed({ fps: 6.5 })
        expect(health.levelClass).toBe('fps-health-low')
        feed({ fps: 3 })
        expect(health.levelClass).toBe('fps-health-critical')
    })

    it('low frame rate only glows: no banner, no toast', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 6.5 })
        advance(60_000)
        expect(health.level).toBe('low')
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('ignores the first seconds of process uptime', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 2, uptimeSec: 5 })
        advance(8000)
        expect(health.level).toBe('unknown')
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('ignores missing or zero readings', () => {
        const { health, feed } = setup()
        feed({ fps: 0 })
        expect(health.level).toBe('unknown')
    })

    it('does not warn for a short critical dip', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS - 1000)
        feed({ fps: 12 })
        advance(30_000)
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('shows the banner and one toast once critical has lasted long enough', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        expect(health.bannerVisible).toBe(false)

        advance(FPS_CRITICAL_WARN_AFTER_MS - 1000)
        expect(health.bannerVisible).toBe(false)

        advance(1000)
        expect(health.bannerVisible).toBe(true)
        expect(health.kind).toBe('slow')
        expect(toast).toHaveBeenCalledTimes(1)
        expect(toast.mock.calls[0]?.[0]).toContain('3.0 fps')
        expect(toast.mock.calls[0]?.[1]).toBe('warning')

        feed({ fps: 2.6 })
        feed({ fps: 3.1 })
        advance(20_000)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
    })

    it('keeps the banner dismissed for the rest of the episode', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        health.dismiss()
        expect(health.bannerVisible).toBe(false)

        feed({ fps: 2.8 })
        advance(20_000)
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(1)
    })

    it('announces recovery once and allows a new episode to warn again', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        health.dismiss()

        feed({ fps: 12 })
        expect(health.level).toBe('ok')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(2)
        expect(toast.mock.calls[1]?.[0]).toContain('recovered')
        expect(toast.mock.calls[1]?.[1]).toBe('info')

        feed({ fps: 12.5 })
        expect(toast).toHaveBeenCalledTimes(2)

        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(3)
    })

    it('recovering into the low band still counts as recovered', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        feed({ fps: 6.5 })
        expect(health.level).toBe('low')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenLastCalledWith(expect.stringContaining('recovered'), 'info')
    })

    it('stays critical (no flapping) until the recovery threshold is passed', () => {
        const { health, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        feed({ fps: 5.5 })
        expect(health.level).toBe('critical')
        expect(health.bannerVisible).toBe(true)
    })

    it('a disconnect mid-episode hides the banner without claiming a recovery', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        health.setConnection(false)
        expect(health.level).toBe('unknown')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(1)

        // Reconnecting with the frame rate still bad starts a fresh episode.
        health.setConnection(true)
        expect(health.level).toBe('critical')
        expect(health.bannerVisible).toBe(false)
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
    })

    it('does not evaluate while disconnected', () => {
        const { health, feed } = setup()
        health.setConnection(false)
        feed({ fps: 2 })
        expect(health.level).toBe('unknown')
    })

    it('follows the fps and uptime in the robot store status', async () => {
        const { health, robot } = setup()
        robot.setStatus({
            connected: true,
            fps: 3,
            robotCount: 0,
            arduinoState: 'Connected',
            uptimeSec: 60,
        })
        await nextTick()
        expect(health.level).toBe('critical')

        robot.setStatus({
            connected: true,
            fps: 12,
            robotCount: 0,
            arduinoState: 'Connected',
            uptimeSec: 61,
        })
        await nextTick()
        expect(health.level).toBe('ok')
    })
})

describe('fpsHealthStore: stalled camera', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('shows NO VIDEO state immediately but warns only after 2 s of stall', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 18 })
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2.4 })

        expect(health.stalled).toBe(true)
        expect(health.kind).toBe('stalled')
        expect(health.level).toBe('critical')
        expect(health.levelClass).toBe('fps-health-critical')
        expect(health.stalledSec).toBe(2)
        expect(health.bannerVisible).toBe(false)

        advance(CAMERA_STALLED_WARN_AFTER_MS - 1000)
        expect(health.bannerVisible).toBe(false)
        advance(1000)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
        expect(toast.mock.calls[0]?.[0]).toContain('No video frames')
    })

    it('reads the stalled seconds from frameAgeSec', () => {
        const { health, feed } = setup()
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2.4 })
        expect(health.stalledSec).toBe(2)
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 9.6 })
        expect(health.stalledSec).toBe(10)
    })

    it('a stall is treated as critical even though fps is zero', () => {
        const { health, feed } = setup()
        feed({ fps: 0, cameraStalled: true })
        expect(health.level).toBe('critical')
        expect(health.fps).toBeNull()
    })

    it('announces "Camera video recovered." once and clears the banner', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 3 })
        advance(CAMERA_STALLED_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        feed({ fps: 15 })
        expect(health.stalled).toBe(false)
        expect(health.level).toBe('ok')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(2)
        expect(toast).toHaveBeenLastCalledWith('Camera video recovered.', 'info')

        feed({ fps: 15 })
        expect(toast).toHaveBeenCalledTimes(2)
    })

    it('does not announce recovery if the stall ended before it warned', () => {
        const { toast, feed, advance } = setup()
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2 })
        advance(1000)
        feed({ fps: 15 })
        advance(30_000)
        expect(toast).not.toHaveBeenCalled()
    })

    it('stays dismissed for the rest of the stall, then can warn again', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2 })
        advance(CAMERA_STALLED_WARN_AFTER_MS)
        health.dismiss()
        advance(20_000)
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(1)

        feed({ fps: 15 })
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2 })
        advance(CAMERA_STALLED_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
        expect(toast.mock.calls.filter(c => c[1] === 'warning')).toHaveLength(2)
    })

    it('a slow episode that turns into a stall keeps one episode and switches text', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 3 })
        advance(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
        expect(health.kind).toBe('slow')

        feed({ fps: 0, cameraStalled: true, frameAgeSec: 2.5 })
        expect(health.kind).toBe('stalled')
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
    })

    it('suppresses a stalled flag during warm-up, then warns once warm-up ends', () => {
        const { health, toast, feed, advance } = setup()
        feed({ fps: 0, cameraStalled: true, frameAgeSec: 4, uptimeSec: 5 })
        advance(8000)
        expect(health.stalled).toBe(false)
        expect(health.level).toBe('unknown')
        expect(toast).not.toHaveBeenCalled()

        feed({ fps: 0, cameraStalled: true, frameAgeSec: 16, uptimeSec: 20 })
        expect(health.stalled).toBe(true)
        advance(CAMERA_STALLED_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
    })

    it('does not show a stall while disconnected', () => {
        const { health, feed } = setup()
        health.setConnection(false)
        feed({ fps: 0, cameraStalled: true })
        expect(health.stalled).toBe(false)
        expect(health.level).toBe('unknown')
    })
})

describe('fpsHealthStore: status silence fallback', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('treats > 5 s without any status message as a stall', () => {
        const { health, feed } = setup()
        feed({ fps: 18 })
        vi.advanceTimersByTime(STATUS_SILENCE_MS - 1)
        expect(health.stalled).toBe(false)

        vi.advanceTimersByTime(1)
        expect(health.stalled).toBe(true)
        expect(health.kind).toBe('stalled')
        expect(health.level).toBe('critical')
    })

    it('warns 2 s after the silence is detected and counts the seconds up', () => {
        const { health, toast, feed } = setup()
        feed({ fps: 18 })
        vi.advanceTimersByTime(STATUS_SILENCE_MS)
        expect(health.bannerVisible).toBe(false)

        vi.advanceTimersByTime(CAMERA_STALLED_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
        expect(health.stalledSec).toBeGreaterThanOrEqual(6)

        vi.advanceTimersByTime(3000)
        expect(health.stalledSec).toBeGreaterThanOrEqual(9)
    })

    it('recovers the moment a status message arrives again', () => {
        const { health, toast, feed } = setup()
        feed({ fps: 18 })
        vi.advanceTimersByTime(STATUS_SILENCE_MS + CAMERA_STALLED_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        feed({ fps: 18 })
        expect(health.stalled).toBe(false)
        expect(health.level).toBe('ok')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenLastCalledWith('Camera video recovered.', 'info')
    })

    it('regular status messages keep the fallback from firing', () => {
        const { health, feed, advance } = setup()
        feed({ fps: 18 })
        advance(60_000)
        expect(health.stalled).toBe(false)
    })

    it('does not fire while disconnected, and re-arms on reconnect', () => {
        const { health, feed } = setup()
        feed({ fps: 18 })
        health.setConnection(false)
        vi.advanceTimersByTime(60_000)
        expect(health.stalled).toBe(false)

        health.setConnection(true)
        vi.advanceTimersByTime(STATUS_SILENCE_MS)
        expect(health.stalled).toBe(true)
    })

    it('does not fire during warm-up', () => {
        const { health, feed } = setup()
        feed({ fps: 18, uptimeSec: 3 })
        vi.advanceTimersByTime(STATUS_SILENCE_MS * 3)
        expect(health.stalled).toBe(false)
        expect(health.level).toBe('unknown')
    })
})
