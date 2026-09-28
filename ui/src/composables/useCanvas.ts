import { ref, computed, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useTempObstacleStore } from '@/stores/tempObstacleStore'
import { useUIStore } from '@/stores/uiStore'
import {
    canvasToNaturalShared,
    naturalToCanvas,
    updateVideoScale,
    videoScale,
} from '@/utils/canvasTransform'
import { createTrackAnimation } from './canvas/trackAnimation'
import { createTrackRenderer } from './canvas/renderTracks'
import { createOverlayRenderer, type FlashPoint } from './canvas/renderOverlays'
import { createCalibrationRenderer } from './canvas/renderCalibration'
import { createCanvasInteraction, type CanvasPoint } from './canvas/useCanvasInteraction'

export type { CanvasPoint }
// Re-exported for existing importers; the implementation lives in utils/canvasTransform.ts.
export { canvasToNaturalShared }

/**
 * The overlay canvas drawn on top of the video: tracks, paths, destination,
 * obstacles-in-progress, calibration aids.
 *
 * This composable is only the orchestrator — it owns the canvas/context refs, the
 * requestAnimationFrame loop, the dirty-flag watcher, resize handling, and DOM
 * listener attach/detach, and calls into focused modules for everything else:
 *
 *  - utils/canvasTransform.ts             video<->canvas coordinate math + measuring
 *  - utils/obstacleCollision.ts           circle-vs-obstacle test for destinations
 *  - canvas/trackAnimation.ts             particle / phase animation state
 *  - canvas/renderTracks.ts               footprints, track boxes, movement indicators
 *  - canvas/renderOverlays.ts             paths, destination, flash, drawing box, temp obstacles
 *  - canvas/renderCalibration.ts          calibration guides and detected-tag outlines
 *  - canvas/useCanvasInteraction.ts       mouse/keyboard handlers and hit-testing
 *  - canvas/theme.ts                      colors and fonts
 */
