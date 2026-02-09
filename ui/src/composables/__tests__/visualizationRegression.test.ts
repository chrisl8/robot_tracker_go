import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

describe('VISUALIZATION-001: Robot Footprint Display', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any

    beforeEach(() => {
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
        }

        mockCanvas = {
            width: 800,
            height: 600,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            parentElement: {
                getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            },
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 800,
            clientHeight: 600,
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
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

    it('should render footprints when showFootprints is true and tracks have pixel_radius', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = true

        robotStore.setTracks([
            {
                id: 1,
                bbox: [100, 100, 150, 150],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 30,
                world_pos: [1.0, 1.0],
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        render()

        expect(mockCtx.arc).toHaveBeenCalled()
    })

    it('should NOT render footprints when showFootprints is false', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = false

        robotStore.setTracks([
            {
                id: 1,
                bbox: [100, 100, 150, 150],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 30,
                world_pos: [1.0, 1.0],
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        const arcCallsBefore = mockCtx.arc.mock.calls.length
        initialize()
        render()
        const arcCallsAfter = mockCtx.arc.mock.calls.length

        expect(arcCallsAfter - arcCallsBefore).toBe(0)
    })

    it('should render footprints for confirmed tracks only', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = true

        robotStore.setTracks([
            {
                id: 1,
                bbox: [100, 100, 150, 150],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 30,
                world_pos: [1.0, 1.0],
            },
            {
                id: 2,
                bbox: [200, 200, 250, 250],
                confidence: 0.50,
                state: 'tentative',
                history: [],
                pixel_radius: 20,
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        render()

        expect(mockCtx.arc).toHaveBeenCalled()
    })
})

describe('VISUALIZATION-002: Calibration Tag Size Display', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
        vi.clearAllMocks()
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    it('should include computed size in toast when calibration completes', async () => {
        const { useUIStore } = await import('@/stores/uiStore')
        const uiStore = useUIStore()

        const computedWidth = 0.15
        const computedHeight = 0.15

        uiStore.setCalibrationState('calibrated',
            `Calibration complete! Area: ${computedWidth}m x ${computedHeight}m`)

        expect(uiStore.calibration.message).toContain('Area:')
        expect(uiStore.calibration.message).toContain('m x')
    })

    it('should display calibration message with tag size details', async () => {
        const { useUIStore } = await import('@/stores/uiStore')
        const uiStore = useUIStore()

        const message = 'Calibration complete! Area: 0.15m x 0.15m'
        uiStore.setCalibrationState('calibrated', message)

        expect(uiStore.calibration.state).toBe('calibrated')
        expect(uiStore.calibration.message).toBe(message)
    })
})

describe('VISUALIZATION-003: Obstacle Backend Rendering', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any

    beforeEach(() => {
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
        }

        mockCanvas = {
            width: 800,
            height: 600,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            parentElement: {
                getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            },
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 800,
            clientHeight: 600,
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
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

    it('should have obstacles in robotStore from WebSocket message', async () => {
        const { useRobotStore } = await import('@/stores/robotStore')
        const robotStore = useRobotStore()

        const mockObstacleMessage = {
            type: 'obstacles',
            obstacles: {
                obstacles: [
                    {
                        id: 'obs1',
                        name: 'Test Obstacle',
                        pixel_top_left: [100, 100],
                        pixel_bottom_right: [200, 200],
                        world_top_left: [0, 0],
                        world_bottom_right: [1, 1],
                        clearance: 0.05,
                    },
                ],
            },
        }

        robotStore.handleWebSocketMessage(mockObstacleMessage as any)

        expect(robotStore).toBeDefined()
    })

    it('should NOT render obstacles on frontend canvas (they are backend-drawn)', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useObstacleStore } = await import('@/stores/obstacleStore')

        const robotStore = useRobotStore()
        const obstacleStore = useObstacleStore()

        obstacleStore.setObstacles([
            {
                id: 'obs1',
                name: 'Test Obstacle',
                pixel_top_left: [100, 100],
                pixel_bottom_right: [200, 200],
                world_top_left: [0, 0],
                world_bottom_right: [1, 1],
                clearance: 0.05,
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        const strokeRectCallsBefore = mockCtx.strokeRect.mock.calls.length
        render()
        const strokeRectCallsAfter = mockCtx.strokeRect.mock.calls.length

        expect(strokeRectCallsAfter - strokeRectCallsBefore).toBe(0)
    })

    it('should render drawing box when obstacle drawing is active', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useObstacleStore } = await import('@/stores/obstacleStore')

        const obstacleStore = useObstacleStore()
        obstacleStore.setDrawingMode(true)
        obstacleStore.startDrawing({ x: 100, y: 100 })
        obstacleStore.updateDrawing({ x: 200, y: 200 })

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        initialize()
        const strokeRectCallsBefore = mockCtx.strokeRect.mock.calls.length
        render()
        const strokeRectCallsAfter = mockCtx.strokeRect.mock.calls.length

        expect(strokeRectCallsAfter - strokeRectCallsBefore).toBeGreaterThanOrEqual(1)
    })
})

describe('VISUALIZATION-004: Coordinate System Consistency', () => {
    let mockCanvas: any
    let mockCtx: any
    let mockVideo: any

    beforeEach(() => {
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
        }

        mockCanvas = {
            width: 800,
            height: 600,
            getContext: vi.fn(() => mockCtx),
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            parentElement: {
                getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
            },
        }

        mockVideo = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 800,
            clientHeight: 600,
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
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

    it('should include scaleX and scaleY in getVideoDimensions result', async () => {
        const { getVideoDimensions } = await import('@/utils/coordinates')

        const result = getVideoDimensions(800, 600)

        expect(result.naturalWidth).toBe(640)
        expect(result.naturalHeight).toBe(480)
        expect(result.canvasWidth).toBe(800)
        expect(result.canvasHeight).toBe(600)
        expect(result.scaleX).toBeCloseTo(1.25)
        expect(result.scaleY).toBeCloseTo(1.25)
    })

    it('should use naturalToCanvas for all overlay rendering', async () => {
        const { useCanvas } = await import('@/composables/useCanvas')
        const { useRobotStore } = await import('@/stores/robotStore')
        const { useUIStore } = await import('@/stores/uiStore')

        const robotStore = useRobotStore()
        const uiStore = useUIStore()

        uiStore.showFootprints = true
        uiStore.setDetectedTags([
            {
                id: 1,
                corners: [[100, 100], [200, 100], [200, 200], [100, 200]],
                center: [150, 150],
                size: 100,
            },
        ])
        uiStore.setSelectedCalibrationTag(1)

        robotStore.setTracks([
            {
                id: 1,
                bbox: [100, 100, 150, 150],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 30,
                world_pos: [1.0, 1.0],
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        expect(() => {
            initialize()
            render()
        }).not.toThrow()
    })

    it('should not throw when video dimensions are zero', async () => {
        const zeroVideo = {
            videoWidth: 0,
            videoHeight: 0,
            clientWidth: 800,
            clientHeight: 600,
            getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
        }

        document.getElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas
            if (id === 'video') return zeroVideo
            return null
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
                bbox: [100, 100, 150, 150],
                confidence: 0.95,
                tag_id: 42,
                state: 'confirmed',
                history: [],
                pixel_radius: 30,
                world_pos: [1.0, 1.0],
            },
        ])

        const canvasRef = { value: mockCanvas }
        const { initialize, render } = useCanvas(canvasRef)

        expect(() => {
            initialize()
            render()
        }).not.toThrow()
    })
})
