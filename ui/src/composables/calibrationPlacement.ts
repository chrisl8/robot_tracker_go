import type { CalibrationTarget, CalibrationTargetTag, DetectedTagInfo } from '@/types/api'

export const PLACEMENT_THRESHOLDS = {
    minTagPxWarn: 40,
    minTagPxBlock: 25,
    minAspectWarn: 0.6,
    minAspectBlock: 0.35,
    boxWarnFrac: 0.12,
    minSpreadXFrac: 0.5,
    minSpreadYFrac: 0.4,
    edgeMarginFrac: 0.03,
    rotationWarnDeg: 20,
    maxVisibleHints: 3,
    boxToTagRatio: 2,
    fallbackBoxFrac: 0.09,
    centerGuideEpsilon: 0.05,
} as const

export type PlacementSeverity = 'ok' | 'warning' | 'blocking'
export type GuideState = 'empty' | 'inside' | 'outside'

export interface PlacementIssue {
    tagId: number | null
    severity: 'warning' | 'blocking'
    /** A tip is advice only: it never changes a tag's colour or blocks Calibrate. */
    tip?: boolean
    message: string
}

export interface TagPlacementStatus {
    id: number
    label: string
    found: boolean
    sizePx: number | null
    severity: PlacementSeverity
}

/** A guide box in natural video-frame pixels, plus how the tag sits relative to it. */
export interface GuideBox {
    id: number
    label: string
    cx: number
    cy: number
    sizePx: number
    state: GuideState
    tagX: number | null
    tagY: number | null
}

export interface PlacementAssessment {
    tags: TagPlacementStatus[]
    issues: PlacementIssue[]
    guides: GuideBox[]
    allFound: boolean
    canCalibrate: boolean
}

interface FrameSize {
    width: number
    height: number
}

interface Point {
    x: number
    y: number
}

function dist(a: Point, b: Point): number {
    return Math.hypot(a.x - b.x, a.y - b.y)
}

function toPoints(tag: DetectedTagInfo): Point[] | null {
    if (tag.corners.length !== 4) return null
    return tag.corners.map(c => ({ x: c[0], y: c[1] }))
}

function centroid(pts: Point[]): Point {
    const sum = pts.reduce((acc, p) => ({ x: acc.x + p.x, y: acc.y + p.y }), { x: 0, y: 0 })
    return { x: sum.x / pts.length, y: sum.y / pts.length }
}

function shortLabel(label: string): string {
    return label.split(' (')[0]
}

function median(values: number[]): number {
    const sorted = [...values].sort((a, b) => a - b)
    const mid = Math.floor(sorted.length / 2)
    return sorted.length % 2 === 1 ? sorted[mid] : (sorted[mid - 1] + sorted[mid]) / 2
}

function worse(a: PlacementSeverity, b: PlacementSeverity): PlacementSeverity {
    const rank: Record<PlacementSeverity, number> = { ok: 0, warning: 1, blocking: 2 }
    return rank[a] >= rank[b] ? a : b
}

function guideDescription(tag: CalibrationTargetTag): string {
    const eps = PLACEMENT_THRESHOLDS.centerGuideEpsilon
    const vertical = tag.guideY < 0.5 - eps ? 'top' : tag.guideY > 0.5 + eps ? 'bottom' : ''
    const horizontal = tag.guideX < 0.5 - eps ? 'left' : tag.guideX > 0.5 + eps ? 'right' : ''
    return [vertical, horizontal].filter(Boolean).join('-') || 'middle'
}

