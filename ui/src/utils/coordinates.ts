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

export function canvasToNatural(
    canvasX: number,
    canvasY: number,
    canvasWidth: number,
    canvasHeight: number
): { x: number; y: number } {
    const dims = getVideoDimensions(canvasWidth, canvasHeight)
    if (dims.naturalWidth === 0 || dims.canvasWidth === 0) {
        return { x: Math.round(canvasX), y: Math.round(canvasY) }
    }
    return {
        x: Math.round(canvasX * (dims.naturalWidth / dims.canvasWidth)),
        y: Math.round(canvasY * (dims.naturalHeight / dims.canvasHeight)),
    }
}

export function naturalToCanvas(
    naturalX: number,
    naturalY: number,
    canvasWidth: number,
    canvasHeight: number
): { x: number; y: number } {
    const dims = getVideoDimensions(canvasWidth, canvasHeight)
    if (dims.naturalWidth === 0 || dims.canvasWidth === 0) {
        return { x: naturalX, y: naturalY }
    }
    return {
        x: naturalX * (dims.canvasWidth / dims.naturalWidth),
        y: naturalY * (dims.canvasHeight / dims.naturalHeight),
    }
}
