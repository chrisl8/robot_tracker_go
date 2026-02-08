import { describe, it, expect } from 'vitest'
import type { Track, RobotStatus, TrackState } from '../robot'

describe('Robot Types', () => {
  describe('TrackState', () => {
    it('should accept valid states', () => {
      const states: TrackState[] = ['tentative', 'confirmed', 'deleted']

      expect(states).toContain('tentative')
      expect(states).toContain('confirmed')
      expect(states).toContain('deleted')
    })
  })

  describe('Track', () => {
    it('should create a valid track object', () => {
      const track: Track = {
        trackId: 1,
        bbox: [100, 100, 50, 50],
        confidence: 0.95,
        tagId: 42,
        state: 'confirmed'
      }

      expect(track.trackId).toBe(1)
      expect(track.bbox).toEqual([100, 100, 50, 50])
      expect(track.confidence).toBe(0.95)
      expect(track.tagId).toBe(42)
      expect(track.state).toBe('confirmed')
    })

    it('should handle tracks without tag ID', () => {
      const track: Track = {
        trackId: 2,
        bbox: [0, 0, 30, 30],
        confidence: 0.75,
        tagId: undefined,
        state: 'tentative'
      }

      expect(track.tagId).toBeUndefined()
    })
  })

  describe('RobotStatus', () => {
    it('should create a valid status object', () => {
      const status: RobotStatus = {
        fps: 30,
        arduinoState: 'connected',
        robotX: 1.5,
        robotY: 2.0,
        robotTheta: 0.5
      }

      expect(status.fps).toBe(30)
      expect(status.arduinoState).toBe('connected')
      expect(status.robotX).toBe(1.5)
      expect(status.robotY).toBe(2.0)
      expect(status.robotTheta).toBe(0.5)
    })

    it('should handle disconnected state', () => {
      const status: RobotStatus = {
        fps: 0,
        arduinoState: 'disconnected',
        robotX: 0,
        robotY: 0,
        robotTheta: 0
      }

      expect(status.arduinoState).toBe('disconnected')
    })
  })
})
