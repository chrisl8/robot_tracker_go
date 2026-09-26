import { describe, it, expect } from 'vitest'
import { assessPlacement, PLACEMENT_THRESHOLDS } from '../calibrationPlacement'
import type { CalibrationTarget, DetectedTagInfo } from '@/types/api'

const FRAME = { width: 1280, height: 720 }

const TARGET: CalibrationTarget = {
    tagSize: 0.15,
    defaultWidth: 1.0,
    defaultDepth: 0.6,
    tags: [
        { id: 100, label: 'Center', role: 'center', col: 0, row: 0 },
        { id: 101, label: 'Corner 1 (top-left)', role: 'corner', col: -1, row: -1 },
        { id: 102, label: 'Corner 2 (top-right)', role: 'corner', col: 1, row: -1 },
        { id: 103, label: 'Corner 3 (bottom-right)', role: 'corner', col: 1, row: 1 },
        { id: 104, label: 'Corner 4 (bottom-left)', role: 'corner', col: -1, row: 1 },
    ],
}

function tagAt(id: number, cx: number, cy: number, size = 60, rotDeg = 0): DetectedTagInfo {
    const r = (rotDeg * Math.PI) / 180
    const half = size / 2
    const local: [number, number][] = [
        [-half, -half],
        [half, -half],
        [half, half],
        [-half, half],
    ]
    const corners = local.map(
        ([x, y]) =>
            [cx + x * Math.cos(r) - y * Math.sin(r), cy + x * Math.sin(r) + y * Math.cos(r)] as [
                number,
                number,
            ]
    )
    return { id, center: [cx, cy], corners }
}

function goodLayout(): DetectedTagInfo[] {
    return [
        tagAt(100, 640, 360),
        tagAt(101, 250, 130),
        tagAt(102, 1030, 130),
        tagAt(103, 1030, 590),
        tagAt(104, 250, 590),
    ]
}

