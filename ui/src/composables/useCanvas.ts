import { ref, computed, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { getTrackColor } from '@/types/robot'

export interface CanvasPoint {
    x: number
    y: number
}

export function useCanvas(canvasRef: Ref<HTMLCanvasElement | null>) {
    const robotStore = useRobotStore()
    const obstacleStore = useObstacleStore()
    const uiStore = useUIStore()

    const canvas = ref<HTMLCanvasElement | null>(null)
    const context = ref<CanvasRenderingContext2D | null>(null)
    const dimensions = ref({ width: 0, height: 0 })
    const videoScale = ref({ x: 1, y: 1, offsetX: 0, offsetY: 0 })

    const ctx = computed(() => context.value)

    function updateVideoScale(): void {
        const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
        if (!video) return

        const naturalWidth = 'videoWidth' in video ? (video as HTMLVideoElement).videoWidth : ('naturalWidth' in video ? (video as HTMLImageElement).naturalWidth : 0)
        const naturalHeight = 'videoHeight' in video ? (video as HTMLVideoElement).videoHeight : ('naturalHeight' in video ? (video as HTMLImageElement).naturalHeight : 0)

        if (naturalWidth === 0 || naturalHeight === 0) return

        const displayedWidth = video.clientWidth
        const displayedHeight = video.clientHeight

        const scaleX = displayedWidth / naturalWidth
        const scaleY = displayedHeight / naturalHeight

        const videoRect = video.getBoundingClientRect()
        const canvasEl = canvas.value
        if (canvasEl) {
            const canvasRect = canvasEl.getBoundingClientRect()
            videoScale.value = {
                x: scaleX,
                y: scaleY,
                offsetX: videoRect.left - canvasRect.left,
                offsetY: videoRect.top - canvasRect.top
            }
        } else {
            videoScale.value = { x: scaleX, y: scaleY, offsetX: 0, offsetY: 0 }
        }
    }

    // Watch for changes and redraw
    watch(
        () => [
            robotStore.tracks,
            obstacleStore.obstacles,
            obstacleStore.drawRect,
            uiStore.selectedCalibrationTagId,
            uiStore.detectedTags
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
        updateVideoScale()
        clear()
        renderObstacles()
        renderFootprints()
        renderDrawingBox()
        renderTracks()
        renderCalibrationTag()
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

    function renderFootprints(): void {
        const tracks = robotStore.confirmedTracks
        if (!ctx.value || !uiStore.showFootprints || tracks.length === 0) return

        // Get video dimensions for scaling
        const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
        if (!video) return

        const naturalWidth = 'videoWidth' in video ? (video as HTMLVideoElement).videoWidth : ('naturalWidth' in video ? (video as HTMLImageElement).naturalWidth : 0)
        const naturalHeight = 'videoHeight' in video ? (video as HTMLVideoElement).videoHeight : ('naturalHeight' in video ? (video as HTMLImageElement).naturalHeight : 0)

        if (naturalWidth === 0 || naturalHeight === 0) return

        const scaleX = dimensions.value.width / naturalWidth
        const scaleY = dimensions.value.height / naturalHeight

        for (const track of tracks) {
            if (!track.pixel_radius || track.pixel_radius <= 0) continue

            const centerX = ((track.bbox[0] + track.bbox[2]) / 2) * scaleX
            const centerY = ((track.bbox[1] + track.bbox[3]) / 2) * scaleY
            const radius = track.pixel_radius * Math.min(scaleX, scaleY)

            // Draw filled circle
            ctx.value.beginPath()
            ctx.value.arc(centerX, centerY, radius, 0, Math.PI * 2)
            ctx.value.fillStyle = 'rgba(0, 255, 255, 0.2)'
            ctx.value.fill()

            // Draw solid edge
            ctx.value.strokeStyle = '#00ffff'
            ctx.value.lineWidth = 2
            ctx.value.stroke()
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

    function renderCalibrationTag(): void {
        const selectedTagId = uiStore.selectedCalibrationTagId
        const detectedTags = uiStore.detectedTags

        if (!ctx.value || selectedTagId === null || detectedTags.length === 0) return

        const selectedTag = detectedTags.find((tag) => tag.id === selectedTagId)
        if (!selectedTag || !selectedTag.corners || selectedTag.corners.length !== 4) return

        const corners = selectedTag.corners

        const scale = videoScale.value
        const scaledCorners = corners.map((corner: [number, number]) => [
            corner[0] * scale.x + scale.offsetX,
            corner[1] * scale.y + scale.offsetY
        ])

        ctx.value.strokeStyle = '#00bcd4'
        ctx.value.lineWidth = 3
        ctx.value.setLineDash([])
        ctx.value.beginPath()
        ctx.value.moveTo(scaledCorners[0][0], scaledCorners[0][1])
        ctx.value.lineTo(scaledCorners[1][0], scaledCorners[1][1])
        ctx.value.lineTo(scaledCorners[2][0], scaledCorners[2][1])
        ctx.value.lineTo(scaledCorners[3][0], scaledCorners[3][1])
        ctx.value.closePath()
        ctx.value.stroke()

        ctx.value.fillStyle = 'rgba(0, 188, 212, 0.2)'
        ctx.value.fill()

        const centerX = (scaledCorners[0][0] + scaledCorners[2][0]) / 2
        const centerY = (scaledCorners[0][1] + scaledCorners[2][1]) / 2
        ctx.value.fillStyle = '#00bcd4'
        ctx.value.fillRect(centerX - 30, centerY - 25, 60, 20)

        ctx.value.fillStyle = '#1a1a2e'
        ctx.value.font = 'bold 12px sans-serif'
        ctx.value.textAlign = 'center'
        ctx.value.textBaseline = 'middle'
        ctx.value.fillText(`Tag ${selectedTagId}`, centerX, centerY - 15)
        ctx.value.textAlign = 'left'
        ctx.value.textBaseline = 'alphabetic'
    }

    function renderTracks(): void {
        const tracks = robotStore.tracks
        if (!ctx.value || tracks.length === 0) return

        // Get video dimensions for scaling
        const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
        if (!video) return

        const naturalWidth = 'videoWidth' in video ? (video as HTMLVideoElement).videoWidth : ('naturalWidth' in video ? (video as HTMLImageElement).naturalWidth : 0)
        const naturalHeight = 'videoHeight' in video ? (video as HTMLVideoElement).videoHeight : ('naturalHeight' in video ? (video as HTMLImageElement).naturalHeight : 0)

        if (naturalWidth === 0 || naturalHeight === 0) return

        const scaleX = dimensions.value.width / naturalWidth
        const scaleY = dimensions.value.height / naturalHeight

        for (const track of tracks) {
            const [x1, y1, x2, y2] = track.bbox
            const scaledX1 = x1 * scaleX
            const scaledY1 = y1 * scaleY
            const scaledX2 = x2 * scaleX
            const scaledY2 = y2 * scaleY
            const width = scaledX2 - scaledX1
            const height = scaledY2 - scaledY1
            const color = getTrackColor(track.id)

            // Draw rectangle
            ctx.value.strokeStyle = color
            ctx.value.lineWidth = 2
            ctx.value.strokeRect(scaledX1, scaledY1, width, height)

            // Draw label background
            ctx.value.fillStyle = color
            ctx.value.fillRect(scaledX1, scaledY1 - 18, 50, 18)

            // Draw label text
            ctx.value.fillStyle = '#1a1a2e'
            ctx.value.font = 'bold 11px sans-serif'
            ctx.value.fillText(`#${track.id}`, scaledX1 + 4, scaledY1 - 5)

            // Draw confidence if available
            if (track.confidence > 0) {
                ctx.value.fillStyle = color
                ctx.value.font = '10px sans-serif'
                ctx.value.fillText(`${(track.confidence * 100).toFixed(0)}%`, scaledX1, scaledY2 + 14)
            }

            // Draw tag ID if available
            if (track.tag_id !== undefined) {
                ctx.value.fillStyle = '#4ecca3'
                ctx.value.fillRect(scaledX2 - 25, scaledY2 - 5, 25, 18)
                ctx.value.fillStyle = '#1a1a2e'
                ctx.value.font = 'bold 10px sans-serif'
                ctx.value.fillText(`T${track.tag_id}`, scaledX2 - 22, scaledY2 + 8)
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

        resizeObserver = new ResizeObserver(() => {
            syncDimensions()
            updateVideoScale()
            render()
        })

        const wrapper = canvas.value?.parentElement
        if (wrapper) {
            resizeObserver.observe(wrapper)
        }

        window.addEventListener('resize', updateVideoScale)
        window.addEventListener('load', updateVideoScale)
    })

    onUnmounted(() => {
        resizeObserver?.disconnect()
        window.removeEventListener('resize', updateVideoScale)
        window.removeEventListener('load', updateVideoScale)
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
