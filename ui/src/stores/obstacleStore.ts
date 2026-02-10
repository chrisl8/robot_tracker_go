import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Obstacle } from '@/types/api'
import type { DrawState, Point, DrawRect } from '@/types/obstacle'

export const useObstacleStore = defineStore('obstacle', () => {
    // State
    const obstacles = ref<Obstacle[]>([])
    const drawing = ref<DrawState>({
        active: false,
        startPoint: null,
        currentPoint: null,
        startTime: null,
    })
    const isSaved = ref(true)
    const drawingMode = ref(false)

    // Computed
    const obstacleCount = computed(() => obstacles.value.length)

    const drawRect = computed((): DrawRect | null => {
        const { startPoint, currentPoint } = drawing.value
        if (!startPoint || !currentPoint) return null
        return {
            x1: Math.min(startPoint.x, currentPoint.x),
            y1: Math.min(startPoint.y, currentPoint.y),
            x2: Math.max(startPoint.x, currentPoint.x),
            y2: Math.max(startPoint.y, currentPoint.y),
        }
    })

    // Actions
    function setObstacles(newObstacles: Obstacle[]): void {
        obstacles.value = newObstacles
    }

    function addObstacle(obstacle: Obstacle): void {
        obstacles.value.push(obstacle)
        isSaved.value = false
    }

    function removeObstacle(id: string): void {
        const idx = obstacles.value.findIndex(o => o.id === id)
        if (idx >= 0) {
            obstacles.value.splice(idx, 1)
            isSaved.value = false
        }
    }

    function clearObstacles(): void {
        obstacles.value = []
        isSaved.value = false
    }

    function setSaved(value: boolean): void {
        isSaved.value = value
    }

    // Drawing mode actions
    function toggleDrawingMode(): void {
        drawingMode.value = !drawingMode.value
        if (!drawingMode.value) {
            cancelDrawing()
        }
    }

    function setDrawingMode(value: boolean): void {
        drawingMode.value = value
        if (!value) {
            cancelDrawing()
        }
    }

    // Drawing actions
    function startDrawing(point: Point): void {
        if (!drawingMode.value) return
        drawing.value = {
            active: true,
            startPoint: point,
            currentPoint: point,
            startTime: Date.now(),
        }
    }

    function updateDrawing(point: Point): void {
        if (!drawing.value.active) return
        drawing.value.currentPoint = point
    }

    function cancelDrawing(): void {
        drawing.value = {
            active: false,
            startPoint: null,
            currentPoint: null,
            startTime: null,
        }
    }

    function finishDrawing(): Point | null {
        const { startPoint } = drawing.value
        const result = startPoint
        cancelDrawing()
        return result
    }

    return {
        // State
        obstacles,
        drawing,
        isSaved,
        drawingMode,
        // Computed
        obstacleCount,
        drawRect,
        // Actions
        setObstacles,
        addObstacle,
        removeObstacle,
        clearObstacles,
        setSaved,
        // Drawing mode actions
        toggleDrawingMode,
        setDrawingMode,
        // Drawing actions
        startDrawing,
        updateDrawing,
        cancelDrawing,
        finishDrawing,
    }
})
