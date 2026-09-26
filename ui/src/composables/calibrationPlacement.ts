import type { CalibrationTarget, CalibrationTargetTag, DetectedTagInfo } from '@/types/api'

export const PLACEMENT_THRESHOLDS = {
    minTagPxWarn: 40,
    minTagPxBlock: 25,
    minAspectWarn: 0.6,
    minAspectBlock: 0.35,
    maxCenterOffsetFrac: 0.2,
    minSpreadXFrac: 0.5,
    minSpreadYFrac: 0.4,
    edgeMarginFrac: 0.03,
    rotationWarnDeg: 20,
    rotationBlockDeg: 45,
} as const

export type PlacementSeverity = 'ok' | 'warning' | 'blocking'

export interface PlacementIssue {
    tagId: number | null
    severity: 'warning' | 'blocking'
    message: string
}

export interface TagPlacementStatus {
    id: number
    label: string
    found: boolean
    sizePx: number | null
    severity: PlacementSeverity
}

export interface PlacementAssessment {
    tags: TagPlacementStatus[]
    issues: PlacementIssue[]
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

function worse(a: PlacementSeverity, b: PlacementSeverity): PlacementSeverity {
    const rank: Record<PlacementSeverity, number> = { ok: 0, warning: 1, blocking: 2 }
    return rank[a] >= rank[b] ? a : b
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
                message: `${t.label} not detected yet. Make sure it is flat, in view and well lit.`,
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

        const topAngle = Math.atan2(
            pts[1].y - pts[0].y + (pts[2].y - pts[3].y),
            pts[1].x - pts[0].x + (pts[2].x - pts[3].x)
        )
        const rotationDeg = Math.abs((topAngle * 180) / Math.PI)
        if (rotationDeg > T.rotationBlockDeg) {
            flag(
                t,
                'blocking',
                `${name} is rotated. Turn it so its UP arrow points toward the top of the camera view.`
            )
        } else if (rotationDeg > T.rotationWarnDeg) {
            flag(
                t,
                'warning',
                `${name} is slightly rotated. Line its UP arrow up with the top of the camera view.`
            )
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
    }

    const centerTag = target.tags.find(t => t.role === 'center')
    const centerPts = centerTag ? geometry.get(centerTag.id) : undefined
    if (centerTag && centerPts) {
        const c = centroid(centerPts)
        const offset = dist(c, { x: frame.width / 2, y: frame.height / 2 }) / frame.width
        if (offset > T.maxCenterOffsetFrac) {
            flag(
                centerTag,
                'warning',
                `The Center tag should be near the middle of the view. Move it toward the middle.`
            )
        }

        for (const t of target.tags) {
            if (t.role !== 'corner') continue
            const pts = geometry.get(t.id)
            if (!pts) continue
            const p = centroid(pts)
            const wrongX = Math.sign(p.x - c.x) !== Math.sign(t.col)
            const wrongY = Math.sign(p.y - c.y) !== Math.sign(t.row)
            if (wrongX || wrongY) {
                flag(
                    t,
                    'blocking',
                    `${shortLabel(t.label)} must be ${t.row < 0 ? 'above' : 'below'} and to the ${t.col < 0 ? 'left' : 'right'} of the Center tag as seen in the camera view.`
                )
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
                message: `The tags only cover ${Math.round(spreadX * 100)}% of the view width and ${Math.round(spreadY * 100)}% of its height. Spread the corner tags toward the edges of the area the robot will drive in.`,
            })
        }
    }

    const allFound = target.tags.every(t => statuses.get(t.id)?.found === true)
    return {
        tags: target.tags.map(t => statuses.get(t.id) as TagPlacementStatus),
        issues,
        allFound,
        canCalibrate: allFound && !issues.some(i => i.severity === 'blocking'),
    }
}
