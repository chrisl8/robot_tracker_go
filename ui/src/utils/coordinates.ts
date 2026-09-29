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
