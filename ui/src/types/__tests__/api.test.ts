import { describe, it, expect } from 'vitest'
import type { CalibrationState, CalibrationMessage, ObstaclesMessage, TracksMessage, StatusMessage } from '../api'

describe('API Types', () => {
  describe('CalibrationState', () => {
    it('should accept valid states', () => {
      const state1: CalibrationState = 'not_calibrated'
      const state2: CalibrationState = 'calibrating'
      const state3: CalibrationState = 'calibrated'

      expect(state1).toBe('not_calibrated')
      expect(state2).toBe('calibrating')
      expect(state3).toBe('calibrated')
    })
  })

  describe('CalibrationMessage', () => {
    it('should validate calibration message structure', () => {
      const message: CalibrationMessage = {
        type: 'calibration',
        state: 'calibrated',
        message: 'Calibration complete'
      }

      expect(message.type).toBe('calibration')
      expect(message.state).toBe('calibrated')
    })
  })

  describe('ObstaclesMessage', () => {
    it('should validate obstacles message structure', () => {
      const message: ObstaclesMessage = {
        type: 'obstacles',
        obstacles: [
          { id: '1', x: 100, y: 100, width: 50, height: 50 }
        ]
      }

      expect(message.type).toBe('obstacles')
      expect(message.obstacles).toHaveLength(1)
    })
  })

  describe('TracksMessage', () => {
    it('should validate tracks message structure', () => {
      const message: TracksMessage = {
        type: 'tracks',
        tracks: [
          {
            trackId: 1,
            bbox: [100, 100, 50, 50],
            confidence: 0.95,
            tagId: 42,
            state: 'confirmed'
          }
        ]
      }

      expect(message.type).toBe('tracks')
      expect(message.tracks).toHaveLength(1)
      expect(message.tracks[0].confidence).toBe(0.95)
    })
  })

  describe('StatusMessage', () => {
    it('should validate status message structure', () => {
      const message: StatusMessage = {
        type: 'status',
        fps: 30,
        arduinoState: 'connected',
        robotX: 1.5,
        robotY: 2.0,
        robotTheta: 0.5
      }

      expect(message.type).toBe('status')
      expect(message.fps).toBe(30)
      expect(message.arduinoState).toBe('connected')
    })
  })
})
