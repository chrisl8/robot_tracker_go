import { describe, it, expect, beforeEach, vi, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUIStore } from '../uiStore'
import type { CalibrationTarget, DetectedTagInfo } from '@/types/api'
import type { PlacementAssessment } from '@/composables/calibrationPlacement'

const TARGET: CalibrationTarget = {
    tagSize: 0.15,
    tags: [{ id: 100, label: 'Center', role: 'center', guideX: 0.5, guideY: 0.5 }],
}

const TAG: DetectedTagInfo = {
    id: 100,
    center: [50, 50],
    corners: [
        [0, 0],
        [100, 0],
        [100, 100],
        [0, 100],
    ],
}

const PLACEMENT: PlacementAssessment = {
    tags: [{ id: 100, label: 'Center', found: true, sizePx: 100, severity: 'ok' }],
    issues: [],
    guides: [
        {
            id: 100,
            label: 'Center',
            cx: 50,
            cy: 50,
            sizePx: 100,
            state: 'inside',
            tagX: 50,
            tagY: 50,
        },
    ],
    allFound: true,
    canCalibrate: true,
}

describe('uiStore calibration', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
    })

    it('starts not calibrated with an empty wizard', () => {
        const store = useUIStore()
        expect(store.calibration.state).toBe('not_calibrated')
        expect(store.detectedTags).toEqual([])
        expect(store.calibrationTarget).toBeNull()
        expect(store.calibrationPlacement).toBeNull()
        expect(store.calibrationStep).toBe('print')
        expect(store.calibrationClearView).toBe(false)
    })

    it('opens and closes the calibration panel', () => {
        const store = useUIStore()
        store.openCalibration()
        expect(store.panels.calibrationOpen).toBe(true)
        store.closeCalibration()
        expect(store.panels.calibrationOpen).toBe(false)
    })

    it('records calibration state with the resolution mismatch flag', () => {
        const store = useUIStore()
        store.setCalibrationState('calibrated', 'ok')
        expect(store.calibration).toEqual({
            state: 'calibrated',
            message: 'ok',
            resolutionMismatch: false,
        })
        store.setCalibrationState('calibrated', 'stale', true)
        expect(store.calibration.resolutionMismatch).toBe(true)
    })

    it('stores the target, detected tags and placement', () => {
        const store = useUIStore()
        store.setCalibrationTarget(TARGET)
        store.setDetectedTags([TAG])
        store.setCalibrationPlacement(PLACEMENT)
        expect(store.calibrationTarget).toEqual(TARGET)
        expect(store.detectedTags).toEqual([TAG])
        expect(store.calibrationPlacement).toEqual(PLACEMENT)
    })

    it('resets wizard state but keeps the fetched target', () => {
        const store = useUIStore()
        store.setCalibrationTarget(TARGET)
        store.setDetectedTags([TAG])
        store.setCalibrationPlacement(PLACEMENT)
        store.resetCalibrationWizard()
        expect(store.detectedTags).toEqual([])
        expect(store.calibrationPlacement).toBeNull()
        expect(store.calibrationTarget).toEqual(TARGET)
    })

    it('clears the canvas view only while the wizard is on the place step', () => {
        const store = useUIStore()
        store.setCalibrationStep('place')
        expect(store.calibrationClearView).toBe(false) // wizard not open yet
        store.openCalibration()
        expect(store.calibrationClearView).toBe(true)
        store.setCalibrationStep('result')
        expect(store.calibrationClearView).toBe(false)
        store.setCalibrationStep('place')
        expect(store.calibrationClearView).toBe(true)
    })

    it('restores the normal view when the wizard closes or resets', () => {
        const store = useUIStore()
        store.openCalibration()
        store.setCalibrationStep('place')
        store.closeCalibration()
        expect(store.calibrationClearView).toBe(false)
        expect(store.calibrationStep).toBe('print')

        store.openCalibration()
        store.setCalibrationStep('place')
        store.resetCalibrationWizard()
        expect(store.calibrationStep).toBe('print')
        expect(store.calibrationClearView).toBe(false)
    })
})

describe('toast and log ids', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
        vi.useFakeTimers()
        vi.setSystemTime(1_700_000_000_000) // Date.now() frozen: same millisecond for everything
    })
    afterEach(() => vi.useRealTimers())

    it('stay unique when created within the same millisecond', () => {
        const store = useUIStore()
        store.showToast('a', 'info', 60_000)
        store.showToast('b', 'info', 60_000)
        store.showToast('c', 'info', 60_000)

        const ids = store.toasts.map(t => t.id)
        expect(new Set(ids).size).toBe(3)
    })

    it('removing one toast removes only that toast', () => {
        const store = useUIStore()
        store.showToast('a', 'info', 60_000)
        store.showToast('b', 'info', 60_000)
        const [first, second] = store.toasts.map(t => t.id)

        store.removeToast(second)

        expect(store.toasts.map(t => t.id)).toEqual([first])
    })

    it('log entries get unique ids too', () => {
        const store = useUIStore()
        store.addLogEntry('info', 'one')
        store.addLogEntry('info', 'two')

        expect(new Set(store.activityLog.map(e => e.id)).size).toBe(store.activityLog.length)
    })
})

describe('checkForUpdate', () => {
    afterEach(() => {
        vi.unstubAllGlobals()
    })

    function serve(body: unknown, ok = true): void {
        vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok, json: () => Promise.resolve(body) }))
    }

    beforeEach(() => {
        setActivePinia(createPinia())
        vi.stubGlobal('__BUILD_ID__', 'mine')
    })

    it('flags an update when the server reports a different build', async () => {
        serve({ id: 'newer' })
        const ui = useUIStore()
        await ui.checkForUpdate()
        expect(ui.updateAvailable).toBe(true)
    })

    it.each([
        ['the same build', { id: 'mine' }, true],
        ['no build embedded', { id: '' }, true],
        ['an error response', { id: 'newer' }, false],
    ])('does not flag an update for %s', async (_name, body, ok) => {
        serve(body, ok)
        const ui = useUIStore()
        await ui.checkForUpdate()
        expect(ui.updateAvailable).toBe(false)
    })

    it('does not flag an update when the server is unreachable', async () => {
        vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new Error('down')))
        const ui = useUIStore()
        await ui.checkForUpdate()
        expect(ui.updateAvailable).toBe(false)
    })
})
