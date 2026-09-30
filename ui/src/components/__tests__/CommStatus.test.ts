import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import CommStatus from '../CommStatus.vue'
import { useRobotStore } from '@/stores/robotStore'
import type { RobotStatus } from '@/types/api'

describe('CommStatus robot rows', () => {
    let app: App
    let root: HTMLElement

    const status = (over: Partial<RobotStatus> = {}): RobotStatus => ({
        connected: true,
        fps: 30,
        robotCount: 0,
        arduinoState: 'Connected',
        robotLink: 'alive',
        ...over,
    })

    async function show(s: RobotStatus): Promise<string> {
        useRobotStore().status = s
        await nextTick()
        return root.textContent ?? ''
    }

    beforeEach(() => {
        const pinia = createPinia()
        setActivePinia(pinia)
        root = document.createElement('div')
        document.body.appendChild(root)
        app = createApp(CommStatus)
        app.use(pinia)
        app.mount(root)
    })

    afterEach(() => {
        app.unmount()
        root.remove()
    })

    it('shows the robot as responding, with its servo state', async () => {
        const text = await show(status({ robotServos: 'asleep' }))

        expect(text).toContain('Responding')
        expect(text).toContain('Asleep')
    })

    it('shows NOT RESPONDING and hides servo details when the robot is silent', async () => {
        const text = await show(status({ robotLink: 'silent' }))

        expect(text).toContain('NOT RESPONDING')
        expect(text).not.toContain('Servos')
    })

    it('does not blame the robot when the controller itself is disconnected', async () => {
        const text = await show(status({ arduinoState: 'Disconnected', robotLink: 'silent' }))

        expect(text).toContain('No controller')
        expect(text).not.toContain('NOT RESPONDING')
    })

    it('shows the restart count only once there has been a restart', async () => {
        expect(await show(status({ robotReboots: 0 }))).not.toContain('Restarts')

        const text = await show(status({ robotReboots: 2 }))

        expect(text).toContain('Restarts')
        expect(text).toContain('2')
    })
})
