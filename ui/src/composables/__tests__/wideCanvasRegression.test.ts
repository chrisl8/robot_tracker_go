import { ref } from 'vue'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

describe('WIDE-CANVAS-001: Wide Canvas Coordinate Conversion Bug', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any

    beforeEach(() => {
        vi.resetModules()
        setActivePinia(createPinia())
        vi.clearAllMocks()

        mockCtx = {
            clearRect: vi.fn(),
            beginPath: vi.fn(),
            arc: vi.fn(() => true),
            fill: vi.fn(() => true),
            stroke: vi.fn(() => true),
            fillStyle: '',
            strokeStyle: '',
            lineWidth: 1,
            setLineDash: vi.fn(),
            strokeRect: vi.fn(() => true),
            fillRect: vi.fn(() => true),
            moveTo: vi.fn(),
            lineTo: vi.fn(),
            closePath: vi.fn(() => true),
            font: '',
            textAlign: '',
            textBaseline: '',
            fillText: vi.fn(() => true),
            measureText: vi.fn(() => ({ width: 50 })),
        }

        mockCanvas = {
            width: 1200,
            height: 600,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 1200, height: 600 })),
            parentElement: {
                getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 1200, height: 600 })),
            },
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 640,
            clientHeight: 480,
            getBoundingClientRect: vi.fn(() => ({ left: 280, top: 60, width: 640, height: 480 })),
        }

        document.getElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas
            if (id === 'video') return mockVideo
            return null
        })
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('should account for video offset when rendering footprint at center of video', async () => {
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
                pixel_radius: 40,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        render()

        expect(mockCtx.arc).toHaveBeenCalled()
        const arcCall = mockCtx.arc.mock.calls[0]
        const centerX = arcCall[0]
        const centerY = arcCall[1]

        expect(centerX).toBe(280 + 320)
        expect(centerY).toBe(60 + 240)
    })

    it('should use video offset when canvas is wider than video', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = true

        robotStore.setTracks([
            {
                id: 1,
                bbox: [100, 100, 100, 100],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 20,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize, canvasToNatural } = useCanvas(canvasRef)

        initialize()

        const arcCall = mockCtx.arc.mock.calls[0]
        const centerX = arcCall[0]
        const centerY = arcCall[1]

        expect(centerX).toBe(280 + 100)
        expect(centerY).toBe(60 + 100)

        const naturalCenter = canvasToNatural(centerX, centerY)
        expect(naturalCenter.x).toBeCloseTo(100, 0)
        expect(naturalCenter.y).toBeCloseTo(100, 0)
    })

    it('should render drawing box at correct position when canvas is wide', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useObstacleStore } = await import('@/stores/obstacleStore')

        const obstacleStore = useObstacleStore()
        obstacleStore.setDrawingMode(true)

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        obstacleStore.startDrawing({ x: 350, y: 250 })
        render()

        expect(mockCtx.strokeRect).toHaveBeenCalled()
        const strokeRectCall = mockCtx.strokeRect.mock.calls[0]
        const x = strokeRectCall[0]
        const y = strokeRectCall[1]

        expect(x).toBe(350)
        expect(y).toBe(250)
    })
})

describe('WIDE-CANVAS-002: Tall Canvas Coordinate Conversion Bug', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any

    beforeEach(() => {
        vi.resetModules()
        setActivePinia(createPinia())
        vi.clearAllMocks()

        mockCtx = {
            clearRect: vi.fn(),
            beginPath: vi.fn(),
            arc: vi.fn(() => true),
            fill: vi.fn(() => true),
            stroke: vi.fn(() => true),
            fillStyle: '',
            strokeStyle: '',
            lineWidth: 1,
            setLineDash: vi.fn(),
            strokeRect: vi.fn(() => true),
            fillRect: vi.fn(() => true),
            moveTo: vi.fn(),
            lineTo: vi.fn(),
            closePath: vi.fn(() => true),
            font: '',
            textAlign: '',
            textBaseline: '',
            fillText: vi.fn(() => true),
            measureText: vi.fn(() => ({ width: 50 })),
        }

        mockCanvas = {
            width: 800,
            height: 800,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 800 })),
            parentElement: {
                getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 800 })),
            },
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 640,
            clientHeight: 480,
            getBoundingClientRect: vi.fn(() => ({ left: 80, top: 160, width: 640, height: 480 })),
        }

        document.getElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas
            if (id === 'video') return mockVideo
            return null
        })
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('should account for video offset when canvas is taller than video', async () => {
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
                pixel_radius: 30,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize } = useCanvas(canvasRef)

        initialize()

        expect(mockCtx.arc).toHaveBeenCalled()
        const arcCall = mockCtx.arc.mock.calls[0]
        const centerX = arcCall[0]
        const centerY = arcCall[1]

        expect(centerX).toBe(80 + 320)
        expect(centerY).toBe(160 + 240)
    })
})

describe('WIDE-CANVAS-003: Video Max-Width Scaling Bug', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any
    let parentElementMock: any

    beforeEach(() => {
        vi.resetModules()
        setActivePinia(createPinia())
        vi.clearAllMocks()

        mockCtx = {
            clearRect: vi.fn(),
            beginPath: vi.fn(() => true),
            arc: vi.fn(() => true),
            fill: vi.fn(() => true),
            stroke: vi.fn(() => true),
            fillStyle: '',
            strokeStyle: '',
            lineWidth: 1,
            setLineDash: vi.fn(),
            strokeRect: vi.fn(() => true),
            fillRect: vi.fn(() => true),
            moveTo: vi.fn(),
            lineTo: vi.fn(),
            closePath: vi.fn(() => true),
            font: '',
            textAlign: '',
            textBaseline: '',
            fillText: vi.fn(() => true),
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

        document.getElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas
            if (id === 'video') return mockVideo
            return null
        })
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('should NOT scale footprint radius when canvas is larger than video natural size', async () => {
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
                pixel_radius: 40,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize } = useCanvas(canvasRef)

        initialize()

        expect(parentElementMock.getBoundingClientRect).toHaveBeenCalled()
        expect(mockCtx.arc).toHaveBeenCalled()
    })

    it('should keep same radius when video is at natural size (scale=1)', async () => {
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

        expect(mockCtx.arc).toHaveBeenCalled()
        const arcCall = mockCtx.arc.mock.calls[0]
        const radius = arcCall[2]

        expect(radius).toBe(25)
    })

    it('should use natural video size for radius calculation, not canvas size', async () => {
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
                pixel_radius: 30,
            },
        ])

        const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
        const { initialize } = useCanvas(canvasRef)

        initialize()

        expect(mockCtx.arc).toHaveBeenCalled()
        const arcCall = mockCtx.arc.mock.calls[0]
        const radius = arcCall[2]

        expect(radius).toBe(30)
        expect(radius).not.toBeGreaterThan(40)
    })
})
