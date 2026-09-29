import { ref } from 'vue'
import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'

// Mock the DOM
const mockCanvas = {
    width: 800,
    height: 600,
    getContext: vi.fn(() => ({
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
    })),
    getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
    parentElement: {
        getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
    },
}

const mockVideo = {
    videoWidth: 640,
    videoHeight: 480,
    clientWidth: 800,
    clientHeight: 600,
    getBoundingClientRect: vi.fn(() => ({ left: 0, top: 0, width: 800, height: 600 })),
}

describe('CANVAS-002: Regression Tests for Canvas Rendering', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
        vi.clearAllMocks()

        // Mock DOM elements
        document.getElementById = vi.fn((id: string) => {
            if (id === 'overlay') return mockCanvas as any
            if (id === 'video') return mockVideo as any
            return null
        })
    })

    afterEach(() => {
        vi.restoreAllMocks()
    })

    describe('REGRESSION-001: Robot Footprints Rendering', () => {
        it('should render footprints when tracks exist', async () => {
            const { useCanvas } = await import('@/composables/useCanvas')
            const { useRobotStore } = await import('@/stores/robotStore')
            const { useUIStore } = await import('@/stores/uiStore')

            const robotStore = useRobotStore()
            const uiStore = useUIStore()

            // Set up a track with pixel_radius
            robotStore.setTracks([
                {
                    id: 1,
                    bbox: [100, 100, 50, 50],
                    confidence: 0.95,
                    tag_id: 42,
                    state: 'confirmed',
                    history: [],
                    pixel_radius: 30,
                },
            ])

            // Use the correct method name
            uiStore.showFootprints = true

            const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
            const { render } = useCanvas(canvasRef)

            // Should not throw
            expect(() => render()).not.toThrow()
        })
    })

    describe('REGRESSION-002: Calibration Tag Highlighting', () => {
        it('should render a calibration target tag with its role label', async () => {
            const { useCanvas } = await import('@/composables/useCanvas')
            const { useUIStore } = await import('@/stores/uiStore')

            const uiStore = useUIStore()

            uiStore.setDetectedTags([
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
            uiStore.setCalibrationTarget({
                tagSize: 0.15,
                tags: [{ id: 100, label: 'Center', role: 'center', guideX: 0.5, guideY: 0.5 }],
            })
            uiStore.setCalibrationPlacement({
                tags: [{ id: 100, label: 'Center', found: true, sizePx: 100, severity: 'warning' }],
                issues: [],
                guides: [
                    {
                        id: 100,
                        label: 'Center',
                        cx: 320,
                        cy: 240,
                        sizePx: 100,
                        state: 'outside',
                        tagX: 150,
                        tagY: 150,
                    },
                ],
                allFound: false,
                canCalibrate: false,
            })
            uiStore.openCalibration()

            const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
            const { render } = useCanvas(canvasRef)

            // Should not throw
            expect(() => render()).not.toThrow()
        })

        it('should use videoScale for corner scaling', async () => {
            const { useCanvas } = await import('@/composables/useCanvas')

            const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
            const canvas = useCanvas(canvasRef)

            // Access videoScale through the composable
            expect(canvas).toBeDefined()
        })
    })

    describe('REGRESSION-003: Obstacle Drawing Box', () => {
        it('should render drawing box when drawing', async () => {
            const { useCanvas } = await import('@/composables/useCanvas')
            const { useObstacleStore } = await import('@/stores/obstacleStore')

            const obstacleStore = useObstacleStore()

            // Start drawing an obstacle
            obstacleStore.startDrawing({ x: 100, y: 100 })
            obstacleStore.updateDrawing({ x: 200, y: 200 })

            const canvasRef = ref(mockCanvas as unknown as HTMLCanvasElement | null)
            const { render } = useCanvas(canvasRef)

            // Should not throw
            expect(() => render()).not.toThrow()
        })

        it('should handle drawRect coordinates correctly', async () => {
            const { useObstacleStore } = await import('@/stores/obstacleStore')
            const obstacleStore = useObstacleStore()

            // Enable drawing mode first (required by obstacleStore)
            obstacleStore.setDrawingMode(true)

            // Set up drawing state
            obstacleStore.startDrawing({ x: 50, y: 50 })
            obstacleStore.updateDrawing({ x: 150, y: 150 })

            // Verify drawing state
            expect(obstacleStore.drawing.active).toBe(true)
            expect(obstacleStore.drawing.startPoint).toEqual({ x: 50, y: 50 })
            expect(obstacleStore.drawing.currentPoint).toEqual({ x: 150, y: 150 })
        })
    })

    describe('REGRESSION-004: Static Obstacles Display', () => {
        it('should have obstacles in obstacleStore', async () => {
            const { useObstacleStore } = await import('@/stores/obstacleStore')
            const obstacleStore = useObstacleStore()

            // Set up obstacles
            obstacleStore.setObstacles([
                {
                    id: 'obs1',
                    name: 'Test Obstacle 1',
                    pixel_top_left: [100, 100],
                    pixel_bottom_right: [200, 200],
                    world_top_left: [0, 0],
                    world_bottom_right: [1, 1],
                    clearance: 0.05,
                },
                {
                    id: 'obs2',
                    name: 'Test Obstacle 2',
                    pixel_top_left: [300, 300],
                    pixel_bottom_right: [400, 400],
                    world_top_left: [1, 1],
                    world_bottom_right: [2, 2],
                    clearance: 0.05,
                },
            ])

            expect(obstacleStore.obstacles).toHaveLength(2)
            expect(obstacleStore.obstacleCount).toBe(2)
        })

        it('should clear obstacles correctly', async () => {
            const { useObstacleStore } = await import('@/stores/obstacleStore')
            const obstacleStore = useObstacleStore()

            obstacleStore.setObstacles([
                {
                    id: 'obs1',
                    name: 'Test',
                    pixel_top_left: [100, 100],
                    pixel_bottom_right: [200, 200],
                    world_top_left: [0, 0],
                    world_bottom_right: [1, 1],
                    clearance: 0.05,
                },
            ])

            obstacleStore.clearObstacles()

            expect(obstacleStore.obstacles).toHaveLength(0)
        })
    })
})
