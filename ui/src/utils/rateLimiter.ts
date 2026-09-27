/**
 * Returns a function that answers "may I act now?": true at most once per
 * intervalMs. `now` is injectable so tests control the clock.
 */
export function createRateLimiter(intervalMs: number, now: () => number = Date.now): () => boolean {
    let last = Number.NEGATIVE_INFINITY
    return () => {
        const t = now()
        if (t - last < intervalMs) return false
        last = t
        return true
    }
}
