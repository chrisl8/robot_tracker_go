import type { ComputedRef, Ref } from 'vue'
import type { Track } from '@/types/api'
import type { useRobotStore } from '@/stores/robotStore'
import type { useUIStore } from '@/stores/uiStore'
import { getTrackColor } from '@/types/robot'
import { getVideoDimensions } from '@/utils/coordinates'
import { naturalToCanvas, videoScale } from '@/utils/canvasTransform'
import { THEME } from './theme'
import { computePixelHeading, type TrackAnimation, type TrackAnimState } from './trackAnimation'

export interface TrackRendererDeps {
    ctx: ComputedRef<CanvasRenderingContext2D | null>
    dimensions: Ref<{ width: number; height: number }>
    robotStore: ReturnType<typeof useRobotStore>
    uiStore: ReturnType<typeof useUIStore>
    animation: TrackAnimation
}

// Draws robot footprints, tracked-object boxes/labels, and the animated movement
// indicators (heading chevron, thrust particles, rotation arc).
export function createTrackRenderer({
    ctx,
    dimensions,
    robotStore,
    uiStore,
    animation,
}: TrackRendererDeps) {
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
        const state = animation.animationState.get(track.id)
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

        const state = animation.animationState.get(track.id)
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

    return { renderFootprints, renderTracks }
}
