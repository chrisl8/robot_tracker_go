import { ref, watch, onScopeDispose, type Ref } from 'vue'

const POLL_MS = 500

// Polls the detector's debug mask while `active` is true and exposes the
// latest image as a blob URL. URLs are revoked when replaced and on stop.
export function useMaskPreview(active: Ref<boolean>, onError: (message: string) => void) {
    const url = ref<string | null>(null)
    let timer: ReturnType<typeof setInterval> | null = null
    let inFlight = false
    let failed = false

    function revoke(): void {
        if (url.value) {
            URL.revokeObjectURL(url.value)
            url.value = null
        }
    }

    async function poll(): Promise<void> {
        if (inFlight) return
        inFlight = true
        try {
            const response = await fetch('/api/foreground/debug.jpg')
            if (response.status === 204) return
            if (!response.ok) {
                if (!failed)
                    onError(`Mask preview failed: ${response.status} ${response.statusText}`)
                failed = true
                return
            }
            failed = false
            const blob = await response.blob()
            const next = URL.createObjectURL(blob)
            revoke()
            url.value = next
        } catch {
            if (!failed) onError('Mask preview failed: could not reach the tracker service')
            failed = true
        } finally {
            inFlight = false
        }
    }

    function stop(): void {
        if (timer !== null) {
            clearInterval(timer)
            timer = null
        }
        revoke()
    }

    watch(
        active,
        on => {
            stop()
            if (on) {
                failed = false
                void poll()
                timer = setInterval(() => void poll(), POLL_MS)
            }
        },
        { immediate: true }
    )

    onScopeDispose(stop)

    return { url, poll, stop }
}
