import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { RobotStatus } from '@/types/api'

const status = (over: Partial<RobotStatus> = {}): RobotStatus => ({
    connected: true,
    fps: 30,
    robotCount: 0,
    arduinoState: 'Connected',
    robotLink: 'alive',
    ...over,
})

async function setup() {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    const { useRobotStore } = await import('@/stores/robotStore')
    const { useUIStore } = await import('@/stores/uiStore')
    return { robot: useRobotStore(), ui: useUIStore() }
}

const errorToasts = (ui: { toasts: { type: string }[] }) =>
    ui.toasts.filter(t => t.type === 'error').length

describe('Robot link alert', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    it('warns once when the robot goes silent while the Arduino is connected', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status())
        expect(errorToasts(ui)).toBe(0)

        robot.setStatus(status({ robotLink: 'silent' }))
        robot.setStatus(status({ robotLink: 'silent' }))
        robot.setStatus(status({ robotLink: 'silent' }))

        expect(errorToasts(ui)).toBe(1)
        expect(ui.toasts.find(t => t.type === 'error')?.message).toMatch(/not responding/i)
        expect(ui.activityLog.filter(e => e.type === 'error')).toHaveLength(1)
    })

    it('announces the recovery, and warns again on the next outage', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotLink: 'silent' }))
        robot.setStatus(status({ robotLink: 'alive' }))
        expect(ui.toasts.filter(t => t.type === 'success')).toHaveLength(1)

        robot.setStatus(status({ robotLink: 'silent' }))
        expect(errorToasts(ui)).toBe(2)
    })

    it('does not warn while still checking, or for an older backend without robotLink', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotLink: 'unknown' }))
        robot.setStatus(status({ robotLink: undefined }))

        expect(ui.toasts).toHaveLength(0)
    })

    it('does not blame the robot when the Arduino itself is disconnected', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ arduinoState: 'Disconnected', robotLink: 'silent' }))

        expect(ui.toasts).toHaveLength(0)
    })

    it('resets quietly when the Arduino drops, then warns again if the robot is still silent', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotLink: 'silent' }))
        robot.setStatus(status({ arduinoState: 'Disconnected', robotLink: 'unknown' }))
        expect(ui.toasts.filter(t => t.type === 'success')).toHaveLength(0)

        robot.setStatus(status({ robotLink: 'silent' }))
        expect(errorToasts(ui)).toBe(2)
    })
})

describe('Robot restart alert', () => {
    beforeEach(() => {
        vi.clearAllMocks()
    })

    const warnings = (ui: { toasts: { type: string; message: string }[] }) =>
        ui.toasts.filter(t => t.type === 'warning')

    it('does not announce restarts that happened before the page opened', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotReboots: 3 }))

        expect(warnings(ui)).toHaveLength(0)
    })

    it('warns when the restart count grows, once per restart', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotReboots: 0 }))
        robot.setStatus(status({ robotReboots: 0 }))
        expect(warnings(ui)).toHaveLength(0)

        robot.setStatus(status({ robotReboots: 1 }))
        robot.setStatus(status({ robotReboots: 1 }))
        expect(warnings(ui)).toHaveLength(1)
        expect(warnings(ui)[0]?.message).toMatch(/restarted/i)
        expect(ui.activityLog.filter(e => e.type === 'warning')).toHaveLength(1)

        robot.setStatus(status({ robotReboots: 2 }))
        expect(warnings(ui)).toHaveLength(2)
    })

    it('treats a lower count (the service restarted) as a new baseline, not a restart', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotReboots: 5 }))
        robot.setStatus(status({ robotReboots: 0 }))
        expect(warnings(ui)).toHaveLength(0)

        robot.setStatus(status({ robotReboots: 1 }))
        expect(warnings(ui)).toHaveLength(1)
    })

    it('ignores an older backend that does not send the count', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotReboots: undefined }))
        robot.setStatus(status({ robotReboots: 1 }))

        expect(warnings(ui)).toHaveLength(0)
    })
})

describe('Robot silent message', () => {
    it('tells the user the goal was released', async () => {
        const { robot, ui } = await setup()

        robot.setStatus(status({ robotLink: 'silent' }))

        expect(ui.toasts.find(t => t.type === 'error')?.message).toMatch(/goal was released/i)
    })
})
