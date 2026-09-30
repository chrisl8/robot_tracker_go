import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUIStore } from '@/stores/uiStore'
import { handleDriveKeyDown, handleDriveKeyUp } from '../driveKeys'

type Modifiers = { metaKey?: boolean; ctrlKey?: boolean; altKey?: boolean }

function key(type: 'keydown' | 'keyup', k: string, init: Modifiers = {}): KeyboardEvent {
    return new KeyboardEvent(type, { key: k, cancelable: true, ...init })
}

describe('drive keys', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
    })

    it('holds a drive key from keydown to keyup', () => {
        const ui = useUIStore()
        handleDriveKeyDown(key('keydown', 'w'), ui)
        expect(ui.keyboard.w).toBe(true)
        handleDriveKeyUp(key('keyup', 'W'), ui)
        expect(ui.keyboard.w).toBe(false)
    })

    it('ignores keys that do not drive the robot', () => {
        const ui = useUIStore()
        const event = key('keydown', 'q')
        handleDriveKeyDown(event, ui)
        expect(Object.values(ui.keyboard).some(Boolean)).toBe(false)
        expect(event.defaultPrevented).toBe(false)
    })

    // Cmd+W, Cmd+S, Cmd+D, Ctrl+A ... are browser shortcuts, not drive
    // commands. macOS also never delivers the keyup for a letter pressed with
    // Cmd held, so treating it as a drive key leaves it "held" and the
    // ControlPanel refresh timer keeps the robot moving until the window loses
    // focus.
    it.each([
        ['meta', { metaKey: true }],
        ['ctrl', { ctrlKey: true }],
        ['alt', { altKey: true }],
    ])('does not drive on %s+key, and leaves the shortcut alone', (_name, mods) => {
        const ui = useUIStore()
        for (const k of ['w', 'a', 's', 'd']) {
            const event = key('keydown', k, mods)
            handleDriveKeyDown(event, ui)
            expect(ui.keyboard[k as 'w' | 'a' | 's' | 'd']).toBe(false)
            expect(event.defaultPrevented).toBe(false)
        }
    })

    it('releases a held drive key when a shortcut modifier goes down', () => {
        const ui = useUIStore()
        handleDriveKeyDown(key('keydown', 'w'), ui) // driving forward
        handleDriveKeyDown(key('keydown', 'd', { metaKey: true }), ui) // Cmd+D
        expect(Object.values(ui.keyboard).some(Boolean)).toBe(false)
    })

    it('still releases a key on keyup while a modifier is held', () => {
        const ui = useUIStore()
        handleDriveKeyDown(key('keydown', 'w'), ui)
        handleDriveKeyUp(key('keyup', 'w', { ctrlKey: true }), ui)
        expect(ui.keyboard.w).toBe(false)
    })
})
