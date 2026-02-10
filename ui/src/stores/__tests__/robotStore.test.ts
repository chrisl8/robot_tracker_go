import { describe, it, expect, beforeEach, vi } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import type { WebSocketMessage } from '@/types/api'

describe('Robot Store - Destination Planning', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
        vi.clearAllMocks()
    })

    describe('Destination State', () => {
        it('should have default destination mode state', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            expect(store.destinationMode).toBe(false)
            expect(store.destination).toBe(null)
            expect(store.selectedTrackId).toBe(null)
        })

        it('should have selectedTrack computed property', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            expect(store.selectedTrack).toBe(null)
        })
    })

    describe('selectTrack', () => {
        it('should set selectedTrackId and enter destination mode', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            store.selectTrack(1)

            expect(store.selectedTrackId).toBe(1)
            expect(store.destinationMode).toBe(true)
        })
    })

    describe('clearSelection', () => {
        it('should clear selection and exit destination mode', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            store.selectTrack(1)
            store.clearSelection()

            expect(store.selectedTrackId).toBe(null)
            expect(store.destinationMode).toBe(false)
        })
    })

    describe('cancelDestinationMode', () => {
        it('should exit destination mode without clearing selection', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            store.selectTrack(1)
            store.cancelDestinationMode()

            expect(store.selectedTrackId).toBe(1)
            expect(store.destinationMode).toBe(false)
        })
    })

    describe('confirmDestination', () => {
        it('should not confirm when no robot is selected', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            const result = await store.confirmDestination(100, 200)

            expect(result).toBe(false)
        })
    })

    describe('setDestination', () => {
        it('should set the destination', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            store.setDestination({
                id: 'dest-1-123',
                robot_id: 1,
                x: 100,
                y: 200,
            })

            expect(store.destination).toEqual({
                id: 'dest-1-123',
                robot_id: 1,
                x: 100,
                y: 200,
            })
        })

        it('should clear destination when null is passed', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            store.setDestination({
                id: 'dest-1-123',
                robot_id: 1,
                x: 100,
                y: 200,
            })
            store.setDestination(null)

            expect(store.destination).toBe(null)
        })
    })

    describe('WebSocket message handling', () => {
        it('should handle destination message', async () => {
            const { useRobotStore } = await import('@/stores/robotStore')
            const store = useRobotStore()

            const message: WebSocketMessage = {
                type: 'destination',
                destination: {
                    id: 'dest-1-123456',
                    robot_id: 1,
                    x: 150,
                    y: 250,
                    valid: true,
                },
            }

            store.handleWebSocketMessage(message)

            expect(store.destination).toBeDefined()
            expect(store.destination?.robot_id).toBe(1)
            expect(store.destination?.x).toBe(150)
            expect(store.destination?.y).toBe(250)
        })
    })
})

describe('Destination Types', () => {
    it('should create valid Destination object', () => {
        const destination = {
            id: 'dest-1-123456',
            robot_id: 1,
            x: 320,
            y: 240,
        }

        expect(destination.id).toBe('dest-1-123456')
        expect(destination.robot_id).toBe(1)
        expect(destination.x).toBe(320)
        expect(destination.y).toBe(240)
    })

    it('should create valid DestinationMessage object', () => {
        const message = {
            type: 'destination' as const,
            destination: {
                robot_id: 2,
                x: 640,
                y: 480,
                valid: true,
            },
        }

        expect(message.type).toBe('destination')
        expect(message.destination.valid).toBe(true)
    })
})
