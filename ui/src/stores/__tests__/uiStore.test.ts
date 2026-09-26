import { describe, it, expect, beforeEach } from 'vitest'
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
