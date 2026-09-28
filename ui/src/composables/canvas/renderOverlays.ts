import type { ComputedRef, Ref } from 'vue'
import type { useRobotStore } from '@/stores/robotStore'
import type { useObstacleStore } from '@/stores/obstacleStore'
import type { useTempObstacleStore } from '@/stores/tempObstacleStore'
import {
    tempObstacleCanvasPolygon,
    tempObstacleCanvasRect,
    tempObstacleLabel,
    tempObstacleStatusChip,
    topmostPoint,
} from '@/utils/tempObstacles'
import { canvasToNaturalShared, naturalToCanvas, videoScale } from '@/utils/canvasTransform'
import { isCircleInObstacle } from '@/utils/obstacleCollision'
import { THEME } from './theme'
import type { TrackAnimation } from './trackAnimation'

export interface FlashPoint {
    x: number
    y: number
    active: boolean
}

export interface OverlayRendererDeps {
    ctx: ComputedRef<CanvasRenderingContext2D | null>
    robotStore: ReturnType<typeof useRobotStore>
    obstacleStore: ReturnType<typeof useObstacleStore>
    tempObstacleStore: ReturnType<typeof useTempObstacleStore>
    animation: TrackAnimation
    mousePosition: Ref<{ x: number; y: number } | null>
    flashInvalid: Ref<FlashPoint | null>
}

// Draws everything on the overlay that is not a track or calibration aid: planned
// paths, destination cursor/marker, the invalid-destination flash, the obstacle
// drawing box, and temporary (foreground-detected) obstacles. Static obstacles are
// drawn by the backend on the video frame itself.
export function createOverlayRenderer({
    ctx,
    robotStore,
    obstacleStore,
    tempObstacleStore,
    animation,
    mousePosition,
    flashInvalid,
}: OverlayRendererDeps) {
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
            const pulse = 0.5 + 0.5 * Math.sin(animation.getPathPhase() * 4 * Math.PI)

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

    function renderTempObstacles(): void {
        if (!ctx.value || !tempObstacleStore.enabled) return
        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        const c = ctx.value
        const applied = tempObstacleStore.applied

        for (const obs of tempObstacleStore.obstacles) {
            const polygon = tempObstacleCanvasPolygon(obs, naturalToCanvas)
            const r = polygon ? null : tempObstacleCanvasRect(obs, naturalToCanvas)
            const tracePath = (): void => {
                c.beginPath()
                if (polygon) {
                    c.moveTo(polygon[0].x, polygon[0].y)
                    for (let i = 1; i < polygon.length; i++) c.lineTo(polygon[i].x, polygon[i].y)
                    c.closePath()
                } else if (r) {
                    c.rect(r.x, r.y, r.w, r.h)
                }
            }
            // Bounds of the shape (its rect, or the polygon's own AABB), used
            // for the hatch fill's span so diagonal lines still cover it.
            const bounds = r ?? {
                x: Math.min(...polygon!.map(p => p.x)),
                y: Math.min(...polygon!.map(p => p.y)),
                w: Math.max(...polygon!.map(p => p.x)) - Math.min(...polygon!.map(p => p.x)),
                h: Math.max(...polygon!.map(p => p.y)) - Math.min(...polygon!.map(p => p.y)),
            }

            c.save()
            c.fillStyle = applied ? THEME.tempFill : THEME.tempIdleFill
            tracePath()
            c.fill()

            if (!applied) {
                // Hatch the shape to show the planner is ignoring it
                tracePath()
                c.clip()
                c.strokeStyle = THEME.tempIdleStroke
                c.lineWidth = 1
                c.beginPath()
                for (let d = -bounds.h; d < bounds.w; d += 8) {
                    c.moveTo(bounds.x + d, bounds.y + bounds.h)
                    c.lineTo(bounds.x + d + bounds.h, bounds.y)
                }
                c.stroke()
            }
            c.restore()

            c.save()
            c.setLineDash([6, 4])
            c.lineWidth = 2
            c.strokeStyle = applied ? THEME.tempStroke : THEME.tempIdleStroke
            tracePath()
            c.stroke()
            c.restore()

            const labelPoint = polygon ? topmostPoint(polygon) : { x: r!.x, y: r!.y }
            c.font = THEME.fontLabel
            c.fillStyle = THEME.tempStroke
            c.fillText(tempObstacleLabel(applied), labelPoint.x + 3, Math.max(12, labelPoint.y - 4))
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

    function renderDestinationCursor(): void {
        if (!ctx.value || !robotStore.destinationMode || !mousePosition.value) return

        const selectedTrack = robotStore.selectedTrack
        if (!selectedTrack || !selectedTrack.pixel_radius) return

        const pos = mousePosition.value

        if (videoScale.value.x === 0 || videoScale.value.y === 0) return

        // Convert canvas mouse position to natural video coordinates for collision check
        const naturalPos = canvasToNaturalShared(pos.x, pos.y)
        const radius = selectedTrack.pixel_radius * videoScale.value.x
        const naturalRadius = selectedTrack.pixel_radius

        // Check collision using natural video coordinates
        const isInvalid = isCircleInObstacle(
            obstacleStore.obstacles,
            naturalPos.x,
            naturalPos.y,
            naturalRadius
        )

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

    return {
        renderPaths,
        renderTempObstacles,
        renderDestinationCursor,
        triggerInvalidFlash,
        renderInvalidFlash,
        renderDestinationMarker,
        renderDrawingBox,
    }
}
