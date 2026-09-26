import { describe, it, expect, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useUIStore } from '../uiStore'
import type { CalibrationTarget, DetectedTagInfo } from '@/types/api'
import type { PlacementAssessment } from '@/composables/calibrationPlacement'

const TARGET: CalibrationTarget = {
    tagSize: 0.15,
    defaultWidth: 1.0,
    defaultDepth: 0.6,
    tags: [{ id: 100, label: 'Center', role: 'center', col: 0, row: 0 }],
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
})
