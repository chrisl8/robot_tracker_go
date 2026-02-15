import { ref, computed, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { getTrackColor } from '@/types/robot'
import { getVideoDimensions } from '@/utils/coordinates'

export interface CanvasPoint {
    x: number
    y: number
}

// Module-level videoScale that is shared across all usages
const videoScale = ref({ x: 1, y: 1, offsetX: 0, offsetY: 0 })

// Export canvasToNatural for use by other modules (e.g., robotStore)
// Uses the shared module-level videoScale ref
export function canvasToNaturalShared(canvasX: number, canvasY: number): { x: number; y: number } {
    const scale = videoScale.value
    if (scale.x === 0 || scale.y === 0) {
        return { x: Math.round(canvasX), y: Math.round(canvasY) }
    }
    return {
        x: Math.round((canvasX - scale.offsetX) / scale.x),
        y: Math.round((canvasY - scale.offsetY) / scale.y),
    }
}

export function useCanvas(canvasRef: Ref<HTMLCanvasElement | null>) {
    const robotStore = useRobotStore()
    const obstacleStore = useObstacleStore()
    const uiStore = useUIStore()

    const canvas = ref<HTMLCanvasElement | null>(null)
    const context = ref<CanvasRenderingContext2D | null>(null)
    const dimensions = ref({ width: 0, height: 0 })
    const mousePosition = ref<{ x: number; y: number } | null>(null)
    const flashInvalid = ref<{ x: number; y: number; active: boolean } | null>(null)

    // Export videoScale for use by other modules (e.g., robotStore)
    const getVideoScale = () => videoScale.value

    const ctx = computed(() => context.value)

    function canvasToNatural(canvasX: number, canvasY: number): { x: number; y: number } {
        const scale = videoScale.value
        if (scale.x === 0 || scale.y === 0) {
            return { x: Math.round(canvasX), y: Math.round(canvasY) }
        }
        return {
            x: Math.round((canvasX - scale.offsetX) / scale.x),
            y: Math.round((canvasY - scale.offsetY) / scale.y),
        }
    }

    function naturalToCanvas(naturalX: number, naturalY: number): { x: number; y: number } {
        const scale = videoScale.value
        if (scale.x === 0 || scale.y === 0) {
            return { x: naturalX, y: naturalY }
        }
        return {
            x: naturalX * scale.x + scale.offsetX,
            y: naturalY * scale.y + scale.offsetY,
        }
    }

    function updateVideoScale(): void {
        const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
        if (!video) return

        const naturalWidth =
            'videoWidth' in video
                ? (video as HTMLVideoElement).videoWidth
                : 'naturalWidth' in video
                  ? (video as HTMLImageElement).naturalWidth
                  : 0
        const naturalHeight =
            'videoHeight' in video
                ? (video as HTMLVideoElement).videoHeight
                : 'naturalHeight' in video
                  ? (video as HTMLImageElement).naturalHeight
                  : 0

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
                offsetY: videoRect.top - canvasRect.top,
            }
        } else {
            videoScale.value = { x: scaleX, y: scaleY, offsetX: 0, offsetY: 0 }
        }
    }

    // Watch for changes and redraw
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
            uiStore.selectedCalibrationTagId,
            uiStore.detectedTags,
            mousePosition.value,
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
        // Obstacles are now drawn by the backend on the video frame
        renderDestinationMarker()
        renderFootprints()
        renderDestinationCursor()
        renderPaths()
        renderDrawingBox()
        renderTracks()
        renderCalibrationTag()
        renderInvalidFlash()
    }

    // renderObstacles removed - obstacles are now drawn by the backend

    function renderPaths(): void {
        const pathMessages = robotStore.paths
        if (!ctx.value || !Array.isArray(pathMessages) || pathMessages.length === 0) return

        for (const path of pathMessages) {
            if (!path.points || path.points.length < 2) continue

            ctx.value.beginPath()
            ctx.value.strokeStyle = path.color || '#FFFF00'
            ctx.value.lineWidth = 2

            const first = naturalToCanvas(path.points[0][0], path.points[0][1])
            ctx.value.moveTo(first.x, first.y)

            for (let i = 1; i < path.points.length; i++) {
                const p = naturalToCanvas(path.points[i][0], path.points[i][1])
                ctx.value.lineTo(p.x, p.y)
            }
            ctx.value.stroke()

            // Draw waypoint dots
            ctx.value.fillStyle = path.color || '#FFFF00'
            for (let i = 0; i < path.points.length; i++) {
                const p = naturalToCanvas(path.points[i][0], path.points[i][1])
                ctx.value.beginPath()
                ctx.value.arc(p.x, p.y, 4, 0, Math.PI * 2)
                ctx.value.fill()
            }
        }
    }

    function renderFootprints(): void {
        const tracks = robotStore.confirmedTracks
        if (!ctx.value || !uiStore.showFootprints || tracks.length === 0) return

        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        const selectedTrackId = robotStore.selectedTrackId

        for (const track of tracks) {
            if (!track.pixel_radius || track.pixel_radius <= 0) continue

            const centerNatural = naturalToCanvas(
                (track.bbox[0] + track.bbox[2]) / 2,
                (track.bbox[1] + track.bbox[3]) / 2
            )
            const radius = track.pixel_radius * videoScale.value.x

            const isSelected = track.id === selectedTrackId

            // Draw filled circle
            ctx.value.beginPath()
            ctx.value.arc(centerNatural.x, centerNatural.y, radius, 0, Math.PI * 2)
            if (isSelected) {
                ctx.value.fillStyle = 'rgba(255, 255, 255, 0.3)'
            } else {
                ctx.value.fillStyle = 'rgba(0, 255, 255, 0.2)'
            }
            ctx.value.fill()

            // Draw solid edge
            ctx.value.lineWidth = isSelected ? 3 : 2
            ctx.value.strokeStyle = isSelected ? '#ffffff' : '#00ffff'
            ctx.value.stroke()
        }
    }

    // x, y, radius are all in NATURAL VIDEO coordinates (not canvas pixels)
    function isCircleInObstacle(cx: number, cy: number, radius: number): boolean {
        const obstacles = obstacleStore.obstacles
        for (const obs of obstacles) {
            const rectX1 = obs.pixel_top_left[0]
            const rectY1 = obs.pixel_top_left[1]
            const rectX2 = obs.pixel_bottom_right[0]
            const rectY2 = obs.pixel_bottom_right[1]

            // Find the closest point on the rectangle to the circle center
            const closestX = Math.max(rectX1, Math.min(cx, rectX2))
            const closestY = Math.max(rectY1, Math.min(cy, rectY2))

            // Calculate the distance from the closest point to the circle center
            const distanceX = cx - closestX
            const distanceY = cy - closestY
            const distanceSquared = distanceX * distanceX + distanceY * distanceY

            // If the distance is less than the circle radius, they intersect
            if (distanceSquared < radius * radius) {
                return true
            }

            // Also check if circle is completely inside rectangle
            if (
                cx >= rectX1 &&
                cx <= rectX2 &&
                cy >= rectY1 &&
                cy <= rectY2 &&
                cx - radius >= rectX1 &&
                cx + radius <= rectX2 &&
                cy - radius >= rectY1 &&
                cy + radius <= rectY2
            ) {
                return true
            }
        }
        return false
    }

    function renderDestinationCursor(): void {
        if (!ctx.value || !robotStore.destinationMode || !mousePosition.value) return

        const selectedTrack = robotStore.selectedTrack
        if (!selectedTrack || !selectedTrack.pixel_radius) return

        const pos = mousePosition.value

        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        // Convert canvas mouse position to natural video coordinates for collision check
        const naturalPos = canvasToNatural(pos.x, pos.y)
        const radius = selectedTrack.pixel_radius * videoScale.value.x
        const naturalRadius = selectedTrack.pixel_radius

        // Check collision using natural video coordinates
        const isInvalid = isCircleInObstacle(naturalPos.x, naturalPos.y, naturalRadius)

        // Draw filled circle at canvas coordinates
        ctx.value.beginPath()
        ctx.value.arc(pos.x, pos.y, radius, 0, Math.PI * 2)
        ctx.value.fillStyle = isInvalid ? 'rgba(255, 0, 0, 0.3)' : 'rgba(0, 255, 0, 0.3)'
        ctx.value.fill()

        // Draw solid edge
        ctx.value.strokeStyle = isInvalid ? '#ff0000' : '#00ff00'
        ctx.value.lineWidth = 2
        ctx.value.setLineDash([5, 5])
        ctx.value.stroke()
        ctx.value.setLineDash([])

        // Draw "DEST" label
        ctx.value.fillStyle = isInvalid ? '#ff0000' : '#00ff00'
        ctx.value.font = 'bold 12px sans-serif'
        ctx.value.fillText('DEST', pos.x + radius + 5, pos.y)
    }

    function triggerInvalidFlash(x: number, y: number): void {
        flashInvalid.value = { x, y, active: true }
        setTimeout(() => {
            flashInvalid.value = null
        }, 200)
    }

    function renderInvalidFlash(): void {
        if (!ctx.value || !flashInvalid.value || !flashInvalid.value.active) return

        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        const pos = flashInvalid.value
        const radius = 20

        ctx.value.beginPath()
        ctx.value.arc(pos.x, pos.y, radius * 1.2, 0, Math.PI * 2)
        ctx.value.strokeStyle = 'rgba(255, 0, 0, 0.8)'
        ctx.value.lineWidth = 4
        ctx.value.stroke()

        ctx.value.beginPath()
        ctx.value.arc(pos.x, pos.y, radius, 0, Math.PI * 2)
        ctx.value.strokeStyle = 'rgba(255, 0, 0, 0.4)'
        ctx.value.lineWidth = 2
        ctx.value.stroke()
    }

    function renderDestinationMarker(): void {
        const dest = robotStore.destination
        if (!ctx.value || !dest) return

        const selectedTrack = robotStore.confirmedTracks.find(t => t.tag_id === dest.robot_id)
        const selectedPixelRadius = selectedTrack?.pixel_radius

        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        const scaled = naturalToCanvas(dest.x, dest.y)
        const radius = (selectedPixelRadius || 20) * videoScale.value.x

        // Draw filled circle
        ctx.value.beginPath()
        ctx.value.arc(scaled.x, scaled.y, radius, 0, Math.PI * 2)
        ctx.value.fillStyle = 'rgba(147, 51, 234, 0.3)'
        ctx.value.fill()

        // Draw solid edge
        ctx.value.strokeStyle = '#9333ea'
        ctx.value.lineWidth = 3
        ctx.value.stroke()

        // Draw "GOAL" label
        ctx.value.fillStyle = '#9333ea'
        ctx.value.font = 'bold 12px sans-serif'
        ctx.value.fillText('GOAL', scaled.x + radius + 5, scaled.y)
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
        if (!ctx.value) return

        const detectedTags = uiStore.detectedTags
        const selectedTagId = uiStore.selectedCalibrationTagId
        const calibrationOpen = uiStore.panels.calibrationOpen

        if (!calibrationOpen || detectedTags.length === 0) return

        // Draw all detected tags with dim markers for spatial context
        for (const tag of detectedTags) {
            if (!tag.corners || tag.corners.length !== 4) continue
            if (tag.id === selectedTagId) continue // drawn separately below

            const scaledCorners = tag.corners.map((corner: [number, number]) =>
                naturalToCanvas(corner[0], corner[1])
            )

            ctx.value.strokeStyle = 'rgba(255, 255, 255, 0.5)'
            ctx.value.lineWidth = 1
            ctx.value.setLineDash([])
            ctx.value.beginPath()
            ctx.value.moveTo(scaledCorners[0].x, scaledCorners[0].y)
            ctx.value.lineTo(scaledCorners[1].x, scaledCorners[1].y)
            ctx.value.lineTo(scaledCorners[2].x, scaledCorners[2].y)
            ctx.value.lineTo(scaledCorners[3].x, scaledCorners[3].y)
            ctx.value.closePath()
            ctx.value.stroke()

            const cx = (scaledCorners[0].x + scaledCorners[2].x) / 2
            const cy = (scaledCorners[0].y + scaledCorners[2].y) / 2
            ctx.value.fillStyle = 'rgba(255, 255, 255, 0.7)'
            ctx.value.font = '11px sans-serif'
            ctx.value.textAlign = 'center'
            ctx.value.textBaseline = 'middle'
            ctx.value.fillText(`Tag ${tag.id}`, cx, cy)
        }

        // Draw selected tag with prominent highlight
        if (selectedTagId === null) {
            ctx.value.textAlign = 'left'
            ctx.value.textBaseline = 'alphabetic'
            return
        }

        const selectedTag = detectedTags.find(tag => tag.id === selectedTagId)
        if (!selectedTag || !selectedTag.corners || selectedTag.corners.length !== 4) {
            ctx.value.textAlign = 'left'
            ctx.value.textBaseline = 'alphabetic'
            return
        }

        const scaledCorners = selectedTag.corners.map((corner: [number, number]) =>
            naturalToCanvas(corner[0], corner[1])
        )

        // Glow effect
        ctx.value.save()
        ctx.value.shadowBlur = 15
        ctx.value.shadowColor = '#00bcd4'

        // Thick cyan outline
        ctx.value.strokeStyle = '#00bcd4'
        ctx.value.lineWidth = 4
        ctx.value.setLineDash([])
        ctx.value.beginPath()
        ctx.value.moveTo(scaledCorners[0].x, scaledCorners[0].y)
        ctx.value.lineTo(scaledCorners[1].x, scaledCorners[1].y)
        ctx.value.lineTo(scaledCorners[2].x, scaledCorners[2].y)
        ctx.value.lineTo(scaledCorners[3].x, scaledCorners[3].y)
        ctx.value.closePath()
        ctx.value.stroke()

        ctx.value.restore()

        // Stronger fill
        ctx.value.fillStyle = 'rgba(0, 188, 212, 0.35)'
        ctx.value.beginPath()
        ctx.value.moveTo(scaledCorners[0].x, scaledCorners[0].y)
        ctx.value.lineTo(scaledCorners[1].x, scaledCorners[1].y)
        ctx.value.lineTo(scaledCorners[2].x, scaledCorners[2].y)
        ctx.value.lineTo(scaledCorners[3].x, scaledCorners[3].y)
        ctx.value.closePath()
        ctx.value.fill()

        // Corner markers
        ctx.value.fillStyle = '#00bcd4'
        for (const corner of scaledCorners) {
            ctx.value.beginPath()
            ctx.value.arc(corner.x, corner.y, 5, 0, Math.PI * 2)
            ctx.value.fill()
        }

        // Larger label
        const centerX = (scaledCorners[0].x + scaledCorners[2].x) / 2
        const centerY = (scaledCorners[0].y + scaledCorners[2].y) / 2
        ctx.value.fillStyle = '#00bcd4'
        ctx.value.fillRect(centerX - 35, centerY - 27, 70, 24)

        ctx.value.fillStyle = '#1a1a2e'
        ctx.value.font = 'bold 14px sans-serif'
        ctx.value.textAlign = 'center'
        ctx.value.textBaseline = 'middle'
        ctx.value.fillText(`Tag ${selectedTagId}`, centerX, centerY - 15)
        ctx.value.textAlign = 'left'
        ctx.value.textBaseline = 'alphabetic'
    }

    function renderTracks(): void {
        const tracks = robotStore.tracks
        if (!ctx.value || tracks.length === 0) return

        const info = getVideoDimensions(dimensions.value.width, dimensions.value.height)
        if (info.naturalWidth === 0 || info.naturalHeight === 0) return

        for (const track of tracks) {
            const [x1, y1, x2, y2] = track.bbox
            const scaled1 = naturalToCanvas(x1, y1)
            const scaled2 = naturalToCanvas(x2, y2)
            const width = scaled2.x - scaled1.x
            const height = scaled2.y - scaled1.y
            const color = getTrackColor(track.id)

            // Draw rectangle
            ctx.value.strokeStyle = color
            ctx.value.lineWidth = 2
            ctx.value.strokeRect(scaled1.x, scaled1.y, width, height)

            // Draw label
            const label = track.tag_id !== undefined ? `Robot ${track.tag_id}` : `#${track.id}`
            ctx.value.font = 'bold 11px sans-serif'
            const labelWidth = ctx.value.measureText(label).width + 8
            ctx.value.fillStyle = color
            ctx.value.fillRect(scaled1.x, scaled1.y - 18, labelWidth, 18)
            ctx.value.fillStyle = '#1a1a2e'
            ctx.value.fillText(label, scaled1.x + 4, scaled1.y - 5)

            // Draw confidence if available
            if (track.confidence > 0) {
                ctx.value.fillStyle = color
                ctx.value.font = '10px sans-serif'
                ctx.value.fillText(
                    `${(track.confidence * 100).toFixed(0)}%`,
                    scaled1.x,
                    scaled2.y + 14
                )
            }

            // Draw heading arrow from AprilTag corners in pixel space
            if (track.corners && track.corners.length === 4) {
                const botMidX = (track.corners[2][0] + track.corners[3][0]) / 2
                const botMidY = (track.corners[2][1] + track.corners[3][1]) / 2
                const topMidX = (track.corners[0][0] + track.corners[1][0]) / 2
                const topMidY = (track.corners[0][1] + track.corners[1][1]) / 2
                const pixelHeading =
                    Math.atan2(topMidY - botMidY, topMidX - botMidX) + (track.heading_offset || 0)

                const centerX = (scaled1.x + scaled2.x) / 2
                const centerY = (scaled1.y + scaled2.y) / 2
                const arrowLen = 30
                const tipX = centerX + arrowLen * Math.cos(pixelHeading)
                const tipY = centerY + arrowLen * Math.sin(pixelHeading)

                // Draw arrow line
                ctx.value.beginPath()
                ctx.value.strokeStyle = '#00ff00'
                ctx.value.lineWidth = 2
                ctx.value.moveTo(centerX, centerY)
                ctx.value.lineTo(tipX, tipY)
                ctx.value.stroke()

                // Draw arrowhead
                const headLen = 8
                const angle = Math.atan2(tipY - centerY, tipX - centerX)
                ctx.value.beginPath()
                ctx.value.fillStyle = '#00ff00'
                ctx.value.moveTo(tipX, tipY)
                ctx.value.lineTo(
                    tipX - headLen * Math.cos(angle - Math.PI / 6),
                    tipY - headLen * Math.sin(angle - Math.PI / 6)
                )
                ctx.value.lineTo(
                    tipX - headLen * Math.cos(angle + Math.PI / 6),
                    tipY - headLen * Math.sin(angle + Math.PI / 6)
                )
                ctx.value.closePath()
                ctx.value.fill()
            }
        }
    }

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
                if (isCircleInObstacle(naturalPos.x, naturalPos.y, pixelRadius)) {
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
        }
    }

    function onKeyDown(event: KeyboardEvent): void {
        if (event.key === 'Escape') {
            robotStore.cancelDestinationMode()
            mousePosition.value = null
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
        canvas.value?.addEventListener('mousemove', onMouseMove)
        canvas.value?.addEventListener('click', onClick)
        window.addEventListener('keydown', onKeyDown)
    })

    onUnmounted(() => {
        resizeObserver?.disconnect()
        window.removeEventListener('resize', updateVideoScale)
        window.removeEventListener('load', updateVideoScale)
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
