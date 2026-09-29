export interface VideoDimensions {
    naturalWidth: number
    naturalHeight: number
    canvasWidth: number
    canvasHeight: number
    scaleX: number
    scaleY: number
}

export function getVideoDimensions(canvasWidth: number, canvasHeight: number): VideoDimensions {
    const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
    if (!video) {
        return {
            naturalWidth: 640,
            naturalHeight: 480,
            canvasWidth,
            canvasHeight,
            scaleX: 1,
            scaleY: 1,
        }
    }

    const naturalWidth =
        'videoWidth' in video
            ? (video as HTMLVideoElement).videoWidth
            : 'naturalWidth' in video
              ? (video as HTMLImageElement).naturalWidth
              : 640
    const naturalHeight =
        'videoHeight' in video
            ? (video as HTMLVideoElement).videoHeight
            : 'naturalHeight' in video
              ? (video as HTMLImageElement).naturalHeight
              : 480

    const scaleX = naturalWidth > 0 ? canvasWidth / naturalWidth : 1
    const scaleY = naturalHeight > 0 ? canvasHeight / naturalHeight : 1

    return { naturalWidth, naturalHeight, canvasWidth, canvasHeight, scaleX, scaleY }
}

// The displayed video's frame size in pixels, or null while it is unknown
// (no element, or the first frame has not loaded).
export function videoFrameSize(): { width: number; height: number } | null {
    const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
    if (!video) return null
    const width =
        'videoWidth' in video
            ? (video as HTMLVideoElement).videoWidth
            : (video as HTMLImageElement).naturalWidth
    const height =
        'videoHeight' in video
            ? (video as HTMLVideoElement).videoHeight
            : (video as HTMLImageElement).naturalHeight
    return width > 0 && height > 0 ? { width, height } : null
}

// Clamp a point in video pixels to the frame. Returns the point unchanged while
// the frame size is unknown (the server validates ranges as well).
export function clampToFrame(p: { x: number; y: number }): { x: number; y: number } {
    const size = videoFrameSize()
    if (!size) return p
    return {
        x: Math.min(Math.max(p.x, 0), size.width),
        y: Math.min(Math.max(p.y, 0), size.height),
    }
}
