import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useRobotStore } from '../robotStore'
import { useUIStore } from '../uiStore'
import type { WebSocketMessage } from '@/types/api'

function setup() {
    setActivePinia(createPinia())
    const robot = useRobotStore()
    const toast = vi.spyOn(useUIStore(), 'showToast')
    return { robot, toast }
}

function json(status: number, body: unknown): Response {
    return new Response(JSON.stringify(body), {
        status,
        headers: { 'Content-Type': 'application/json' },
    })
}

describe('control state broadcasts', () => {
    it('applies a control_state message from another operator or tab', () => {
        const { robot } = setup()
        expect(robot.controlMode).toBe('hold')

        robot.handleWebSocketMessage({
            type: 'control_state',
            control: { mode: 'manual', emergency_stopped: false },
        } as WebSocketMessage)
        expect(robot.controlMode).toBe('manual')
        expect(robot.emergencyStopped).toBe(false)

        robot.handleWebSocketMessage({
            type: 'control_state',
            control: { mode: 'hold', emergency_stopped: true },
        } as WebSocketMessage)
        expect(robot.controlMode).toBe('hold')
        expect(robot.emergencyStopped).toBe(true)
    })
})

describe('failed control requests are visible', () => {
    let fetchMock: ReturnType<typeof vi.fn>
    beforeEach(() => {
        fetchMock = vi.fn()
        vi.stubGlobal('fetch', fetchMock)
        vi.spyOn(console, 'error').mockImplementation(() => {})
    })
    afterEach(() => {
        vi.unstubAllGlobals()
        vi.restoreAllMocks()
    })

    it('a rejected mode change shows the server reason and re-reads the real state', async () => {
        const { robot, toast } = setup()
        robot.controlMode = 'manual' // our (stale) view
        fetchMock.mockImplementation(async (url: string) =>
            url === '/api/mode'
                ? json(409, { error: 'cannot change mode while emergency stop is active' })
                : json(200, { mode: 'hold', emergency_stopped: true })
        )

        const ok = await robot.setControlMode('autonomous')

        expect(ok).toBe(false)
        expect(toast).toHaveBeenCalledWith(
            expect.stringContaining('emergency stop is active'),
            'error'
        )
        expect(robot.controlMode).toBe('hold')
        expect(robot.emergencyStopped).toBe(true)
    })

    it('an unreachable server is reported when changing mode', async () => {
        const { robot, toast } = setup()
        fetchMock.mockRejectedValue(new Error('network down'))

        expect(await robot.setControlMode('manual')).toBe(false)
        expect(toast).toHaveBeenCalledWith(expect.stringContaining('unreachable'), 'error')
    })

    it('a failed clear-goal request is reported', async () => {
        const { robot, toast } = setup()
        robot.setDestination({ id: 'd', robot_id: 1, x: 1, y: 1 })
        fetchMock.mockResolvedValue(json(500, { error: 'boom' }))

        expect(await robot.clearDestination()).toBe(false)
        expect(toast).toHaveBeenCalledWith(expect.stringContaining('clear the goal'), 'error')
    })
})
