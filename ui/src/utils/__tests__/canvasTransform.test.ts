import { describe, it, expect, beforeEach } from 'vitest'
import {
    canvasToNaturalShared,
    naturalToCanvas,
    updateVideoScale,
    videoScale,
} from '@/utils/canvasTransform'

describe('canvasTransform', () => {
    beforeEach(() => {
        videoScale.value = { x: 1, y: 1, offsetX: 0, offsetY: 0 }
    })

    it('round-trips natural -> canvas -> natural with scale and offset', () => {
        videoScale.value = { x: 1.25, y: 0.5, offsetX: 40, offsetY: 10 }
        const c = naturalToCanvas(320, 240)
        expect(c).toEqual({ x: 440, y: 130 })
        expect(canvasToNaturalShared(c.x, c.y)).toEqual({ x: 320, y: 240 })
    })

    it('passes coordinates through when scale is degenerate', () => {
        videoScale.value = { x: 0, y: 0, offsetX: 5, offsetY: 5 }
        expect(naturalToCanvas(10, 20)).toEqual({ x: 10, y: 20 })
        expect(canvasToNaturalShared(10.4, 20.6)).toEqual({ x: 10, y: 21 })
    })

    it('measures scale and letterbox offset from the #video element', () => {
        const video = {
            videoWidth: 640,
            videoHeight: 480,
            clientWidth: 800,
            clientHeight: 600,
            getBoundingClientRect: () => ({ left: 100, top: 50 }),
        }
        const canvasEl = { getBoundingClientRect: () => ({ left: 60, top: 40 }) }
        document.getElementById = ((id: string) => (id === 'video' ? video : null)) as never
        updateVideoScale(canvasEl as unknown as HTMLCanvasElement)
        expect(videoScale.value).toEqual({ x: 1.25, y: 1.25, offsetX: 40, offsetY: 10 })
    })
})
