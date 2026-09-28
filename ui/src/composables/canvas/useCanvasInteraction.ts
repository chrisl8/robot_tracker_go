import type { Ref } from 'vue'
import type { useRobotStore } from '@/stores/robotStore'
import type { useObstacleStore } from '@/stores/obstacleStore'
import type { useTempObstacleStore } from '@/stores/tempObstacleStore'
import type { useUIStore } from '@/stores/uiStore'
import { createRateLimiter } from '@/utils/rateLimiter'
import { hitTestTempObstacle } from '@/utils/tempObstacles'
import { canvasToNaturalShared as canvasToNatural, naturalToCanvas } from '@/utils/canvasTransform'
import { isCircleInObstacle } from '@/utils/obstacleCollision'

export interface CanvasPoint {
    x: number
    y: number
}

export interface InteractionDeps {
    canvas: Ref<HTMLCanvasElement | null>
    mousePosition: Ref<{ x: number; y: number } | null>
    robotStore: ReturnType<typeof useRobotStore>
    obstacleStore: ReturnType<typeof useObstacleStore>
    tempObstacleStore: ReturnType<typeof useTempObstacleStore>
    uiStore: ReturnType<typeof useUIStore>
    triggerInvalidFlash: (x: number, y: number) => void
}

// Mouse/keyboard handling for the overlay canvas: pointer -> canvas coordinates,
// track hit-testing, destination placement (with obstacle validation), temporary
// obstacle absorb prompts, and Escape to cancel destination mode. Attaching and
// removing the DOM listeners is left to useCanvas's lifecycle.
export function createCanvasInteraction({
    canvas,
    mousePosition,
    robotStore,
    obstacleStore,
    tempObstacleStore,
    uiStore,
    triggerInvalidFlash,
}: InteractionDeps) {
    // A robot is selected but the click landed on empty video and did nothing: say why, sparingly.
    const canShowSelectionHint = createRateLimiter(8000)

    function getCanvasPoint(event: MouseEvent): CanvasPoint | null {
        if (!canvas.value) return null
        const rect = canvas.value.getBoundingClientRect()
        return {
            x: (event.clientX - rect.left) * (canvas.value.width / rect.width),
            y: (event.clientY - rect.top) * (canvas.value.height / rect.height),
        }
    }

    function onMouseMove(event: MouseEvent): void {
        const point = getCanvasPoint(event)
        if (point) {
            mousePosition.value = point
        }
    }

    function onClick(event: MouseEvent): void {
        const point = getCanvasPoint(event)
        if (!point) return

        if (robotStore.destinationMode && robotStore.selectedTrackId !== null) {
            const tracks = robotStore.confirmedTracks
            for (const track of tracks) {
                const [x1, y1, x2, y2] = track.bbox
                const scaled1 = naturalToCanvas(x1, y1)
                const scaled2 = naturalToCanvas(x2, y2)

                if (
                    point.x >= scaled1.x &&
                    point.x <= scaled2.x &&
                    point.y >= scaled1.y &&
                    point.y <= scaled2.y
                ) {
                    if (track.id === robotStore.selectedTrackId) {
                        robotStore.clearSelection()
                    } else {
                        robotStore.selectTrack(track.id)
                    }
                    return
                }
            }

            const naturalPos = canvasToNatural(point.x, point.y)
            const selectedTrack = robotStore.confirmedTracks.find(
                t => t.id === robotStore.selectedTrackId
            )
            const pixelRadius = selectedTrack?.pixel_radius

            if (pixelRadius && pixelRadius > 0) {
                if (
                    isCircleInObstacle(
                        obstacleStore.obstacles,
                        naturalPos.x,
                        naturalPos.y,
                        pixelRadius
                    )
                ) {
                    triggerInvalidFlash(point.x, point.y)
                    uiStore.showToast('Destination overlaps with obstacle', 'error')
                    return
                }
            }

            robotStore.confirmDestination(point.x, point.y)
        } else {
            const tracks = robotStore.confirmedTracks
            for (const track of tracks) {
                const [x1, y1, x2, y2] = track.bbox
                const scaled1 = naturalToCanvas(x1, y1)
                const scaled2 = naturalToCanvas(x2, y2)

                if (
                    point.x >= scaled1.x &&
                    point.x <= scaled2.x &&
                    point.y >= scaled1.y &&
                    point.y <= scaled2.y
                ) {
                    robotStore.selectTrack(track.id)
                    return
                }
            }

            // A click on a temporary obstacle offers to absorb it (treat as floor).
            if (tempObstacleStore.enabled) {
                const natural = canvasToNatural(point.x, point.y)
                const hit = hitTestTempObstacle(tempObstacleStore.obstacles, natural.x, natural.y)
                if (hit) {
                    tempObstacleStore.promptAbsorb({
                        x: natural.x,
                        y: natural.y,
                        canvasX: point.x,
                        canvasY: point.y,
                    })
                    return
                }
            }

            // Stray clicks must never dispatch the robot (select, then click), so only explain.
            if (robotStore.selectedTrackId !== null && canShowSelectionHint()) {
                uiStore.showToast('Click the robot again, then click where it should go', 'info')
            }
        }
    }

    function onKeyDown(event: KeyboardEvent): void {
        if (event.key === 'Escape') {
            robotStore.cancelDestinationMode()
            mousePosition.value = null
        }
    }

    return { getCanvasPoint, onMouseMove, onClick, onKeyDown }
}
