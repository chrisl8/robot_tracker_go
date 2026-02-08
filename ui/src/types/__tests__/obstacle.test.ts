import { describe, it, expect } from 'vitest'
import type { Obstacle, DrawState, Point } from '../obstacle'

describe('Obstacle Types', () => {
  describe('Obstacle', () => {
    it('should create a valid obstacle object', () => {
      const obstacle: Obstacle = {
        id: 'obs-1',
        name: 'Test Obstacle',
        pixel_top_left: [100, 100],
        pixel_bottom_right: [150, 150],
        world_top_left: [0, 0],
        world_bottom_right: [1, 1],
        clearance: 0.5
      }

      expect(obstacle.id).toBe('obs-1')
      expect(obstacle.pixel_top_left).toEqual([100, 100])
      expect(obstacle.pixel_bottom_right).toEqual([150, 150])
    })

    it('should calculate center point', () => {
      const obstacle: Obstacle = {
        id: 'obs-1',
        name: 'Test Obstacle',
        pixel_top_left: [100, 100],
        pixel_bottom_right: [150, 150],
        world_top_left: [0, 0],
        world_bottom_right: [1, 1],
        clearance: 0.5
      }

      const centerX = (obstacle.pixel_top_left[0] + obstacle.pixel_bottom_right[0]) / 2
      const centerY = (obstacle.pixel_top_left[1] + obstacle.pixel_bottom_right[1]) / 2

      expect(centerX).toBe(125)
      expect(centerY).toBe(125)
    })
  })

  describe('DrawState', () => {
    it('should have correct initial state', () => {
      const state: DrawState = {
        active: false,
        startPoint: null,
        currentPoint: null,
        startTime: null
      }

      expect(state.active).toBe(false)
      expect(state.startPoint).toBeNull()
      expect(state.currentPoint).toBeNull()
    })

    it('should track drawing state', () => {
      const startPoint: Point = { x: 100, y: 100 }
      const currentPoint: Point = { x: 150, y: 150 }

      const state: DrawState = {
        active: true,
        startPoint: startPoint,
        currentPoint: currentPoint,
        startTime: Date.now()
      }

      expect(state.active).toBe(true)
      expect(state.startPoint?.x).toBe(100)
      expect(state.currentPoint?.x).toBe(150)
    })
  })

  describe('Point', () => {
    it('should store coordinates', () => {
      const point: Point = { x: 42.5, y: 78.3 }

      expect(point.x).toBe(42.5)
      expect(point.y).toBe(78.3)
    })
  })
})