export function useCanvas(canvasRef: Ref<HTMLCanvasElement | null>) {
    const robotStore = useRobotStore()
    const obstacleStore = useObstacleStore()
    const tempObstacleStore = useTempObstacleStore()
    const uiStore = useUIStore()

    const canvas = ref<HTMLCanvasElement | null>(null)
    const context = ref<CanvasRenderingContext2D | null>(null)
    const dimensions = ref({ width: 0, height: 0 })
    const mousePosition = ref<{ x: number; y: number } | null>(null)
    const flashInvalid = ref<FlashPoint | null>(null)

    // Export videoScale for use by other modules (e.g., robotStore)
    const getVideoScale = () => videoScale.value

    const ctx = computed(() => context.value)

    // Alias, not a second implementation: exactly one canvas->natural implementation
    // exists (utils/canvasTransform.ts), shared with robotStore.confirmDestination.
    const canvasToNatural = canvasToNaturalShared

    const measureVideoScale = (): void => updateVideoScale(canvas.value)

    const animation = createTrackAnimation()
    const trackRenderer = createTrackRenderer({ ctx, dimensions, robotStore, uiStore, animation })
    const overlayRenderer = createOverlayRenderer({
        ctx,
        robotStore,
        obstacleStore,
        tempObstacleStore,
        animation,
        mousePosition,
        flashInvalid,
    })
    const calibrationRenderer = createCalibrationRenderer({ ctx, uiStore })
    const interaction = createCanvasInteraction({
        canvas,
        mousePosition,
        robotStore,
        obstacleStore,
        tempObstacleStore,
        uiStore,
        triggerInvalidFlash: overlayRenderer.triggerInvalidFlash,
    })
    const { getCanvasPoint, onMouseMove, onClick, onKeyDown } = interaction

    let animationFrameId: number | null = null
    let lastFrameTime = 0
    let dirty = true

    // Watch for data changes — sets dirty flag for next rAF frame
    watch(
        () => [
            robotStore.tracks,
            robotStore.selectedTrackId,
            robotStore.destinationMode,
            robotStore.destination,
            robotStore.paths,
            obstacleStore.obstacles,
            obstacleStore.drawRect,
            uiStore.panels,
            uiStore.calibrationTarget,
            uiStore.calibrationPlacement,
            uiStore.calibrationClearView,
            uiStore.detectedTags,
            mousePosition.value,
        ],
        () => {
            dirty = true
        },
        { deep: true }
    )

    function animationLoop(timestamp: number): void {
        animationFrameId = requestAnimationFrame(animationLoop)

        const dt = lastFrameTime === 0 ? 16 : Math.min(timestamp - lastFrameTime, 50)
        lastFrameTime = timestamp

        const hasAnimatedTracks = robotStore.tracks.some(
            t => t.configured && t.corners && t.corners.length === 4
        )
        const hasPaths = robotStore.paths.length > 0

        if (!dirty && !hasAnimatedTracks && !hasPaths) return

        dirty = false
        animation.update(dt, robotStore.tracks)
        render()
    }

    function initialize(): void {
        canvas.value = canvasRef.value
        if (canvas.value) {
            context.value = canvas.value.getContext('2d')
            syncDimensions()
            render()
        }
    }

    function syncDimensions(): void {
        if (!canvas.value) return
        const rect = canvas.value.parentElement?.getBoundingClientRect()
        if (rect && rect.width > 0 && rect.height > 0) {
            const newWidth = Math.floor(rect.width)
            const newHeight = Math.floor(rect.height)
            if (canvas.value.width !== newWidth || canvas.value.height !== newHeight) {
                canvas.value.width = newWidth
                canvas.value.height = newHeight
                dimensions.value = { width: newWidth, height: newHeight }
            }
        }
    }

    function clear(): void {
        ctx.value?.clearRect(0, 0, dimensions.value.width, dimensions.value.height)
    }

    // Draw order matters (later draws sit on top of earlier ones).
    function render(): void {
        if (!ctx.value) return
        measureVideoScale()
        clear()
        if (uiStore.calibrationClearView) {
            // Placing calibration tags: show only the guide boxes and the tags themselves
            calibrationRenderer.renderCalibrationGuides()
            calibrationRenderer.renderCalibrationTag()
            return
        }
        // Obstacles are drawn by the backend on the video frame
        overlayRenderer.renderDestinationMarker()
        trackRenderer.renderFootprints()
        overlayRenderer.renderTempObstacles()
        overlayRenderer.renderDestinationCursor()
        overlayRenderer.renderPaths()
        overlayRenderer.renderDrawingBox()
        trackRenderer.renderTracks()
        calibrationRenderer.renderCalibrationTag()
        overlayRenderer.renderInvalidFlash()
    }

    let resizeObserver: ResizeObserver | null = null

    onMounted(() => {
        initialize()
        animationFrameId = requestAnimationFrame(animationLoop)

        resizeObserver = new ResizeObserver(() => {
            syncDimensions()
            measureVideoScale()
            dirty = true
        })

        const wrapper = canvas.value?.parentElement
        if (wrapper) {
            resizeObserver.observe(wrapper)
        }

        window.addEventListener('resize', measureVideoScale)
        window.addEventListener('load', measureVideoScale)
        canvas.value?.addEventListener('mousemove', onMouseMove)
        canvas.value?.addEventListener('click', onClick)
        window.addEventListener('keydown', onKeyDown)
    })

    onUnmounted(() => {
        if (animationFrameId !== null) {
            cancelAnimationFrame(animationFrameId)
            animationFrameId = null
        }
        resizeObserver?.disconnect()
        window.removeEventListener('resize', measureVideoScale)
        window.removeEventListener('load', measureVideoScale)
        canvas.value?.removeEventListener('mousemove', onMouseMove)
        canvas.value?.removeEventListener('click', onClick)
        window.removeEventListener('keydown', onKeyDown)
    })

    return {
        canvas,
        context,
        dimensions,
        ctx,
        initialize,
        syncDimensions,
        render,
        getCanvasPoint,
        canvasToNatural,
        naturalToCanvas,
        getVideoScale,
    }
}
