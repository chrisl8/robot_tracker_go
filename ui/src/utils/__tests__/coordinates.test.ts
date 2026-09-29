import { describe, it, expect, afterEach } from 'vitest'
import { clampToFrame, videoFrameSize } from '../coordinates'

function mountVideo(width: number, height: number): HTMLImageElement {
    const img = document.createElement('img')
    img.id = 'video'
    Object.defineProperty(img, 'naturalWidth', { value: width })
    Object.defineProperty(img, 'naturalHeight', { value: height })
    document.body.appendChild(img)
    return img
}

afterEach(() => {
    document.getElementById('video')?.remove()
})

describe('videoFrameSize', () => {
    it('is null when there is no video element', () => {
        expect(videoFrameSize()).toBeNull()
    })

    it('is null before the first frame has loaded', () => {
        mountVideo(0, 0)
        expect(videoFrameSize()).toBeNull()
    })

    it('reports the frame size once loaded', () => {
        mountVideo(1280, 720)
        expect(videoFrameSize()).toEqual({ width: 1280, height: 720 })
    })
})

describe('clampToFrame', () => {
    it('leaves a point inside the frame alone', () => {
        mountVideo(640, 480)
        expect(clampToFrame({ x: 100, y: 200 })).toEqual({ x: 100, y: 200 })
    })

    it('clamps a point dragged past every edge', () => {
        mountVideo(640, 480)
        expect(clampToFrame({ x: -20, y: -5 })).toEqual({ x: 0, y: 0 })
        expect(clampToFrame({ x: 900, y: 700 })).toEqual({ x: 640, y: 480 })
    })

    it('does not alter the point while the frame size is unknown', () => {
        expect(clampToFrame({ x: -20, y: 9999 })).toEqual({ x: -20, y: 9999 })
    })
})
