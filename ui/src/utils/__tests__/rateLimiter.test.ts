import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import { createRateLimiter } from '../rateLimiter'

describe('createRateLimiter', () => {
    beforeEach(() => {
        vi.useFakeTimers()
    })

    afterEach(() => {
        vi.useRealTimers()
    })

    it('allows the first call, then at most one per interval', () => {
        const allow = createRateLimiter(8000)
        expect(allow()).toBe(true)
        expect(allow()).toBe(false)
        vi.advanceTimersByTime(7999)
        expect(allow()).toBe(false)
        vi.advanceTimersByTime(1)
        expect(allow()).toBe(true)
        expect(allow()).toBe(false)
    })

    it('only counts allowed calls toward the interval', () => {
        const allow = createRateLimiter(1000)
        expect(allow()).toBe(true)
        vi.advanceTimersByTime(600)
        expect(allow()).toBe(false)
        vi.advanceTimersByTime(600)
        expect(allow()).toBe(true)
    })

    it('uses an injected clock', () => {
        let t = 0
        const allow = createRateLimiter(10, () => t)
        expect(allow()).toBe(true)
        t = 9
        expect(allow()).toBe(false)
        t = 10
        expect(allow()).toBe(true)
    })
})
