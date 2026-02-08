import { ref, computed, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { getTrackColor } from '@/types/robot'

export interface CanvasPoint {
    x: number
    y: number
}

export function useCanvas(canvasRef: Ref<HTMLCanvasElement | null>) {
    const robotStore = useRobotStore()
    const obstacleStore = useObstacleStore()

    const canvas = ref<HTMLCanvasElement | null>(null)
    const context = ref<CanvasRenderingContext2D | null>(null)
    const dimensions = ref({ width: 0, height: 0 })

    const ctx = computed(() => context.value)

    // Watch for changes and redraw
    watch(
        () => [
            robotStore.tracks,
            obstacleStore.obstacles,
            obstacleStore.drawRect
        ],
        () => {
            render()
        },
        { deep: true }
    )

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

    function render(): void {
        if (!ctx.value) return
        clear()
        renderObstacles()
        renderDrawingBox()
        renderTracks()
    }

    function renderObstacles(): void {
        const obs = obstacleStore.obstacles
        if (!ctx.value || obs.length === 0) return

        for (const obstacle of obs) {
            const [x1, y1] = obstacle.pixel_top_left
            const [x2, y2] = obstacle.pixel_bottom_right
            const width = x2 - x1
            const height = y2 - y1

            // Draw dashed rectangle
            ctx.value.strokeStyle = '#ff6b6b'
            ctx.value.lineWidth = 2
            ctx.value.setLineDash([5, 5])
            ctx.value.strokeRect(x1, y1, width, height)
            ctx.value.setLineDash([])

            // Draw label
            ctx.value.fillStyle = '#ff6b6b'
            ctx.value.font = 'bold 11px sans-serif'
            ctx.value.fillText('OBSTACLE', x1, y1 - 5)

            // Draw fill
            ctx.value.fillStyle = 'rgba(255, 107, 107, 0.1)'
            ctx.value.fillRect(x1, y1, width, height)
        }
    }

    function renderDrawingBox(): void {
        const drawRect = obstacleStore.drawRect
        if (!ctx.value || !drawRect) return

        const width = drawRect.x2 - drawRect.x1
        const height = drawRect.y2 - drawRect.y1

        // Draw dashed rectangle
        ctx.value.strokeStyle = '#ffa500'
        ctx.value.lineWidth = 2
        ctx.value.setLineDash([5, 5])
        ctx.value.strokeRect(drawRect.x1, drawRect.y1, width, height)
        ctx.value.setLineDash([])

        // Draw fill
        ctx.value.fillStyle = 'rgba(255, 165, 0, 0.2)'
        ctx.value.fillRect(drawRect.x1, drawRect.y1, width, height)
    }

    function renderTracks(): void {
        const tracks = robotStore.tracks
        if (!ctx.value || tracks.length === 0) return

        for (const track of tracks) {
            const [x1, y1, x2, y2] = track.bbox
            const width = x2 - x1
            const height = y2 - y1
            const color = getTrackColor(track.id)

            // Draw rectangle
            ctx.value.strokeStyle = color
            ctx.value.lineWidth = 2
            ctx.value.strokeRect(x1, y1, width, height)

            // Draw label background
            ctx.value.fillStyle = color
            ctx.value.fillRect(x1, y1 - 18, 50, 18)

            // Draw label text
            ctx.value.fillStyle = '#1a1a2e'
            ctx.value.font = 'bold 11px sans-serif'
            ctx.value.fillText(`#${track.id}`, x1 + 4, y1 - 5)

            // Draw confidence if available
            if (track.confidence > 0) {
                ctx.value.fillStyle = color
                ctx.value.font = '10px sans-serif'
                ctx.value.fillText(`${(track.confidence * 100).toFixed(0)}%`, x1, y2 + 14)
            }

            // Draw tag ID if available
            if (track.tag_id !== undefined) {
                ctx.value.fillStyle = '#4ecca3'
                ctx.value.fillRect(x2 - 25, y2 - 5, 25, 18)
                ctx.value.fillStyle = '#1a1a2e'
                ctx.value.font = 'bold 10px sans-serif'
                ctx.value.fillText(`T${track.tag_id}`, x2 - 22, y2 + 8)
            }
        }
    }

    function getCanvasPoint(event: MouseEvent): CanvasPoint | null {
        if (!canvas.value) return null
        const rect = canvas.value.getBoundingClientRect()
        return {
            x: (event.clientX - rect.left) * (canvas.value.width / rect.width),
            y: (event.clientY - rect.top) * (canvas.value.height / rect.height)
        }
    }

    let resizeObserver: ResizeObserver | null = null

    onMounted(() => {
        initialize()

        // Set up resize observer
        resizeObserver = new ResizeObserver(() => {
            syncDimensions()
            render()
        })

        const wrapper = canvas.value?.parentElement
        if (wrapper) {
            resizeObserver.observe(wrapper)
        }
    })

    onUnmounted(() => {
        resizeObserver?.disconnect()
    })

    return {
        canvas,
        context,
        dimensions,
        ctx,
        initialize,
        syncDimensions,
        render,
        getCanvasPoint
    }
}
