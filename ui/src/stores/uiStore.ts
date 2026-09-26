import { defineStore } from 'pinia'
import { computed, ref } from 'vue'
import type { CalibrationState, CalibrationTarget, DetectedTagInfo } from '@/types/api'
import type { PlacementAssessment } from '@/composables/calibrationPlacement'
import type {
    CalibrationStep,
    Toast,
    ToastType,
    PanelState,
    KeyboardState,
    LogEntry,
    LogEntryType,
} from '@/types/ui'

export const useUIStore = defineStore('ui', () => {
    // State
    const panels = ref<PanelState>({
        obstacleOpen: false,
        calibrationOpen: false,
        leftPanelOpen: true,
        rightPanelOpen: true,
    })

    const toasts = ref<Toast[]>([])

    const calibration = ref<CalibrationState>({
        state: 'not_calibrated',
        message: '',
    })

    const detectedTags = ref<DetectedTagInfo[]>([])
    const calibrationTarget = ref<CalibrationTarget | null>(null)
    const calibrationPlacement = ref<PlacementAssessment | null>(null)
    const calibrationStep = ref<CalibrationStep>('print')
    // While placing calibration tags the canvas shows only the calibration view.
    const calibrationClearView = computed(
        () => panels.value.calibrationOpen && calibrationStep.value === 'place'
    )

    const showFootprints = ref(true)
    const activityLog = ref<LogEntry[]>([])

    const keyboard = ref<KeyboardState>({
        w: false,
        a: false,
        s: false,
        d: false,
        x: false,
        e: false,
        z: false,
        c: false,
    })

    // Actions - Panels
    function toggleObstaclePanel(): void {
        panels.value.obstacleOpen = !panels.value.obstacleOpen
    }

    function openObstaclePanel(): void {
        panels.value.obstacleOpen = true
    }

    function closeObstaclePanel(): void {
        panels.value.obstacleOpen = false
    }

    function openCalibration(): void {
        panels.value.calibrationOpen = true
    }

    function closeCalibration(): void {
        panels.value.calibrationOpen = false
        calibrationStep.value = 'print'
    }

    // Actions - Toasts
    function showToast(message: string, type: ToastType = 'info', duration: number = 3000): void {
        const id = Date.now().toString()
        const toast: Toast = { id, message, type, duration }
        toasts.value.push(toast)

        if (duration > 0) {
            setTimeout(() => {
                removeToast(id)
            }, duration)
        }
    }

    function removeToast(id: string): void {
        const idx = toasts.value.findIndex((t: Toast) => t.id === id)
        if (idx >= 0) {
            toasts.value.splice(idx, 1)
        }
    }

    // Actions - Calibration
    function setCalibrationState(
        state: CalibrationState['state'],
        message: string = '',
        resolutionMismatch: boolean = false
    ): void {
        calibration.value = { state, message, resolutionMismatch }
    }

    function setDetectedTags(tags: DetectedTagInfo[]): void {
        detectedTags.value = tags
    }

    function setCalibrationTarget(target: CalibrationTarget | null): void {
        calibrationTarget.value = target
    }

    function setCalibrationPlacement(placement: PlacementAssessment | null): void {
        calibrationPlacement.value = placement
    }

    function setCalibrationStep(step: CalibrationStep): void {
        calibrationStep.value = step
    }

    function resetCalibrationWizard(): void {
        detectedTags.value = []
        calibrationPlacement.value = null
        calibrationStep.value = 'print'
    }

    // Actions - Keyboard
    function setKey(key: keyof KeyboardState, pressed: boolean): void {
        keyboard.value[key] = pressed
    }

    function resetKeyboard(): void {
        keyboard.value = {
            w: false,
            a: false,
            s: false,
            d: false,
            x: false,
            e: false,
            z: false,
            c: false,
        }
    }

    function toggleFootprints(): void {
        showFootprints.value = !showFootprints.value
    }

    // Actions - Activity Log
    function addLogEntry(type: LogEntryType, message: string): void {
        const entry: LogEntry = {
            id: Date.now().toString(),
            timestamp: new Date(),
            type,
            message,
        }
        activityLog.value.push(entry)
        if (activityLog.value.length > 50) {
            activityLog.value.shift()
        }
    }

    function clearLog(): void {
        activityLog.value = []
    }

    return {
        // State
        panels,
        toasts,
        calibration,
        detectedTags,
        calibrationTarget,
        calibrationPlacement,
        calibrationStep,
        calibrationClearView,
        keyboard,
        showFootprints,
        activityLog,
        // Panel actions
        toggleObstaclePanel,
        openObstaclePanel,
        closeObstaclePanel,
        openCalibration,
        closeCalibration,
        // Toast actions
        showToast,
        removeToast,
        // Calibration actions
        setCalibrationState,
        setDetectedTags,
        setCalibrationTarget,
        setCalibrationPlacement,
        setCalibrationStep,
        resetCalibrationWizard,
        // Keyboard actions
        setKey,
        resetKeyboard,
        // Footprint actions
        toggleFootprints,
        // Activity Log actions
        addLogEntry,
        clearLog,
    }
})
