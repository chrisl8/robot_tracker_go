import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createApp, nextTick, type App } from 'vue'
import { createPinia, setActivePinia } from 'pinia'
import ControlPanel from '../ControlPanel.vue'
import { useUIStore } from '@/stores/uiStore'
import { useRobotStore } from '@/stores/robotStore'

function sentCommands(fetchMock: ReturnType<typeof vi.fn>): string[] {
    return fetchMock.mock.calls
        .filter(([url]) => url === '/api/command')
        .map(([, init]) => JSON.parse((init as { body: string }).body).command)
}

describe('ControlPanel keyboard safety', () => {
    let app: App
    let root: HTMLElement
    let fetchMock: ReturnType<typeof vi.fn>

    beforeEach(() => {
        fetchMock = vi.fn().mockResolvedValue({ ok: true, json: async () => ({}) })
        vi.stubGlobal('fetch', fetchMock)

        const pinia = createPinia()
        setActivePinia(pinia)
        // fetchControlState() runs on mount; keep it from changing state.
        vi.spyOn(useRobotStore(), 'fetchControlState').mockResolvedValue(undefined as never)
        useRobotStore().controlMode = 'manual'

        root = document.createElement('div')
        document.body.appendChild(root)
        app = createApp(ControlPanel)
        app.use(pinia)
        app.mount(root)
    })

    afterEach(() => {
        app.unmount()
        root.remove()
        vi.unstubAllGlobals()
    })

    it('sends Stop when a held key is released', async () => {
        const ui = useUIStore()
        ui.setKey('w', true)
        await nextTick()
        ui.setKey('w', false)
        await nextTick()

        expect(sentCommands(fetchMock)).toEqual(['F', 'S'])
    })

    it('sends Stop when the keyboard state is reset (window blur / tab hidden)', async () => {
        const ui = useUIStore()
        ui.setKey('w', true)
        await nextTick()

        ui.resetKeyboard()
        await nextTick()

        const cmds = sentCommands(fetchMock)
        expect(cmds[cmds.length - 1]).toBe('S')
    })

    it('does not stop the robot when the pointer leaves an unpressed D-pad button', async () => {
        const btn = root.querySelector('.dpad-btn.forward')
        expect(btn).not.toBeNull()

        btn!.dispatchEvent(new MouseEvent('mouseleave'))
        await nextTick()

        expect(sentCommands(fetchMock)).toEqual([])
    })

    it('still stops the robot when a pressed D-pad button is released or left', async () => {
        const btn = root.querySelector('.dpad-btn.forward')!

        btn.dispatchEvent(new MouseEvent('mousedown'))
        await nextTick()
        btn.dispatchEvent(new MouseEvent('mouseleave'))
        await nextTick()

        expect(sentCommands(fetchMock)).toEqual(['F', 'S'])
    })
})