describe('assessPlacement', () => {
    it('accepts a well spread layout with no issues', () => {
        const result = assessPlacement(goodLayout(), FRAME, TARGET)
        expect(result.allFound).toBe(true)
        expect(result.canCalibrate).toBe(true)
        expect(result.issues).toEqual([])
        expect(result.tags.every(t => t.found && t.severity === 'ok')).toBe(true)
        expect(result.tags[0].sizePx).toBe(60)
    })

    it('ignores non-target tags', () => {
        const result = assessPlacement([...goodLayout(), tagAt(1, 900, 300)], FRAME, TARGET)
        expect(result.issues).toEqual([])
        expect(result.tags).toHaveLength(5)
    })

    it('blocks calibration and names each missing tag', () => {
        const tags = goodLayout().filter(t => t.id !== 103)
        const result = assessPlacement(tags, FRAME, TARGET)
        expect(result.allFound).toBe(false)
        expect(result.canCalibrate).toBe(false)
        const missing = result.issues.find(i => i.tagId === 103)
        expect(missing?.severity).toBe('blocking')
        expect(missing?.message).toContain('Corner 3')
        expect(result.tags.find(t => t.id === 103)?.found).toBe(false)
    })

    it('reports everything missing when nothing is detected', () => {
        const result = assessPlacement([], FRAME, TARGET)
        expect(result.canCalibrate).toBe(false)
        expect(result.issues).toHaveLength(5)
    })

    describe('tag size', () => {
        const cases = [
            { name: 'comfortably large', size: PLACEMENT_THRESHOLDS.minTagPxWarn + 10, sev: null },
            { name: 'small warns', size: PLACEMENT_THRESHOLDS.minTagPxWarn - 5, sev: 'warning' },
            { name: 'tiny blocks', size: PLACEMENT_THRESHOLDS.minTagPxBlock - 5, sev: 'blocking' },
        ]
        for (const c of cases) {
            it(c.name, () => {
                const tags = goodLayout().map(t =>
                    t.id === 102 ? tagAt(102, 1030, 130, c.size) : t
                )
                const result = assessPlacement(tags, FRAME, TARGET)
                const issue = result.issues.find(i => i.tagId === 102)
                expect(issue?.severity ?? null).toBe(c.sev)
                expect(result.canCalibrate).toBe(c.sev !== 'blocking')
            })
        }
    })

    describe('steep viewing angle', () => {
        function squashed(id: number, cx: number, cy: number, ratio: number): DetectedTagInfo {
            const t = tagAt(id, cx, cy, 80)
            const squash = t.corners.map(([x, y]) => [x, cy + (y - cy) * ratio] as [number, number])
            return { ...t, corners: squash }
        }
        it('warns when moderately foreshortened', () => {
            const tags = goodLayout().map(t => (t.id === 101 ? squashed(101, 250, 130, 0.5) : t))
            const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 101)
            expect(issue?.severity).toBe('warning')
            expect(issue?.message).toContain('steep angle')
        })
        it('blocks when extremely foreshortened', () => {
            const tags = goodLayout().map(t => (t.id === 101 ? squashed(101, 250, 130, 0.25) : t))
            const result = assessPlacement(tags, FRAME, TARGET)
            expect(result.issues.find(i => i.tagId === 101)?.severity).toBe('blocking')
            expect(result.canCalibrate).toBe(false)
        })
    })

    describe('rotation', () => {
        it('warns for a slight rotation', () => {
            const tags = goodLayout().map(t => (t.id === 100 ? tagAt(100, 640, 360, 60, 30) : t))
            const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 100)
            expect(issue?.severity).toBe('warning')
        })
        it('blocks a tag turned a quarter turn', () => {
            const tags = goodLayout().map(t => (t.id === 100 ? tagAt(100, 640, 360, 60, 90) : t))
            const result = assessPlacement(tags, FRAME, TARGET)
            expect(result.issues.find(i => i.tagId === 100)?.severity).toBe('blocking')
            expect(result.canCalibrate).toBe(false)
        })
        it('blocks a tag turned upside down', () => {
            const tags = goodLayout().map(t => (t.id === 100 ? tagAt(100, 640, 360, 60, 180) : t))
            expect(assessPlacement(tags, FRAME, TARGET).canCalibrate).toBe(false)
        })
    })

    it('warns when the Center tag is far from the middle of the view', () => {
        const tags = goodLayout().map(t => (t.id === 100 ? tagAt(100, 240, 360) : t))
        const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 100)
        expect(issue?.severity).toBe('warning')
        expect(issue?.message).toContain('Center')
    })

    it('blocks a corner tag on the wrong side of the Center tag', () => {
        const swapped = goodLayout().map(t => {
            if (t.id === 101) return tagAt(101, 1030, 130)
            if (t.id === 102) return tagAt(102, 250, 130)
            return t
        })
        const result = assessPlacement(swapped, FRAME, TARGET)
        const issue = result.issues.find(i => i.tagId === 101)
        expect(issue?.severity).toBe('blocking')
        expect(issue?.message).toContain('left')
        expect(result.canCalibrate).toBe(false)
    })

    it('warns about tags that cover too little of the view', () => {
        const tight = [
            tagAt(100, 640, 360),
            tagAt(101, 560, 300),
            tagAt(102, 720, 300),
            tagAt(103, 720, 420),
            tagAt(104, 560, 420),
        ]
        const result = assessPlacement(tight, FRAME, TARGET)
        const spread = result.issues.find(i => i.tagId === null)
        expect(spread?.severity).toBe('warning')
        expect(spread?.message).toContain('Spread')
        expect(result.canCalibrate).toBe(true)
    })

    it('warns about a tag hugging the edge of the frame', () => {
        const tags = goodLayout().map(t => (t.id === 104 ? tagAt(104, 22, 590) : t))
        const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 104)
        expect(issue?.severity).toBe('warning')
        expect(issue?.message).toContain('edge')
    })

    it('treats a detection without four corners as missing', () => {
        const broken = goodLayout().map(t => (t.id === 100 ? { ...t, corners: [] } : t))
        const result = assessPlacement(broken, FRAME, TARGET)
        expect(result.tags.find(t => t.id === 100)?.found).toBe(false)
        expect(result.canCalibrate).toBe(false)
    })
})