export function assessPlacement(
    tags: DetectedTagInfo[],
    frame: FrameSize,
    target: CalibrationTarget
): PlacementAssessment {
    const T = PLACEMENT_THRESHOLDS
    const issues: PlacementIssue[] = []
    const statuses = new Map<number, TagPlacementStatus>()
    const geometry = new Map<number, Point[]>()

    const flag = (
        tag: CalibrationTargetTag,
        severity: 'warning' | 'blocking',
        message: string
    ): void => {
        issues.push({ tagId: tag.id, severity, message })
        const status = statuses.get(tag.id)
        if (status) status.severity = worse(status.severity, severity)
    }

    for (const t of target.tags) {
        const detected = tags.find(d => d.id === t.id)
        const pts = detected ? toPoints(detected) : null
        if (!pts) {
            statuses.set(t.id, {
                id: t.id,
                label: t.label,
                found: false,
                sizePx: null,
                severity: 'blocking',
            })
            issues.push({
                tagId: t.id,
                severity: 'blocking',
                message: `${t.label} not detected yet. Put it in its dashed box, flat and well lit.`,
            })
            continue
        }
        geometry.set(t.id, pts)
        const w = (dist(pts[0], pts[1]) + dist(pts[3], pts[2])) / 2
        const h = (dist(pts[1], pts[2]) + dist(pts[0], pts[3])) / 2
        statuses.set(t.id, {
            id: t.id,
            label: t.label,
            found: true,
            sizePx: Math.round((w + h) / 2),
            severity: 'ok',
        })
    }

    const sizes = [...statuses.values()].flatMap(s => (s.sizePx === null ? [] : [s.sizePx]))
    const boxSizePx =
        sizes.length > 0 ? T.boxToTagRatio * median(sizes) : T.fallbackBoxFrac * frame.width

    const guides: GuideBox[] = target.tags.map(t => {
        const cx = t.guideX * frame.width
        const cy = t.guideY * frame.height
        const pts = geometry.get(t.id)
        if (!pts) {
            return {
                id: t.id,
                label: t.label,
                cx,
                cy,
                sizePx: boxSizePx,
                state: 'empty',
                tagX: null,
                tagY: null,
            }
        }
        const c = centroid(pts)
        const inside = Math.abs(c.x - cx) <= boxSizePx / 2 && Math.abs(c.y - cy) <= boxSizePx / 2
        return {
            id: t.id,
            label: t.label,
            cx,
            cy,
            sizePx: boxSizePx,
            state: inside ? 'inside' : 'outside',
            tagX: c.x,
            tagY: c.y,
        }
    })

    for (const t of target.tags) {
        const pts = geometry.get(t.id)
        const status = statuses.get(t.id)
        if (!pts || !status || status.sizePx === null) continue
        const name = shortLabel(t.label)

        if (status.sizePx < T.minTagPxBlock) {
            flag(
                t,
                'blocking',
                `${name} is only ${status.sizePx} px across. Move it closer to the camera.`
            )
        } else if (status.sizePx < T.minTagPxWarn) {
            flag(
                t,
                'warning',
                `${name} is small (${status.sizePx} px, want at least ${T.minTagPxWarn}). Move it closer to the camera.`
            )
        }

        const w = (dist(pts[0], pts[1]) + dist(pts[3], pts[2])) / 2
        const h = (dist(pts[1], pts[2]) + dist(pts[0], pts[3])) / 2
        const aspect = Math.min(w, h) / Math.max(w, h)
        if (aspect < T.minAspectBlock) {
            flag(
                t,
                'blocking',
                `${name} is seen at a very steep angle. Move it toward the middle of the view or aim the camera more downward.`
            )
        } else if (aspect < T.minAspectWarn) {
            flag(
                t,
                'warning',
                `${name} is seen at a steep angle. Moving it toward the middle of the view will help.`
            )
        }

        // Only the Center tag's orientation matters, and only for tidy floor axes.
        if (t.role === 'center') {
            const topAngle = Math.atan2(
                pts[1].y - pts[0].y + (pts[2].y - pts[3].y),
                pts[1].x - pts[0].x + (pts[2].x - pts[3].x)
            )
            const rotationDeg = Math.abs((topAngle * 180) / Math.PI)
            if (rotationDeg > T.rotationWarnDeg) {
                issues.push({
                    tagId: t.id,
                    severity: 'warning',
                    tip: true,
                    message:
                        'Tip: turn the Center tag so its UP arrow points toward the top of the video for tidy axes.',
                })
            }
        }

        const margin = T.edgeMarginFrac * Math.min(frame.width, frame.height)
        const nearEdge = pts.some(
            p =>
                p.x < margin ||
                p.y < margin ||
                p.x > frame.width - margin ||
                p.y > frame.height - margin
        )
        if (nearEdge) {
            flag(
                t,
                'warning',
                `${name} is very close to the edge of the view. Move it a little toward the middle.`
            )
        }

        const guide = guides.find(g => g.id === t.id)
        if (guide && guide.tagX !== null && guide.tagY !== null) {
            const eps = T.centerGuideEpsilon
            const wrongX =
                Math.abs(t.guideX - 0.5) > eps &&
                (guide.tagX / frame.width - 0.5) * (t.guideX - 0.5) < 0
            const wrongY =
                Math.abs(t.guideY - 0.5) > eps &&
                (guide.tagY / frame.height - 0.5) * (t.guideY - 0.5) < 0
            if (wrongX || wrongY) {
                flag(
                    t,
                    'blocking',
                    `${name} belongs in the ${guideDescription(t)} box, but it is on the wrong side of the view. Move it into its dashed box.`
                )
            } else if (
                dist({ x: guide.tagX, y: guide.tagY }, { x: guide.cx, y: guide.cy }) >
                T.boxWarnFrac * frame.width
            ) {
                flag(t, 'warning', `${name} is far from its box. Move it into the dashed box.`)
            }
        }
    }

    const found = [...geometry.values()].map(centroid)
    if (found.length >= 2) {
        const xs = found.map(p => p.x)
        const ys = found.map(p => p.y)
        const spreadX = (Math.max(...xs) - Math.min(...xs)) / frame.width
        const spreadY = (Math.max(...ys) - Math.min(...ys)) / frame.height
        if (spreadX < T.minSpreadXFrac || spreadY < T.minSpreadYFrac) {
            issues.push({
                tagId: null,
                severity: 'warning',
                message: `The tags only cover ${Math.round(spreadX * 100)}% of the view width and ${Math.round(spreadY * 100)}% of its height. Move the corner tags out toward their boxes.`,
            })
        }
    }

    for (const g of guides) {
        const status = statuses.get(g.id)
        if (status && g.state === 'outside') status.severity = worse(status.severity, 'warning')
    }

    const allFound = target.tags.every(t => statuses.get(t.id)?.found === true)
    issues.sort((a, b) => Number(a.severity === 'warning') - Number(b.severity === 'warning'))
    issues.sort((a, b) => Number(a.tip === true) - Number(b.tip === true))
    return {
        tags: target.tags.map(t => statuses.get(t.id) as TagPlacementStatus),
        issues,
        guides,
        allFound,
        canCalibrate: allFound && !issues.some(i => i.severity === 'blocking'),
    }
}
