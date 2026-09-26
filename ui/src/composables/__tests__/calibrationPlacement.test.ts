import { describe, it, expect } from 'vitest'
import { assessPlacement, PLACEMENT_THRESHOLDS } from '../calibrationPlacement'
import type { CalibrationTarget, DetectedTagInfo } from '@/types/api'

const FRAME = { width: 1280, height: 720 }

const TARGET: CalibrationTarget = {
    tagSize: 0.15,
    tags: [
        { id: 100, label: 'Center', role: 'center', guideX: 0.5, guideY: 0.5 },
        { id: 101, label: 'Corner 1 (top-left)', role: 'corner', guideX: 0.22, guideY: 0.24 },
        { id: 102, label: 'Corner 2 (top-right)', role: 'corner', guideX: 0.78, guideY: 0.24 },
        { id: 103, label: 'Corner 3 (bottom-right)', role: 'corner', guideX: 0.78, guideY: 0.76 },
        { id: 104, label: 'Corner 4 (bottom-left)', role: 'corner', guideX: 0.22, guideY: 0.76 },
    ],
}

const GUIDE = {
    100: [640, 360],
    101: [281.6, 172.8],
    102: [998.4, 172.8],
    103: [998.4, 547.2],
    104: [281.6, 547.2],
} as const

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

function inBox(id: keyof typeof GUIDE, size = 60, rotDeg = 0): DetectedTagInfo {
    return tagAt(id, GUIDE[id][0], GUIDE[id][1], size, rotDeg)
}

function goodLayout(): DetectedTagInfo[] {
    return ([100, 101, 102, 103, 104] as const).map(id => inBox(id))
}

function replace(tags: DetectedTagInfo[], next: DetectedTagInfo): DetectedTagInfo[] {
    return tags.map(t => (t.id === next.id ? next : t))
}

