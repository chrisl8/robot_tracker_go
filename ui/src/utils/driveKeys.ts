import type { KeyboardState } from '@/types/ui'

const VALID_KEYS: readonly string[] = ['w', 'a', 's', 'd', 'x', 'e', 'z', 'c']
const PREVENT_DEFAULT_KEYS: readonly string[] = ['w', 'a', 's', 'd', 'x', 'e']

// What the key handlers need from the UI store.
export interface DriveKeyTarget {
    setKey(key: keyof KeyboardState, pressed: boolean): void
    resetKeyboard(): void
}

// Letter keys pressed with Cmd/Ctrl/Alt are browser or OS shortcuts (Cmd+W,
// Cmd+S, Cmd+D, ...), not drive commands. macOS also never delivers the keyup
// for a letter pressed while Cmd is held, so counting one as held would leave
// the robot driving: ControlPanel re-sends a held key every 250 ms, which
// defeats the server's 2 s deadman until the window loses focus. A shortcut
// therefore also releases whatever was held.
export function handleDriveKeyDown(event: KeyboardEvent, target: DriveKeyTarget): void {
    if (event.metaKey || event.ctrlKey || event.altKey) {
        target.resetKeyboard()
        return
    }

    const key = event.key.toLowerCase()
    if (!VALID_KEYS.includes(key)) return

    target.setKey(key as keyof KeyboardState, true)
    if (PREVENT_DEFAULT_KEYS.includes(key)) {
        event.preventDefault()
    }
}

// A keyup always releases, whatever modifiers are down.
export function handleDriveKeyUp(event: KeyboardEvent, target: DriveKeyTarget): void {
    const key = event.key.toLowerCase()
    if (VALID_KEYS.includes(key)) {
        target.setKey(key as keyof KeyboardState, false)
    }
}
