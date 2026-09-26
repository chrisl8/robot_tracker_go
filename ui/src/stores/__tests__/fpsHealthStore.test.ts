import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { nextTick } from 'vue'
import { FPS_CRITICAL_WARN_AFTER_MS } from '@/composables/fpsHealth'
import { useFpsHealthStore } from '../fpsHealthStore'
import { useRobotStore } from '../robotStore'
import { useUIStore } from '../uiStore'

function setup() {
    setActivePinia(createPinia())
    const health = useFpsHealthStore()
    const ui = useUIStore()
    const robot = useRobotStore()
    const toast = vi.spyOn(ui, 'showToast')
    health.setConnection(true)
    return { health, ui, robot, toast }
}

const settled = (fps: number) => ({ fps, uptimeSec: 60 })

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
    })

    it('maps levels to the glow classes', () => {
        const { health } = setup()
        health.evaluate(settled(12))
        expect(health.levelClass).toBe('')
        health.evaluate(settled(6.5))
        expect(health.levelClass).toBe('fps-health-low')
        health.evaluate(settled(3))
        expect(health.levelClass).toBe('fps-health-critical')
    })

    it('low frame rate only glows: no banner, no toast', () => {
        const { health, toast } = setup()
        health.evaluate(settled(6.5))
        vi.advanceTimersByTime(60_000)
        expect(health.level).toBe('low')
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('ignores the first seconds of process uptime', () => {
        const { health, toast } = setup()
        health.evaluate({ fps: 2, uptimeSec: 5 })
        vi.advanceTimersByTime(30_000)
        expect(health.level).toBe('unknown')
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('ignores missing or zero readings', () => {
        const { health } = setup()
        health.evaluate({ fps: 0, uptimeSec: 60 })
        expect(health.level).toBe('unknown')
        health.evaluate({ fps: undefined, uptimeSec: 60 })
        expect(health.level).toBe('unknown')
    })

    it('does not warn for a short critical dip', () => {
        const { health, toast } = setup()
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS - 1)
        health.evaluate(settled(12))
        vi.advanceTimersByTime(30_000)
        expect(health.bannerVisible).toBe(false)
        expect(toast).not.toHaveBeenCalled()
    })

    it('shows the banner and one toast once critical has lasted long enough', () => {
        const { health, toast } = setup()
        health.evaluate(settled(3))
        expect(health.bannerVisible).toBe(false)

        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS - 1)
        expect(health.bannerVisible).toBe(false)

        vi.advanceTimersByTime(1)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
        expect(toast.mock.calls[0]?.[0]).toContain('3.0 fps')
        expect(toast.mock.calls[0]?.[1]).toBe('warning')

        health.evaluate(settled(2.6))
        health.evaluate(settled(3.1))
        vi.advanceTimersByTime(20_000)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(1)
    })

    it('keeps the banner dismissed for the rest of the episode', () => {
        const { health, toast } = setup()
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        health.dismiss()
        expect(health.bannerVisible).toBe(false)

        health.evaluate(settled(2.8))
        vi.advanceTimersByTime(20_000)
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(1)
    })

    it('announces recovery once and allows a new episode to warn again', () => {
        const { health, toast } = setup()
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        health.dismiss()

        health.evaluate(settled(12))
        expect(health.level).toBe('ok')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(2)
        expect(toast.mock.calls[1]?.[0]).toContain('recovered')
        expect(toast.mock.calls[1]?.[1]).toBe('info')

        health.evaluate(settled(12.5))
        expect(toast).toHaveBeenCalledTimes(2)

        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
        expect(toast).toHaveBeenCalledTimes(3)
    })

    it('recovering into the low band still counts as recovered', () => {
        const { health, toast } = setup()
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        health.evaluate(settled(6.5))
        expect(health.level).toBe('low')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenLastCalledWith(expect.stringContaining('recovered'), 'info')
    })

    it('stays critical (no flapping) until the recovery threshold is passed', () => {
        const { health } = setup()
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        health.evaluate(settled(5.5))
        expect(health.level).toBe('critical')
        expect(health.bannerVisible).toBe(true)
    })

    it('a disconnect mid-episode hides the banner without claiming a recovery', () => {
        const { health, robot, toast } = setup()
        robot.setStatus({
            connected: true,
            fps: 3,
            robotCount: 1,
            arduinoState: 'Connected',
            uptimeSec: 60,
        })
        health.evaluate(settled(3))
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)

        health.setConnection(false)
        expect(health.level).toBe('unknown')
        expect(health.bannerVisible).toBe(false)
        expect(toast).toHaveBeenCalledTimes(1)

        // Reconnecting with the frame rate still bad starts a fresh episode.
        health.setConnection(true)
        expect(health.level).toBe('critical')
        expect(health.bannerVisible).toBe(false)
        vi.advanceTimersByTime(FPS_CRITICAL_WARN_AFTER_MS)
        expect(health.bannerVisible).toBe(true)
    })

    it('does not evaluate while disconnected', () => {
        const { health } = setup()
        health.setConnection(false)
        health.evaluate(settled(2))
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