describe('assessPlacement', () => {
    it('accepts tags sitting in their boxes with no issues', () => {
        const result = assessPlacement(goodLayout(), FRAME, TARGET)
        expect(result.allFound).toBe(true)
        expect(result.canCalibrate).toBe(true)
        expect(result.issues).toEqual([])
        expect(result.tags.every(t => t.found && t.severity === 'ok')).toBe(true)
        expect(result.tags[0].sizePx).toBe(60)
        expect(result.guides.every(g => g.state === 'inside')).toBe(true)
    })

    it('accepts sloppy placement: tags anywhere inside their boxes', () => {
        let tags = goodLayout()
        tags = replace(tags, tagAt(101, GUIDE[101][0] + 40, GUIDE[101][1] - 35))
        tags = replace(tags, tagAt(103, GUIDE[103][0] - 30, GUIDE[103][1] + 45, 60, 25))
        const result = assessPlacement(tags, FRAME, TARGET)
        expect(result.canCalibrate).toBe(true)
        expect(result.guides.find(g => g.id === 101)?.state).toBe('inside')
    })

    it('ignores non-target tags', () => {
        const result = assessPlacement([...goodLayout(), tagAt(1, 900, 300)], FRAME, TARGET)
        expect(result.issues).toEqual([])
        expect(result.tags).toHaveLength(5)
        expect(result.guides).toHaveLength(5)
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
        expect(result.guides.find(g => g.id === 103)?.state).toBe('empty')
    })

    it('reports everything missing when nothing is detected', () => {
        const result = assessPlacement([], FRAME, TARGET)
        expect(result.canCalibrate).toBe(false)
        expect(result.issues).toHaveLength(5)
        expect(result.guides.every(g => g.state === 'empty' && g.tagX === null)).toBe(true)
    })

    describe('guide boxes', () => {
        it('places boxes at the guide fractions of the frame', () => {
            const guides = assessPlacement([], FRAME, TARGET).guides
            expect(guides.find(g => g.id === 100)).toMatchObject({ cx: 640, cy: 360 })
            expect(guides.find(g => g.id === 104)).toMatchObject({ cx: 281.6, cy: 547.2 })
        })

        it('falls back to a fraction of the frame width when no tags are visible', () => {
            const guides = assessPlacement([], FRAME, TARGET).guides
            expect(guides[0].sizePx).toBeCloseTo(PLACEMENT_THRESHOLDS.fallbackBoxFrac * 1280)
        })

        it('sizes boxes at twice the median detected tag size', () => {
            const tags = [inBox(100, 40), inBox(101, 60), inBox(102, 80)]
            const guides = assessPlacement(tags, FRAME, TARGET).guides
            expect(guides.every(g => g.sizePx === 120)).toBe(true)
        })

        it('marks a tag just outside its box without complaining', () => {
            // 60 px tag, 120 px box: 80 px off centre is outside the box but close by
            const tags = replace(goodLayout(), tagAt(102, GUIDE[102][0] + 80, GUIDE[102][1]))
            const result = assessPlacement(tags, FRAME, TARGET)
            const guide = result.guides.find(g => g.id === 102)
            expect(guide?.state).toBe('outside')
            expect(guide?.tagX).toBeCloseTo(GUIDE[102][0] + 80)
            expect(result.issues).toEqual([])
            expect(result.canCalibrate).toBe(true)
        })

        it('warns when a tag is far from its box', () => {
            const far = PLACEMENT_THRESHOLDS.boxWarnFrac * FRAME.width + 30
            const tags = replace(goodLayout(), tagAt(102, GUIDE[102][0] - far, GUIDE[102][1]))
            const result = assessPlacement(tags, FRAME, TARGET)
            const issue = result.issues.find(i => i.tagId === 102)
            expect(issue?.severity).toBe('warning')
            expect(issue?.message).toContain('box')
            expect(result.canCalibrate).toBe(true)
        })

        it('blocks a tag on the wrong side of the view for its box', () => {
            const swapped = replace(
                replace(goodLayout(), tagAt(101, GUIDE[102][0], GUIDE[102][1])),
                tagAt(102, GUIDE[101][0], GUIDE[101][1])
            )
            const result = assessPlacement(swapped, FRAME, TARGET)
            const issue = result.issues.find(i => i.tagId === 101)
            expect(issue?.severity).toBe('blocking')
            expect(issue?.message).toContain('top-left')
            expect(result.canCalibrate).toBe(false)
        })

        it('blocks a bottom tag placed in the top half', () => {
            const tags = replace(goodLayout(), tagAt(104, GUIDE[104][0], 100))
            const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 104)
            expect(issue?.severity).toBe('blocking')
            expect(issue?.message).toContain('bottom-left')
        })

        it('never blocks the Center tag for being on one side', () => {
            const tags = replace(goodLayout(), tagAt(100, 300, 200))
            const issues = assessPlacement(tags, FRAME, TARGET).issues.filter(i => i.tagId === 100)
            expect(issues.every(i => i.severity === 'warning')).toBe(true)
        })
    })

    describe('tag size', () => {
        const cases = [
            { name: 'comfortably large', size: PLACEMENT_THRESHOLDS.minTagPxWarn + 10, sev: null },
            { name: 'small warns', size: PLACEMENT_THRESHOLDS.minTagPxWarn - 5, sev: 'warning' },
            { name: 'tiny blocks', size: PLACEMENT_THRESHOLDS.minTagPxBlock - 5, sev: 'blocking' },
        ]
        for (const c of cases) {
            it(c.name, () => {
                const tags = replace(goodLayout(), inBox(102, c.size))
                const result = assessPlacement(tags, FRAME, TARGET)
                const issue = result.issues.find(i => i.tagId === 102)
                expect(issue?.severity ?? null).toBe(c.sev)
                expect(result.canCalibrate).toBe(c.sev !== 'blocking')
            })
        }
    })

    describe('steep viewing angle', () => {
        function squashed(id: 101, ratio: number): DetectedTagInfo {
            const t = inBox(id, 80)
            const cy = GUIDE[id][1]
            const squash = t.corners.map(([x, y]) => [x, cy + (y - cy) * ratio] as [number, number])
            return { ...t, corners: squash }
        }
        it('warns when moderately foreshortened', () => {
            const tags = replace(goodLayout(), squashed(101, 0.5))
            const issue = assessPlacement(tags, FRAME, TARGET).issues.find(i => i.tagId === 101)
            expect(issue?.severity).toBe('warning')
            expect(issue?.message).toContain('steep angle')
        })
        it('blocks when extremely foreshortened', () => {
            const tags = replace(goodLayout(), squashed(101, 0.25))
            const result = assessPlacement(tags, FRAME, TARGET)
            expect(result.issues.find(i => i.tagId === 101)?.severity).toBe('blocking')
            expect(result.canCalibrate).toBe(false)
        })
    })

    describe('rotation (Center tag only, tip only)', () => {
        it.each([30, 90, 180])('%i degrees is a non-blocking tip', deg => {
            const tags = replace(goodLayout(), inBox(100, 60, deg))
            const result = assessPlacement(tags, FRAME, TARGET)
            const issue = result.issues.find(i => i.tagId === 100)
            expect(issue?.tip).toBe(true)
            expect(issue?.severity).toBe('warning')
            expect(issue?.message).toContain('Tip:')
            expect(result.canCalibrate).toBe(true)
        })
        it('keeps a rotated Center tag inside its box green', () => {
            const result = assessPlacement(replace(goodLayout(), inBox(100, 60, 90)), FRAME, TARGET)
            expect(result.tags.find(t => t.id === 100)?.severity).toBe('ok')
        })
        it('does not care how corner tags are turned', () => {
            let tags = replace(goodLayout(), inBox(101, 60, 90))
            tags = replace(tags, inBox(103, 60, 180))
            const result = assessPlacement(tags, FRAME, TARGET)
            expect(result.issues).toEqual([])
            expect(result.canCalibrate).toBe(true)
        })
    })

    it('colours a tag amber when it is outside its box and red only when blocked', () => {
        const near = replace(goodLayout(), tagAt(102, GUIDE[102][0] + 80, GUIDE[102][1]))
        expect(assessPlacement(near, FRAME, TARGET).tags.find(t => t.id === 102)?.severity).toBe(
            'warning'
        )
        const tiny = replace(goodLayout(), inBox(102, 10))
        expect(assessPlacement(tiny, FRAME, TARGET).tags.find(t => t.id === 102)?.severity).toBe(
            'blocking'
        )
    })

    it('lists blocking issues first and tips last', () => {
        let tags = replace(goodLayout(), inBox(100, 60, 90))
        tags = replace(tags, inBox(101, 15))
        const issues = assessPlacement(tags, FRAME, TARGET).issues
        expect(issues[0].severity).toBe('blocking')
        expect(issues[issues.length - 1].tip).toBe(true)
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
        expect(spread?.message).toContain('boxes')
        expect(result.canCalibrate).toBe(true)
    })

    it('warns about a tag hugging the edge of the frame', () => {
        const tags = replace(goodLayout(), tagAt(104, 22, GUIDE[104][1]))
        const issue = assessPlacement(tags, FRAME, TARGET).issues.find(
            i => i.tagId === 104 && i.message.includes('edge')
        )
        expect(issue?.severity).toBe('warning')
    })

    it('treats a detection without four corners as missing', () => {
        const broken = goodLayout().map(t => (t.id === 100 ? { ...t, corners: [] } : t))
        const result = assessPlacement(broken, FRAME, TARGET)
        expect(result.tags.find(t => t.id === 100)?.found).toBe(false)
        expect(result.canCalibrate).toBe(false)
    })
})
