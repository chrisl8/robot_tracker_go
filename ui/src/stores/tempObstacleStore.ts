import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { ForegroundState, TempObstacle, TempObstaclesPayload } from '@/types/api'
import { useUIStore } from '@/stores/uiStore'

export interface AbsorbPrompt {
    // Natural-frame pixel the user clicked (sent to the backend)
    x: number
    y: number
    // Canvas pixel to anchor the popover at
    canvasX: number
    canvasY: number
}

// Extracts a specific reason from a failed response.
async function failureReason(response: Response): Promise<string> {
    try {
        const body = (await response.json()) as { error?: string }
        if (body.error) return body.error
    } catch {
        // not JSON: fall through to the status text
    }
    return `${response.status} ${response.statusText}`.trim()
}

export const useTempObstacleStore = defineStore('tempObstacle', () => {
    const obstacles = ref<TempObstacle[]>([])
    const enabled = ref(false)
    const applied = ref(false)
    const warming = ref(false)
    const guarded = ref(false)
    const showMask = ref(false)
    const absorbPrompt = ref<AbsorbPrompt | null>(null)

    const count = computed(() => obstacles.value.length)

    function setFromMessage(payload: TempObstaclesPayload): void {
        obstacles.value = Array.isArray(payload.obstacles) ? payload.obstacles : []
        enabled.value = payload.enabled === true
        applied.value = payload.applied === true
        warming.value = payload.warming === true
        guarded.value = payload.guarded === true
        const prompt = absorbPrompt.value
        if (prompt && !obstacles.value.some(o => containsPoint(o, prompt.x, prompt.y))) {
            absorbPrompt.value = null
        }
    }

    function containsPoint(obs: TempObstacle, x: number, y: number): boolean {
        return (
            x >= obs.pixel_top_left[0] &&
            x <= obs.pixel_bottom_right[0] &&
            y >= obs.pixel_top_left[1] &&
            y <= obs.pixel_bottom_right[1]
        )
    }

    function setFlags(state: ForegroundState): void {
        enabled.value = state.enabled === true
        applied.value = state.applied === true
        warming.value = state.warming === true
        guarded.value = state.guarded === true
    }

    async function loadState(): Promise<void> {
        const ui = useUIStore()
        try {
            const response = await fetch('/api/foreground/state')
            if (!response.ok) {
                ui.showToast(
                    `Could not read obstacle detector state: ${await failureReason(response)}`,
                    'error'
                )
                return
            }
            setFlags((await response.json()) as ForegroundState)
        } catch {
            ui.showToast('Could not reach the tracker to read the obstacle detector state', 'error')
        }
    }

    // Sends a control request; toasts a specific reason on failure.
    async function send(path: string, body: unknown, what: string): Promise<boolean> {
        const ui = useUIStore()
        try {
            const response = await fetch(path, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: body === undefined ? undefined : JSON.stringify(body),
            })
            if (!response.ok) {
                ui.showToast(`${what} failed: ${await failureReason(response)}`, 'error')
                return false
            }
            return true
        } catch {
            ui.showToast(`${what} failed: could not reach the tracker service`, 'error')
            return false
        }
    }

    async function setEnabled(value: boolean): Promise<void> {
        if (await send('/api/foreground/enabled', { enabled: value }, 'Changing detection')) {
            enabled.value = value
        }
    }

    async function setApply(value: boolean): Promise<void> {
        if (await send('/api/foreground/apply', { apply: value }, 'Changing obstacle steering')) {
            applied.value = value
        }
    }

    async function resetBackground(): Promise<void> {
        const ui = useUIStore()
        if (await send('/api/foreground/reset', undefined, 'Resetting the background')) {
            warming.value = true
            ui.showToast('Learning the background: floor should be clear', 'info')
        }
    }

    async function absorb(x: number, y: number): Promise<boolean> {
        const ok = await send('/api/foreground/absorb', { x, y }, 'Absorbing the obstacle')
        if (ok) {
            useUIStore().showToast('Obstacle treated as floor', 'success')
        }
        absorbPrompt.value = null
        return ok
    }

    function promptAbsorb(prompt: AbsorbPrompt): void {
        absorbPrompt.value = prompt
    }

    function dismissAbsorb(): void {
        absorbPrompt.value = null
    }

    function toggleMask(): void {
        showMask.value = !showMask.value
    }

    return {
        obstacles,
        enabled,
        applied,
        warming,
        guarded,
        showMask,
        absorbPrompt,
        count,
        setFromMessage,
        loadState,
        setEnabled,
        setApply,
        resetBackground,
        absorb,
        promptAbsorb,
        dismissAbsorb,
        toggleMask,
    }
})
