import { ref } from 'vue'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

describe('VIDEO-MAX-SIZE-001: Footprint Radius Scaling Bug', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any
    let parentElementMock: any

    beforeEach(() => {
        setActivePinia(createPinia())
        vi.clearAllMocks()
        vi.resetModules()
        vi.unstubAllGlobals()

        mockCtx = {
            clearRect: vi.fn(),
            beginPath: vi.fn(),
            arc: vi.fn(),
            fill: vi.fn(),
            stroke: vi.fn(),
            fillStyle: '',
            strokeStyle: '',
            lineWidth: 1,
            setLineDash: vi.fn(),
            strokeRect: vi.fn(),
            fillRect: vi.fn(),
            moveTo: vi.fn(),
            lineTo: vi.fn(),
            closePath: vi.fn(),
            font: '',
            textAlign: '',
            textBaseline: '',
            fillText: vi.fn(),
            measureText: vi.fn(() => ({ width: 50 })),
        }

        parentElementMock = {
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 1920, height: 1080 })),
        }

        mockCanvas = {
            width: 1920,
            height: 1080,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 1920, height: 1080 })),
            parentElement: parentElementMock,
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 640,
            clientHeight: 480,
            getBoundingClientRect: vi.fn(() => ({ left: 640, top: 300, width: 640, height: 480 })),
        }
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('should render footprint at correct position with correct radius', async () => {
        const mockGetElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas
            if (id === 'video') return mockVideo
            return null
        })

        vi.stubGlobal('document', {
            getElementById: mockGetElementById,
        })

        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = true

        robotStore.setTracks([
            {
                id: 1,
                bbox: [320, 240, 320, 240],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 25,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize } = useCanvas(canvasRef)

        initialize()

        expect(mockCtx.clearRect).toHaveBeenCalled()
        expect(mockCtx.arc).toHaveBeenCalled()
        const arcCalls = mockCtx.arc.mock.calls
        expect(arcCalls.length).toBeGreaterThan(0)
        const radius = arcCalls[0][2]
        expect(radius).toBe(25)
    })
})
