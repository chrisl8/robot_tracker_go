import { describe, it, expect } from 'vitest'
import type { Track, RobotStatus } from '../robot'

describe('Robot Types', () => {
  describe('Track', () => {
    it('should create a valid track object', () => {
      const track: Track = {
        id: 1,
        bbox: [100, 100, 50, 50],
        confidence: 0.95,
        tag_id: 42,
        state: 'confirmed',
        history: []
      }

      expect(track.id).toBe(1)
      expect(track.bbox).toEqual([100, 100, 50, 50])
      expect(track.confidence).toBe(0.95)
      expect(track.tag_id).toBe(42)
      expect(track.state).toBe('confirmed')
    })

    it('should handle tracks without tag ID', () => {
      const track: Track = {
        id: 2,
        bbox: [0, 0, 30, 30],
        confidence: 0.75,
        tag_id: undefined,
        state: 'pending',
        history: []
      }

      expect(track.tag_id).toBeUndefined()
    })
  })

  describe('RobotStatus', () => {
    it('should create a valid status object', () => {
      const status: RobotStatus = {
        connected: true,
        fps: 30,
        robotCount: 3,
        arduinoState: 'Connected'
      }

      expect(status.fps).toBe(30)
      expect(status.arduinoState).toBe('Connected')
      expect(status.connected).toBe(true)
      expect(status.robotCount).toBe(3)
    })

    it('should handle disconnected state', () => {
      const status: RobotStatus = {
        connected: false,
        fps: 0,
        robotCount: 0,
        arduinoState: 'Disconnected'
      }

      expect(status.arduinoState).toBe('Disconnected')
    })
  })
})
