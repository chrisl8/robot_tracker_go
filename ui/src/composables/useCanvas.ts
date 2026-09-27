import { ref, computed, onMounted, onUnmounted, watch, type Ref } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useTempObstacleStore } from '@/stores/tempObstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { getTrackColor } from '@/types/robot'
import type { Track } from '@/types/api'
import { getVideoDimensions } from '@/utils/coordinates'
import { createRateLimiter } from '@/utils/rateLimiter'
import {
    hitTestTempObstacle,
    tempObstacleCanvasRect,
    tempObstacleLabel,
    tempObstacleStatusChip,
} from '@/utils/tempObstacles'

export interface CanvasPoint {
    x: number
    y: number
}

// Canvas color/font theme constants — matches CSS custom properties
const THEME = {
    // Footprint colors
    footprintFill: 'rgba(0, 217, 255, 0.15)',
    footprintStroke: '#00d9ff',
    footprintSelectedFill: 'rgba(224, 230, 237, 0.25)',
    footprintSelectedStroke: '#e0e6ed',

    // Temporary obstacles (foreground detector)
    tempFill: 'rgba(255, 145, 0, 0.28)',
    tempStroke: '#ff9100',
    tempIdleFill: 'rgba(255, 145, 0, 0.08)',
    tempIdleStroke: 'rgba(255, 145, 0, 0.7)',
    tempChipBg: 'rgba(10, 14, 20, 0.85)',
    tempChipText: '#ffab00',

    // Destination cursor
    destValidFill: 'rgba(0, 230, 118, 0.3)',
    destValidStroke: '#00e676',
    destInvalidFill: 'rgba(255, 61, 0, 0.3)',
    destInvalidStroke: '#ff3d00',

    // Destination marker (goal)
    goalFill: 'rgba(0, 145, 234, 0.3)',
    goalStroke: '#0091ea',

    // Drawing box (obstacle drawing)
    drawingStroke: '#ffab00',
    drawingFill: 'rgba(255, 171, 0, 0.2)',

    // Movement indicator
    headingChevron: '#00d9ff',
    headingChevronGlow: 'rgba(0, 217, 255, 0.4)',
    thrustForward: '#00d9ff',
    thrustReverse: '#ffab00',
    rotationArc: '#00d9ff',

    // Calibration tag
    calibTagStroke: 'rgba(224, 230, 237, 0.5)',
    calibTagLabel: 'rgba(224, 230, 237, 0.7)',
    calibGuideStroke: 'rgba(255, 255, 255, 0.8)',
    calibOkStroke: '#00e676',
    calibOkFill: 'rgba(0, 230, 118, 0.25)',
    calibWarnStroke: '#ffab00',
    calibWarnFill: 'rgba(255, 171, 0, 0.25)',
    calibBlockStroke: '#ff3d00',
    calibBlockFill: 'rgba(255, 61, 0, 0.25)',

    // Invalid flash
    flashOuter: 'rgba(255, 61, 0, 0.8)',
    flashInner: 'rgba(255, 61, 0, 0.4)',

    // Label backgrounds / text
    labelBg: '#0a0e14',
    textPrimary: '#e0e6ed',

    // Fonts
    fontLabel: "bold 11px 'Inter', sans-serif",
    fontSmall: "10px 'JetBrains Mono', monospace",
    fontDest: "bold 12px 'Inter', sans-serif",
    fontCalibSmall: "11px 'Inter', sans-serif",
    fontCalibLarge: "bold 14px 'Inter', sans-serif",
} as const

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
    const tempObstacleStore = useTempObstacleStore()
    const uiStore = useUIStore()
    // A robot is selected but the click landed on empty video and did nothing: say why, sparingly.
    const canShowSelectionHint = createRateLimiter(8000)

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

    // Animation state for movement indicators, keyed by track ID
    interface Particle {
        x: number
        y: number
        vx: number
        vy: number
        age: number
        maxAge: number
    }
    interface TrackAnimState {
        particles: Particle[]
        phase: number
    }
    const animationState = new Map<number, TrackAnimState>()
    let animationFrameId: number | null = null
    let lastFrameTime = 0
    let dirty = true
    let pathPhase = 0

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
        pathPhase += dt / 1000
        updateAnimationState(dt)
        render()
    }

    function computePixelHeading(track: Track): number | null {
        if (!track.corners || track.corners.length !== 4) return null
        const botMidX = (track.corners[2][0] + track.corners[3][0]) / 2
        const botMidY = (track.corners[2][1] + track.corners[3][1]) / 2
        const topMidX = (track.corners[0][0] + track.corners[1][0]) / 2
        const topMidY = (track.corners[0][1] + track.corners[1][1]) / 2
        return Math.atan2(topMidY - botMidY, topMidX - botMidX) + (track.heading_offset || 0)
    }

    function updateAnimationState(dt: number): void {
        const tracks = robotStore.tracks
        const activeTrackIds = new Set<number>()

        for (const track of tracks) {
            if (!track.configured || !track.corners || track.corners.length !== 4) continue
            activeTrackIds.add(track.id)

            let state = animationState.get(track.id)
            if (!state) {
                state = { particles: [], phase: 0 }
                animationState.set(track.id, state)
            }

            state.phase += dt / 1000

            // Update existing particles
            for (let i = state.particles.length - 1; i >= 0; i--) {
                const p = state.particles[i]
                p.age += dt
                p.x += p.vx * (dt / 1000)
                p.y += p.vy * (dt / 1000)
                if (p.age >= p.maxAge) {
                    state.particles.splice(i, 1)
                }
            }

            // Spawn new particles for forward/backward motion
            const motionState = track.motion_state || 'stopped'
            if (motionState === 'forward' || motionState === 'backward') {
                const heading = computePixelHeading(track)
                if (heading === null) continue

                const centerX = (track.bbox[0] + track.bbox[2]) / 2
                const centerY = (track.bbox[1] + track.bbox[3]) / 2
                const dir = motionState === 'backward' ? heading + Math.PI : heading
                const speed = 80

                state.particles.push({
                    x: centerX,
                    y: centerY,
                    vx: Math.cos(dir) * speed,
                    vy: Math.sin(dir) * speed,
                    age: 0,
                    maxAge: 500,
                })

                if (state.particles.length > 20) {
                    state.particles.shift()
                }
            } else {
                // Clear particles when not moving linearly
                state.particles.length = 0
            }
        }

        // Clean up state for removed tracks
        for (const id of animationState.keys()) {
            if (!activeTrackIds.has(id)) {
                animationState.delete(id)
            }
        }
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

    function render(): void {
        if (!ctx.value) return
        updateVideoScale()
        clear()
        if (uiStore.calibrationClearView) {
            // Placing calibration tags: show only the guide boxes and the tags themselves
            renderCalibrationGuides()
            renderCalibrationTag()
            return
        }
        // Obstacles are now drawn by the backend on the video frame
        renderDestinationMarker()
        renderFootprints()
        renderTempObstacles()
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
            if (!path.points || path.points.length < 1) continue

            const color = path.color || '#FFFF00'

            // Draw path line
            ctx.value.beginPath()
            ctx.value.strokeStyle = color
            ctx.value.lineWidth = 2

            const first = naturalToCanvas(path.points[0][0], path.points[0][1])
            ctx.value.moveTo(first.x, first.y)

            for (let i = 1; i < path.points.length; i++) {
                const p = naturalToCanvas(path.points[i][0], path.points[i][1])
                ctx.value.lineTo(p.x, p.y)
            }
            ctx.value.stroke()

            const lastIdx = path.points.length - 1
            const pulse = 0.5 + 0.5 * Math.sin(pathPhase * 4 * Math.PI)

            // Draw intermediate waypoint dots (indices 1 through length-2)
            ctx.value.fillStyle = color
            for (let i = 1; i < lastIdx; i++) {
                const p = naturalToCanvas(path.points[i][0], path.points[i][1])
                ctx.value.beginPath()
                ctx.value.arc(p.x, p.y, 4, 0, Math.PI * 2)
                ctx.value.fill()
            }

            // Draw first waypoint (index 0) — pulsing "next" indicator
            const firstPt = naturalToCanvas(path.points[0][0], path.points[0][1])
            const nextAlpha = 0.4 + 0.6 * pulse
            const nextRadius = 4 + 3 * pulse
            ctx.value.save()
            ctx.value.globalAlpha = nextAlpha
            ctx.value.fillStyle = color
            ctx.value.shadowBlur = 8
            ctx.value.shadowColor = color
            ctx.value.beginPath()
            ctx.value.arc(firstPt.x, firstPt.y, nextRadius, 0, Math.PI * 2)
            ctx.value.fill()
            ctx.value.restore()

            // Draw last waypoint (index length-1) — goal ring indicator
            // Only when there are 2+ points so it doesn't overlap the "next" pulse
            if (lastIdx > 0) {
                const lastPt = naturalToCanvas(path.points[lastIdx][0], path.points[lastIdx][1])
                const glowAlpha = 0.3 + 0.4 * pulse
                ctx.value.save()
                ctx.value.shadowBlur = 10
                ctx.value.shadowColor = color
                ctx.value.globalAlpha = glowAlpha
                ctx.value.beginPath()
                ctx.value.arc(lastPt.x, lastPt.y, 12, 0, Math.PI * 2)
                ctx.value.strokeStyle = color
                ctx.value.lineWidth = 2
                ctx.value.stroke()
                ctx.value.restore()

                // Inner solid ring (always visible)
                ctx.value.beginPath()
                ctx.value.arc(lastPt.x, lastPt.y, 8, 0, Math.PI * 2)
                ctx.value.strokeStyle = color
                ctx.value.lineWidth = 2
                ctx.value.stroke()

                // Small filled center dot
                ctx.value.fillStyle = color
                ctx.value.beginPath()
                ctx.value.arc(lastPt.x, lastPt.y, 3, 0, Math.PI * 2)
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
                ctx.value.fillStyle = THEME.footprintSelectedFill
            } else {
                ctx.value.fillStyle = THEME.footprintFill
            }
            ctx.value.fill()

            // Draw solid edge
            ctx.value.lineWidth = isSelected ? 3 : 2
            ctx.value.strokeStyle = isSelected
                ? THEME.footprintSelectedStroke
                : THEME.footprintStroke
            ctx.value.stroke()
        }
    }

    function renderTempObstacles(): void {
        if (!ctx.value || !tempObstacleStore.enabled) return
        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        const c = ctx.value
        const applied = tempObstacleStore.applied

        for (const obs of tempObstacleStore.obstacles) {
            const r = tempObstacleCanvasRect(obs, naturalToCanvas)

            c.save()
            c.fillStyle = applied ? THEME.tempFill : THEME.tempIdleFill
            c.fillRect(r.x, r.y, r.w, r.h)

            if (!applied) {
                // Hatch the box to show the planner is ignoring it
                c.beginPath()
                c.rect(r.x, r.y, r.w, r.h)
                c.clip()
                c.strokeStyle = THEME.tempIdleStroke
                c.lineWidth = 1
                c.beginPath()
                for (let d = -r.h; d < r.w; d += 8) {
                    c.moveTo(r.x + d, r.y + r.h)
                    c.lineTo(r.x + d + r.h, r.y)
                }
                c.stroke()
            }
            c.restore()

            c.save()
            c.setLineDash([6, 4])
            c.lineWidth = 2
            c.strokeStyle = applied ? THEME.tempStroke : THEME.tempIdleStroke
            c.strokeRect(r.x, r.y, r.w, r.h)
            c.restore()

            c.font = THEME.fontLabel
            c.fillStyle = THEME.tempStroke
            c.fillText(tempObstacleLabel(applied), r.x + 3, Math.max(12, r.y - 4))
        }

        const chip = tempObstacleStatusChip(tempObstacleStore)
        if (chip) {
            c.font = THEME.fontLabel
            const width = c.measureText(chip).width + 16
            c.fillStyle = THEME.tempChipBg
            c.fillRect(8, 8, width, 22)
            c.fillStyle = THEME.tempChipText
            c.fillText(chip, 16, 23)
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
        ctx.value.fillStyle = isInvalid ? THEME.destInvalidFill : THEME.destValidFill
        ctx.value.fill()

        // Draw solid edge
        ctx.value.strokeStyle = isInvalid ? THEME.destInvalidStroke : THEME.destValidStroke
        ctx.value.lineWidth = 2
        ctx.value.setLineDash([5, 5])
        ctx.value.stroke()
        ctx.value.setLineDash([])

        // Draw "DEST" label
        ctx.value.fillStyle = isInvalid ? THEME.destInvalidStroke : THEME.destValidStroke
        ctx.value.font = THEME.fontDest
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
        ctx.value.strokeStyle = THEME.flashOuter
        ctx.value.lineWidth = 4
        ctx.value.stroke()

        ctx.value.beginPath()
        ctx.value.arc(pos.x, pos.y, radius, 0, Math.PI * 2)
        ctx.value.strokeStyle = THEME.flashInner
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
        ctx.value.fillStyle = THEME.goalFill
        ctx.value.fill()

        // Draw solid edge
        ctx.value.strokeStyle = THEME.goalStroke
        ctx.value.lineWidth = 3
        ctx.value.stroke()

        // Draw "GOAL" label
        ctx.value.fillStyle = THEME.goalStroke
        ctx.value.font = THEME.fontDest
        ctx.value.fillText('GOAL', scaled.x + radius + 5, scaled.y)
    }

    function renderDrawingBox(): void {
        const drawRect = obstacleStore.drawRect
        if (!ctx.value || !drawRect) return

        const width = drawRect.x2 - drawRect.x1
        const height = drawRect.y2 - drawRect.y1

        // Draw dashed rectangle
        ctx.value.strokeStyle = THEME.drawingStroke
        ctx.value.lineWidth = 2
        ctx.value.setLineDash([5, 5])
        ctx.value.strokeRect(drawRect.x1, drawRect.y1, width, height)
        ctx.value.setLineDash([])

        // Draw fill
        ctx.value.fillStyle = THEME.drawingFill
        ctx.value.fillRect(drawRect.x1, drawRect.y1, width, height)
    }

    function roundedRectPath(x: number, y: number, w: number, h: number, r: number): void {
        const c = ctx.value
        if (!c) return
        c.beginPath()
        c.moveTo(x + r, y)
        c.lineTo(x + w - r, y)
        c.arc(x + w - r, y + r, r, -Math.PI / 2, 0)
        c.lineTo(x + w, y + h - r)
        c.arc(x + w - r, y + h - r, r, 0, Math.PI / 2)
        c.lineTo(x + r, y + h)
        c.arc(x + r, y + h - r, r, Math.PI / 2, Math.PI)
        c.lineTo(x, y + r)
        c.arc(x + r, y + r, r, Math.PI, (3 * Math.PI) / 2)
        c.closePath()
    }

    function renderCalibrationGuides(): void {
        const c = ctx.value
        const placement = uiStore.calibrationPlacement
        if (!c || !placement) return

        const sx = videoScale.value.x || 1
        const sy = videoScale.value.y || 1

        for (const guide of placement.guides) {
            const center = naturalToCanvas(guide.cx, guide.cy)
            const w = guide.sizePx * sx
            const h = guide.sizePx * sy
            const left = center.x - w / 2
            const top = center.y - h / 2

            const stroke =
                guide.state === 'inside'
                    ? THEME.calibOkStroke
                    : guide.state === 'outside'
                      ? THEME.calibWarnStroke
                      : THEME.calibGuideStroke

            roundedRectPath(left, top, w, h, Math.min(w, h) * 0.12)
            if (guide.state === 'inside') {
                c.fillStyle = THEME.calibOkFill
                c.fill()
            }
            c.strokeStyle = stroke
            c.lineWidth = guide.state === 'empty' ? 2 : 3
            c.setLineDash(guide.state === 'inside' ? [] : [10, 7])
            c.stroke()
            c.setLineDash([])

            // A tag detected outside its box gets a line pointing toward the box
            if (guide.state === 'outside' && guide.tagX !== null && guide.tagY !== null) {
                const tag = naturalToCanvas(guide.tagX, guide.tagY)
                const endX = Math.min(Math.max(tag.x, left), left + w)
                const endY = Math.min(Math.max(tag.y, top), top + h)
                const angle = Math.atan2(endY - tag.y, endX - tag.x)
                c.beginPath()
                c.moveTo(tag.x, tag.y)
                c.lineTo(endX, endY)
                c.lineWidth = 3
                c.strokeStyle = THEME.calibWarnStroke
                c.stroke()
                c.beginPath()
                c.moveTo(endX, endY)
                c.lineTo(endX - 12 * Math.cos(angle - 0.5), endY - 12 * Math.sin(angle - 0.5))
                c.moveTo(endX, endY)
                c.lineTo(endX - 12 * Math.cos(angle + 0.5), endY - 12 * Math.sin(angle + 0.5))
                c.stroke()
            }

            const label = guide.label.split(' (')[0]
            c.font = THEME.fontCalibLarge
            c.textAlign = 'center'
            c.textBaseline = 'middle'
            const labelWidth = c.measureText(label).width + 12
            c.fillStyle = stroke
            c.fillRect(center.x - labelWidth / 2, top - 24, labelWidth, 20)
            c.fillStyle = THEME.labelBg
            c.fillText(label, center.x, top - 14)

            const role = uiStore.calibrationTarget?.tags.find(t => t.id === guide.id)?.role
            if (role === 'center' && guide.state !== 'inside') {
                // The Center tag defines the world axes: its UP arrow must point up the video
                c.fillStyle = THEME.calibGuideStroke
                c.strokeStyle = THEME.calibGuideStroke
                c.lineWidth = 2
                c.beginPath()
                c.moveTo(center.x, center.y + h * 0.22)
                c.lineTo(center.x, center.y - h * 0.22)
                c.moveTo(center.x, center.y - h * 0.22)
                c.lineTo(center.x - 7, center.y - h * 0.22 + 10)
                c.moveTo(center.x, center.y - h * 0.22)
                c.lineTo(center.x + 7, center.y - h * 0.22 + 10)
                c.stroke()
                c.fillText('UP', center.x, center.y + h * 0.36)
            }
        }

        c.textAlign = 'left'
        c.textBaseline = 'alphabetic'
    }

    function renderCalibrationTag(): void {
        if (!ctx.value) return

        const detectedTags = uiStore.detectedTags
        if (!uiStore.panels.calibrationOpen || detectedTags.length === 0) return

        const target = uiStore.calibrationTarget
        const placement = uiStore.calibrationPlacement

        for (const tag of detectedTags) {
            if (!tag.corners || tag.corners.length !== 4) continue

            const corners = tag.corners.map((corner: [number, number]) =>
                naturalToCanvas(corner[0], corner[1])
            )
            const cx = (corners[0].x + corners[2].x) / 2
            const cy = (corners[0].y + corners[2].y) / 2

            ctx.value.beginPath()
            ctx.value.moveTo(corners[0].x, corners[0].y)
            for (const corner of corners.slice(1)) {
                ctx.value.lineTo(corner.x, corner.y)
            }
            ctx.value.closePath()
            ctx.value.setLineDash([])

            const targetTag = target?.tags.find(t => t.id === tag.id)
            if (!targetTag && uiStore.calibrationClearView) continue
            if (!targetTag) {
                ctx.value.strokeStyle = THEME.calibTagStroke
                ctx.value.lineWidth = 1
                ctx.value.stroke()
                ctx.value.fillStyle = THEME.calibTagLabel
                ctx.value.font = THEME.fontCalibSmall
                ctx.value.textAlign = 'center'
                ctx.value.textBaseline = 'middle'
                ctx.value.fillText(`Tag ${tag.id}`, cx, cy)
                continue
            }

            const severity = placement?.tags.find(t => t.id === tag.id)?.severity ?? 'ok'
            const stroke =
                severity === 'blocking'
                    ? THEME.calibBlockStroke
                    : severity === 'warning'
                      ? THEME.calibWarnStroke
                      : THEME.calibOkStroke
            const fill =
                severity === 'blocking'
                    ? THEME.calibBlockFill
                    : severity === 'warning'
                      ? THEME.calibWarnFill
                      : THEME.calibOkFill

            ctx.value.fillStyle = fill
            ctx.value.fill()
            ctx.value.strokeStyle = stroke
            ctx.value.lineWidth = 3
            ctx.value.stroke()

            // Line from the centre to the middle of the top edge shows which way is UP
            ctx.value.beginPath()
            ctx.value.moveTo(cx, cy)
            ctx.value.lineTo((corners[0].x + corners[1].x) / 2, (corners[0].y + corners[1].y) / 2)
            ctx.value.lineWidth = 2
            ctx.value.stroke()

            ctx.value.fillStyle = stroke
            for (const corner of corners) {
                ctx.value.beginPath()
                ctx.value.arc(corner.x, corner.y, 4, 0, Math.PI * 2)
                ctx.value.fill()
            }

            const label = `${targetTag.label.split(' (')[0]} #${tag.id}`
            ctx.value.font = THEME.fontCalibLarge
            ctx.value.textAlign = 'center'
            ctx.value.textBaseline = 'middle'
            const labelWidth = ctx.value.measureText(label).width + 12
            ctx.value.fillStyle = stroke
            ctx.value.fillRect(cx - labelWidth / 2, cy + 8, labelWidth, 22)
            ctx.value.fillStyle = THEME.labelBg
            ctx.value.fillText(label, cx, cy + 19)
        }

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
            const isUnconfigured = !track.configured && track.tag_id !== undefined

            // Draw rectangle
            ctx.value.globalAlpha = isUnconfigured ? 0.4 : 1.0
            ctx.value.strokeStyle = color
            ctx.value.lineWidth = 2
            ctx.value.strokeRect(scaled1.x, scaled1.y, width, height)

            // Draw label
            const label =
                track.configured && track.name
                    ? track.name
                    : track.tag_id !== undefined
                      ? `Tag ${track.tag_id}`
                      : `#${track.id}`
            ctx.value.font = THEME.fontLabel
            const labelWidth = ctx.value.measureText(label).width + 8
            ctx.value.fillStyle = color
            ctx.value.fillRect(scaled1.x, scaled1.y - 18, labelWidth, 18)
            ctx.value.fillStyle = THEME.labelBg
            ctx.value.fillText(label, scaled1.x + 4, scaled1.y - 5)

            // Draw confidence if available
            if (track.confidence > 0) {
                ctx.value.fillStyle = color
                ctx.value.font = THEME.fontSmall
                ctx.value.fillText(
                    `${(track.confidence * 100).toFixed(0)}%`,
                    scaled1.x,
                    scaled2.y + 14
                )
            }

            // Draw animated movement indicator
            if (track.corners && track.corners.length === 4) {
                renderMovementIndicator(track, scaled1, scaled2)
            }

            ctx.value.globalAlpha = 1.0
        }
    }

    function renderMovementIndicator(
        track: Track,
        scaled1: { x: number; y: number },
        scaled2: { x: number; y: number }
    ): void {
        if (!ctx.value) return

        const pixelHeading = computePixelHeading(track)
        if (pixelHeading === null) return

        const centerX = (scaled1.x + scaled2.x) / 2
        const centerY = (scaled1.y + scaled2.y) / 2
        const state = animationState.get(track.id)
        const motionState = track.motion_state || 'stopped'

        // --- Direction chevron (always shown) ---
        const chevronLen = 40
        const chevronSpread = Math.PI / 5 // 36 degrees each side
        const tipX = centerX + chevronLen * Math.cos(pixelHeading)
        const tipY = centerY + chevronLen * Math.sin(pixelHeading)

        // Pulsing opacity for stopped state
        let chevronAlpha = 1.0
        if (motionState === 'stopped' && state) {
            chevronAlpha = 0.4 + 0.6 * (0.5 + 0.5 * Math.sin(state.phase * 2 * Math.PI))
        }

        ctx.value.save()
        ctx.value.globalAlpha = chevronAlpha
        ctx.value.strokeStyle = THEME.headingChevron
        ctx.value.lineWidth = 3.5
        ctx.value.lineCap = 'round'
        ctx.value.lineJoin = 'round'
        ctx.value.shadowBlur = 8
        ctx.value.shadowColor = THEME.headingChevronGlow

        const armLen = chevronLen * 0.6
        const leftX = tipX - armLen * Math.cos(pixelHeading - chevronSpread)
        const leftY = tipY - armLen * Math.sin(pixelHeading - chevronSpread)
        const rightX = tipX - armLen * Math.cos(pixelHeading + chevronSpread)
        const rightY = tipY - armLen * Math.sin(pixelHeading + chevronSpread)

        ctx.value.beginPath()
        ctx.value.moveTo(leftX, leftY)
        ctx.value.lineTo(tipX, tipY)
        ctx.value.lineTo(rightX, rightY)
        ctx.value.stroke()
        ctx.value.restore()

        // --- Movement-specific animations ---
        if (motionState === 'forward' || motionState === 'backward') {
            renderThrustParticles(track, motionState === 'backward')
        } else if (motionState === 'rotating_left' || motionState === 'rotating_right') {
            renderRotationArc(
                centerX,
                centerY,
                pixelHeading,
                motionState === 'rotating_left',
                state
            )
        }
    }

    function renderThrustParticles(track: Track, isReverse: boolean): void {
        if (!ctx.value) return

        const state = animationState.get(track.id)
        if (!state) return

        const color = isReverse ? THEME.thrustReverse : THEME.thrustForward
        const scale = videoScale.value

        for (const p of state.particles) {
            const progress = p.age / p.maxAge
            const alpha = 1.0 - progress
            const radius = 2.5 * (1 - progress * 0.5)

            const canvasPos = naturalToCanvas(p.x, p.y)

            ctx.value.save()
            ctx.value.globalAlpha = alpha * 0.8
            ctx.value.fillStyle = color
            ctx.value.shadowBlur = 6
            ctx.value.shadowColor = color

            ctx.value.beginPath()
            ctx.value.arc(canvasPos.x, canvasPos.y, radius * scale.x, 0, Math.PI * 2)
            ctx.value.fill()
            ctx.value.restore()
        }
    }

    function renderRotationArc(
        centerX: number,
        centerY: number,
        heading: number,
        isLeft: boolean,
        state: TrackAnimState | undefined
    ): void {
        if (!ctx.value || !state) return

        const radius = 25
        const sweepAngle = Math.PI / 2
        const rotationSpeed = 4
        const direction = isLeft ? -1 : 1
        const startAngle = heading + direction * state.phase * rotationSpeed

        ctx.value.save()
        ctx.value.strokeStyle = THEME.rotationArc
        ctx.value.lineWidth = 2.5
        ctx.value.lineCap = 'round'
        ctx.value.shadowBlur = 6
        ctx.value.shadowColor = THEME.rotationArc

        ctx.value.beginPath()
        ctx.value.arc(
            centerX,
            centerY,
            radius,
            startAngle,
            startAngle + direction * sweepAngle,
            isLeft
        )
        ctx.value.stroke()

        // Arrowhead at the leading edge of the arc
        const arrowAngle = startAngle + direction * sweepAngle
        const arrowX = centerX + radius * Math.cos(arrowAngle)
        const arrowY = centerY + radius * Math.sin(arrowAngle)
        const tangentAngle = arrowAngle + (direction * Math.PI) / 2
        const headLen = 7

        ctx.value.beginPath()
        ctx.value.fillStyle = THEME.rotationArc
        ctx.value.moveTo(arrowX, arrowY)
        ctx.value.lineTo(
            arrowX - headLen * Math.cos(tangentAngle - Math.PI / 5),
            arrowY - headLen * Math.sin(tangentAngle - Math.PI / 5)
        )
        ctx.value.lineTo(
            arrowX - headLen * Math.cos(tangentAngle + Math.PI / 5),
            arrowY - headLen * Math.sin(tangentAngle + Math.PI / 5)
        )
        ctx.value.closePath()
        ctx.value.fill()
        ctx.value.restore()
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

    let resizeObserver: ResizeObserver | null = null

    onMounted(() => {
        initialize()
        animationFrameId = requestAnimationFrame(animationLoop)

        resizeObserver = new ResizeObserver(() => {
            syncDimensions()
            updateVideoScale()
            dirty = true
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
        if (animationFrameId !== null) {
            cancelAnimationFrame(animationFrameId)
            animationFrameId = null
        }
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
