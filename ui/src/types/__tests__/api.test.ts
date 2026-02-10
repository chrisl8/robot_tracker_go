import { describe, it, expect } from 'vitest'
import type {
    CalibrationState,
    CalibrationMessage,
    ObstaclesResponse,
    TracksMessage,
    StatusMessage,
} from '../api'

describe('API Types', () => {
    describe('CalibrationState', () => {
        it('should accept valid states', () => {
            const state1: CalibrationState = { state: 'not_calibrated' }
            const state2: CalibrationState = { state: 'calibrating' }
            const state3: CalibrationState = { state: 'calibrated' }

            expect(state1.state).toBe('not_calibrated')
            expect(state2.state).toBe('calibrating')
            expect(state3.state).toBe('calibrated')
        })
    })

    describe('CalibrationMessage', () => {
        it('should validate calibration message structure', () => {
            const message: CalibrationMessage = {
                type: 'calibration',
                calibration: { state: 'calibrated', message: 'Calibration complete' },
            }

            expect(message.type).toBe('calibration')
            expect(message.calibration.state).toBe('calibrated')
        })
    })

    describe('ObstaclesResponse', () => {
        it('should validate obstacles response structure', () => {
            const response: ObstaclesResponse = {
                obstacles: [
                    {
                        id: '1',
                        name: 'obs1',
                        pixel_top_left: [100, 100],
                        pixel_bottom_right: [150, 150],
                        world_top_left: [0, 0],
                        world_bottom_right: [1, 1],
                        clearance: 0.5,
                    },
                ],
                count: 1,
                saved: false,
            }

            expect(response.obstacles).toHaveLength(1)
            expect(response.count).toBe(1)
        })
    })

    describe('TracksMessage', () => {
        it('should validate tracks message structure', () => {
            const message: TracksMessage = {
                type: 'tracks',
                tracks: {
                    tracks: [
                        {
                            id: 1,
                            bbox: [100, 100, 50, 50],
                            confidence: 0.95,
                            tag_id: 42,
                            state: 'confirmed',
                            history: [],
                        },
                    ],
                    count: 1,
                },
            }

            expect(message.type).toBe('tracks')
            expect(message.tracks.tracks).toHaveLength(1)
            expect(message.tracks.tracks[0].confidence).toBe(0.95)
            expect(message.tracks.count).toBe(1)
        })
    })

    describe('StatusMessage', () => {
        it('should validate status message structure', () => {
            const message: StatusMessage = {
                type: 'status',
                status: {
                    connected: true,
                    fps: 30,
                    robotCount: 3,
                    arduinoState: 'Connected',
                },
            }

            expect(message.type).toBe('status')
            expect(message.status.fps).toBe(30)
            expect(message.status.arduinoState).toBe('Connected')
        })
    })
})
