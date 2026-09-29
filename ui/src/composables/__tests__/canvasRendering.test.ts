import { ref } from 'vue'
import { createApp, defineComponent, h } from 'vue'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { createPinia, setActivePinia, type Pinia } from 'pinia'
import { useCanvas } from '@/composables/useCanvas'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { videoScale } from '@/utils/canvasTransform'
import type { Track } from '@/types/api'

// useCanvas only acquires its 2D context in onMounted, so it has to be mounted
// inside a component: called on its own, render() returns on its first line and
// draws nothing (which is why "should not throw" tests here used to prove
// nothing). The context is a recorder so tests can assert what was drawn.
function createRecordingContext() {
    return {
        clearRect: vi.fn(),
        beginPath: vi.fn(),
        arc: vi.fn(),
        fill: vi.fn(),
        stroke: vi.fn(),
        fillStyle: '' as string,
        strokeStyle: '' as string,
        lineWidth: 1,
        setLineDash: vi.fn(),
        strokeRect: vi.fn(),
        fillRect: vi.fn(),
        moveTo: vi.fn(),
        lineTo: vi.fn(),
        closePath: vi.fn(),
        save: vi.fn(),
        restore: vi.fn(),
        translate: vi.fn(),
        rotate: vi.fn(),
        font: '',
        textAlign: '',
        textBaseline: '',
        fillText: vi.fn(),
        measureText: vi.fn(() => ({ width: 50 })),
    }
}

// A 640x480 video shown at 800x600 (scale 1.25) on an 800x600 canvas.
const RECT = { left: 0, top: 0, width: 800, height: 600 }

function mountCanvas(pinia: Pinia) {
    const ctx = createRecordingContext()
    const canvas = {
        width: 800,
        height: 600,
        getContext: vi.fn(() => ctx),
        getBoundingClientRect: vi.fn(() => RECT),
        parentElement: { getBoundingClientRect: vi.fn(() => RECT) },
        addEventListener: vi.fn(),
        removeEventListener: vi.fn(),
    }
    let api!: ReturnType<typeof useCanvas>
    const Comp = defineComponent({
        setup() {
            api = useCanvas(ref(canvas as unknown as HTMLCanvasElement))
            return () => h('div')
        },
    })
    const app = createApp(Comp)
    app.use(pinia)
    app.mount(document.createElement('div'))
    ctx.clearRect.mockClear() // ignore the initial render inside onMounted
    ctx.arc.mockClear()
    return { ctx, api, unmount: () => app.unmount() }
}

function track(overrides: Partial<Track>): Track {
    return {
        id: 1,
        bbox: [100, 100, 200, 200],
        confidence: 0.95,
        tag_id: 42,
        state: 'confirmed',
        history: [],
        ...overrides,
    } as Track
}

describe('canvas rendering', () => {
    let pinia: Pinia
    let unmount: (() => void) | undefined

    beforeEach(() => {
        pinia = createPinia()
        setActivePinia(pinia)
        videoScale.value = { x: 1, y: 1, offsetX: 0, offsetY: 0 }
        vi.spyOn(document, 'getElementById').mockImplementation(id =>
            id === 'video'
                ? ({
                      videoWidth: 640,
                      videoHeight: 480,
                      clientWidth: 800,
                      clientHeight: 600,
                      getBoundingClientRect: () => RECT,
                  } as unknown as HTMLElement)
                : null
        )
    })

    afterEach(() => {
        unmount?.()
        unmount = undefined
        vi.restoreAllMocks()
    })

    function mount() {
        const m = mountCanvas(pinia)
        unmount = m.unmount
        return m
    }

    it('actually draws once mounted (harness sanity check)', () => {
        const { ctx, api } = mount()

        api.render()

        expect(ctx.clearRect).toHaveBeenCalled()
    })

    describe('robot footprints', () => {
        it('draws a circle at the scaled centre with the scaled radius', () => {
            const { ctx, api } = mount()
            useRobotStore().setTracks([track({ pixel_radius: 30 })])
            useUIStore().showFootprints = true

            api.render()

            // bbox centre (150,150) and radius 30 in video px, at 1.25x.
            expect(ctx.arc).toHaveBeenCalledWith(187.5, 187.5, 37.5, 0, Math.PI * 2)
        })

        it('draws nothing when footprints are switched off', () => {
            const { ctx, api } = mount()
            useRobotStore().setTracks([track({ pixel_radius: 30 })])
            useUIStore().showFootprints = false

            api.render()

            expect(ctx.arc).not.toHaveBeenCalledWith(
                expect.any(Number),
                expect.any(Number),
                37.5,
                0,
                Math.PI * 2
            )
        })

        it('skips a robot with no pixel radius', () => {
            const { ctx, api } = mount()
            useRobotStore().setTracks([track({ pixel_radius: 0 })])
            useUIStore().showFootprints = true

            api.render()

            expect(ctx.arc).not.toHaveBeenCalledWith(
                187.5,
                187.5,
                expect.any(Number),
                0,
                Math.PI * 2
            )
        })
    })

    describe('obstacle drawing box', () => {
        it('outlines and fills the rectangle being dragged', () => {
            const { ctx, api } = mount()
            const obstacles = useObstacleStore()
            obstacles.setDrawingMode(true) // startDrawing ignores input otherwise
            obstacles.startDrawing({ x: 100, y: 120 })
            obstacles.updateDrawing({ x: 300, y: 260 })

            api.render()

            expect(ctx.strokeRect).toHaveBeenCalledWith(100, 120, 200, 140)
            expect(ctx.fillRect).toHaveBeenCalledWith(100, 120, 200, 140)
        })

        it('draws no box when nothing is being dragged', () => {
            const { ctx, api } = mount()
            useObstacleStore().setDrawingMode(true)

            api.render()

            expect(ctx.strokeRect).not.toHaveBeenCalled()
        })
    })

    describe('calibration tags', () => {
        it('labels a detected target tag with its role and id', () => {
            const { ctx, api } = mount()
            const ui = useUIStore()
            ui.setDetectedTags([
                {
                    id: 100,
                    corners: [
                        [100, 100],
                        [200, 100],
                        [200, 200],
                        [100, 200],
                    ],
                    center: [150, 150],
                },
            ])
            ui.setCalibrationTarget({
                tagSize: 0.15,
                tags: [{ id: 100, label: 'Center', role: 'center', guideX: 0.5, guideY: 0.5 }],
            })
            ui.openCalibration()

            api.render()

            expect(ctx.fillText).toHaveBeenCalledWith(
                'Center #100',
                expect.any(Number),
                expect.any(Number)
            )
        })

        it('draws no tag labels while the calibration panel is closed', () => {
            const { ctx, api } = mount()
            const ui = useUIStore()
            ui.setDetectedTags([
                {
                    id: 100,
                    corners: [
                        [100, 100],
                        [200, 100],
                        [200, 200],
                        [100, 200],
                    ],
                    center: [150, 150],
                },
            ])
            ui.setCalibrationTarget({
                tagSize: 0.15,
                tags: [{ id: 100, label: 'Center', role: 'center', guideX: 0.5, guideY: 0.5 }],
            })

            api.render()

            expect(ctx.fillText).not.toHaveBeenCalledWith(
                'Center #100',
                expect.anything(),
                expect.anything()
            )
        })
    })
})
