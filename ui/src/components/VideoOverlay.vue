<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { useCanvas, type CanvasPoint } from '@/composables/useCanvas'

const obstacleStore = useObstacleStore()
const uiStore = useUIStore()

const overlayRef = ref<HTMLCanvasElement | null>(null)
const { render, getCanvasPoint } = useCanvas(overlayRef)

const drawingMode = computed(() => obstacleStore.drawingMode)

function handleMouseDown(event: MouseEvent): void {
    if (!uiStore.panels.obstacleOpen || !drawingMode.value) return

    const point = getCanvasPoint(event)
    if (point) {
        obstacleStore.startDrawing(point)
    }
}

function handleMouseMove(event: MouseEvent): void {
    if (!obstacleStore.drawing.active) return

    const point = getCanvasPoint(event)
    if (point) {
        obstacleStore.updateDrawing(point)
    }
}

function handleMouseUp(event: MouseEvent): void {
    if (!obstacleStore.drawing.active) return

    const point = getCanvasPoint(event)
    if (point) {
        const startPoint = obstacleStore.drawing.startPoint
        if (startPoint) {
            const { topLeft, bottomRight } = calculateObstacleFromPoints(startPoint, point)
            const width = bottomRight[0] - topLeft[0]
            const height = bottomRight[1] - topLeft[1]

            if (width > 10 && height > 10) {
                addObstacle(topLeft, bottomRight)
            }
        }
    }

    obstacleStore.cancelDrawing()
}

async function addObstacle(topLeft: [number, number], bottomRight: [number, number]): Promise<void> {
    const name = `obstacle_${obstacleStore.obstacleCount + 1}`

    try {
        const response = await fetch('/api/obstacles', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                pixel_top_left: topLeft,
                pixel_bottom_right: bottomRight,
                name,
                clearance: 0.02
            })
        })

        if (!response.ok) {
            throw new Error('Failed to add obstacle')
        }

        // Obstacles will be updated via WebSocket broadcast
        uiStore.showToast('Obstacle added', 'success')
    } catch (e) {
        console.error('Failed to add obstacle:', e)
        uiStore.showToast('Failed to add obstacle', 'error')
    }
}

function calculateObstacleFromPoints(
    start: CanvasPoint,
    end: CanvasPoint
): { topLeft: [number, number]; bottomRight: [number, number] } {
    return {
        topLeft: [
            Math.round(Math.min(start.x, end.x)),
            Math.round(Math.min(start.y, end.y))
        ],
        bottomRight: [
            Math.round(Math.max(start.x, end.x)),
            Math.round(Math.max(start.y, end.y))
        ]
    }
}

function handleMouseLeave(): void {
    if (obstacleStore.drawing.active) {
        obstacleStore.cancelDrawing()
    }
}

// Handle video stream events
function handleVideoLoad(): void {
    render()
}

onMounted(() => {
    const video = document.getElementById('video') as HTMLImageElement
    if (video) {
        video.addEventListener('loadeddata', handleVideoLoad)
    }
})
</script>

<template>
    <canvas
        ref="overlayRef"
        id="overlay"
        :style="{ cursor: drawingMode ? 'crosshair' : 'default' }"
        @mousedown="handleMouseDown"
        @mousemove="handleMouseMove"
        @mouseup="handleMouseUp"
        @mouseleave="handleMouseLeave"
    ></canvas>
</template>

<style scoped>
#overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: auto;
}
</style>
