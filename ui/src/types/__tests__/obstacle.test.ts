import { describe, it, expect } from 'vitest'
import type { Obstacle, DrawState, DrawingPoint } from '../obstacle'

describe('Obstacle Types', () => {
  describe('Obstacle', () => {
    it('should create a valid obstacle object', () => {
      const obstacle: Obstacle = {
        id: 'obs-1',
        x: 100,
        y: 100,
        width: 50,
        height: 50
      }

      expect(obstacle.id).toBe('obs-1')
      expect(obstacle.x).toBe(100)
      expect(obstacle.y).toBe(100)
      expect(obstacle.width).toBe(50)
      expect(obstacle.height).toBe(50)
    })

    it('should calculate center point', () => {
      const obstacle: Obstacle = {
        id: 'obs-1',
        x: 100,
        y: 100,
        width: 50,
        height: 50
      }

      const centerX = obstacle.x + obstacle.width / 2
      const centerY = obstacle.y + obstacle.height / 2

      expect(centerX).toBe(125)
      expect(centerY).toBe(125)
    })
  })

  describe('DrawState', () => {
    it('should have correct initial state', () => {
      const state: DrawState = {
        mode: 'none',
        start: null,
        current: null
      }

      expect(state.mode).toBe('none')
      expect(state.start).toBeNull()
      expect(state.current).toBeNull()
    })

    it('should track drawing state', () => {
      const startPoint: DrawingPoint = { x: 100, y: 100 }
      const currentPoint: DrawingPoint = { x: 150, y: 150 }

      const state: DrawState = {
        mode: 'drawing',
        start: startPoint,
        current: currentPoint
      }

      expect(state.mode).toBe('drawing')
      expect(state.start?.x).toBe(100)
      expect(state.current?.x).toBe(150)
    })
  })

  describe('DrawingPoint', () => {
    it('should store coordinates', () => {
      const point: DrawingPoint = { x: 42.5, y: 78.3 }

      expect(point.x).toBe(42.5)
      expect(point.y).toBe(78.3)
    })
  })
})
