import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import MotionStatus from '../MotionStatus.vue'
import { useRobotStore } from '@/stores/robotStore'
import type { MotionStatus as Motion, RobotStatus } from '@/types/api'

describe('MotionStatus', () => {
    let app: App
    let root: HTMLElement

    const motion = (over: Partial<Motion> = {}): Motion => ({
        code: 'no_goal',
        text: 'No goal set',
        severity: 'info',
        ...over,
    })

    beforeEach(() => {
        const pinia = createPinia()
        setActivePinia(pinia)
        root = document.createElement('div')
        document.body.appendChild(root)
        app = createApp(MotionStatus, { overlay: true })
        app.use(pinia)
        app.mount(root)
    })

    afterEach(() => {
        app.unmount()
        root.remove()
    })

    it('shows nothing until the backend has said something', () => {
        expect(root.textContent?.trim()).toBe('')
    })

    it('shows the text and severity of a "motion" message', async () => {
        const store = useRobotStore()
        store.handleWebSocketMessage({
            type: 'motion',
            motion: motion({
                code: 'no_path',
                text: 'Goal set, but there is no path',
                severity: 'error',
            }),
        })
        await nextTick()
        const el = root.querySelector('.motion')
        expect(el?.textContent).toContain('Goal set, but there is no path')
        expect(el?.classList.contains('sev-error')).toBe(true)
        expect(el?.getAttribute('data-code')).toBe('no_path')
    })

    it('follows the motion carried by the 1 s status message', async () => {
        const store = useRobotStore()
        const status: RobotStatus = {
            connected: true,
            fps: 30,
            robotCount: 1,
            arduinoState: 'Connected',
            motion: motion({
                code: 'driving',
                text: 'Driving forward toward the goal',
                severity: 'ok',
            }),
        }
        store.handleWebSocketMessage({ type: 'status', status } as never)
        await nextTick()
        expect(root.textContent).toContain('Driving forward')
        expect(root.querySelector('.sev-ok')).not.toBeNull()
    })

    it('keeps the last motion when an older backend sends a status without one', async () => {
        const store = useRobotStore()
        store.motion = motion()
        store.handleWebSocketMessage({
            type: 'status',
            status: { connected: true, fps: 1, robotCount: 0, arduinoState: 'Connected' },
        } as never)
        await nextTick()
        expect(root.textContent).toContain('No goal set')
    })
})
