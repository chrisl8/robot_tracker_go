// Video <-> canvas coordinate transforms, shared by the overlay canvas (useCanvas)
// and anything else that needs to turn a canvas click into natural video pixels
// (e.g. robotStore.confirmDestination). This is a leaf module — it imports no
// stores or composables — so stores can depend on it without an import cycle.
//
// "Natural" coordinates are pixels of the video frame as the backend sees it;
// "canvas" coordinates are pixels of the overlay canvas, which is letterboxed
// over the displayed video (hence the scale + offset).
import { ref } from 'vue'

// Module-level videoScale that is shared across all usages
export const videoScale = ref({ x: 1, y: 1, offsetX: 0, offsetY: 0 })

// Canvas -> natural video coordinates, using the shared module-level videoScale.
export function canvasToNaturalShared(canvasX: number, canvasY: number): { x: number; y: number } {
    const scale = videoScale.value
    if (scale.x === 0 || scale.y === 0) {
        return { x: Math.round(canvasX), y: Math.round(canvasY) }
    }
    return {
        x: Math.round((canvasX - scale.offsetX) / scale.x),
        y: Math.round((canvasY - scale.offsetY) / scale.y),
    }
}

// Natural video coordinates -> canvas coordinates (inverse of canvasToNaturalShared).
export function naturalToCanvas(naturalX: number, naturalY: number): { x: number; y: number } {
    const scale = videoScale.value
    if (scale.x === 0 || scale.y === 0) {
        return { x: naturalX, y: naturalY }
    }
    return {
        x: naturalX * scale.x + scale.offsetX,
        y: naturalY * scale.y + scale.offsetY,
    }
}

// Re-measure the displayed #video element against the overlay canvas and update the
// shared videoScale. No-op if the element is missing or has no intrinsic size yet.
export function updateVideoScale(canvasEl: HTMLCanvasElement | null): void {
    const video = document.getElementById('video') as HTMLVideoElement | HTMLImageElement | null
    if (!video) return

    const naturalWidth =
        'videoWidth' in video
            ? (video as HTMLVideoElement).videoWidth
            : 'naturalWidth' in video
              ? (video as HTMLImageElement).naturalWidth
              : 0
    const naturalHeight =
        'videoHeight' in video
            ? (video as HTMLVideoElement).videoHeight
            : 'naturalHeight' in video
              ? (video as HTMLImageElement).naturalHeight
              : 0

    if (naturalWidth === 0 || naturalHeight === 0) return

    const displayedWidth = video.clientWidth
    const displayedHeight = video.clientHeight

    const scaleX = displayedWidth / naturalWidth
    const scaleY = displayedHeight / naturalHeight

    const videoRect = video.getBoundingClientRect()
    if (canvasEl) {
        const canvasRect = canvasEl.getBoundingClientRect()
        videoScale.value = {
            x: scaleX,
            y: scaleY,
            offsetX: videoRect.left - canvasRect.left,
            offsetY: videoRect.top - canvasRect.top,
        }
    } else {
        videoScale.value = { x: scaleX, y: scaleY, offsetX: 0, offsetY: 0 }
    }
}
