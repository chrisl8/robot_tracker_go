import type { ComputedRef } from 'vue'
import type { useUIStore } from '@/stores/uiStore'
import { naturalToCanvas, videoScale } from '@/utils/canvasTransform'
import { THEME } from './theme'

export interface CalibrationRendererDeps {
    ctx: ComputedRef<CanvasRenderingContext2D | null>
    uiStore: ReturnType<typeof useUIStore>
}

// Draws the calibration aids: the on-video guide boxes the user drops tags into, and
// outlines/labels for detected tags while the calibration panel is open.
export function createCalibrationRenderer({ ctx, uiStore }: CalibrationRendererDeps) {
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

    return { renderCalibrationGuides, renderCalibrationTag }
}
