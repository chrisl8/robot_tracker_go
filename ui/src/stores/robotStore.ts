import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Track, RobotStatus, WebSocketMessage, Destination } from '@/types/api'
import { canvasToNatural, naturalToCanvas } from '@/utils/coordinates'
import { useUIStore } from './uiStore'
import { useObstacleStore } from './obstacleStore'

export const useRobotStore = defineStore('robot', () => {
    // State
    const tracks = ref<Track[]>([])
    const status = ref<RobotStatus>({
        connected: false,
        fps: 0,
        robotCount: 0,
        arduinoState: 'Disconnected'
    })
    const selectedTrackId = ref<number | null>(null)
    const destinationMode = ref(false)
    const destination = ref<Destination | null>(null)

    // Computed
    const confirmedTracks = computed(() => {
        if (!Array.isArray(tracks.value)) {
            return []
        }
        return tracks.value.filter(t => t.state === 'confirmed')
    })

    const trackCount = computed(() => tracks.value.length)

    const confirmedCount = computed(() => confirmedTracks.value.length)

    const selectedTrack = computed(() => {
        if (selectedTrackId.value === null) return null
        return tracks.value.find(t => t.id === selectedTrackId.value) || null
    })

    // Actions
    function handleWebSocketMessage(data: WebSocketMessage): void {
        const uiStore = useUIStore()
        const obstacleStore = useObstacleStore()
        switch (data.type) {
            case 'track':
                updateTrack(data.track)
                break
            case 'tracks':
                // Backend sends nested format: { type: 'tracks', tracks: { tracks: [...], count: 3 } }
                // Frontend expects flat format: { type: 'tracks', tracks: [...] }
                let tracksArray: Track[] | undefined
                if (Array.isArray((data as any).tracks)) {
                    // Flat format (shouldn't happen with current backend)
                    tracksArray = (data as any).tracks
                } else if ((data as any).tracks && typeof (data as any).tracks === 'object') {
                    // Nested format from backend
                    tracksArray = (data as any).tracks.tracks
                }
                if (Array.isArray(tracksArray)) {
                    tracks.value = tracksArray
                }
                break
            case 'status':
                status.value = data.status
                break
            case 'obstacles':
                if (data.obstacles && Array.isArray(data.obstacles.obstacles)) {
                    obstacleStore.setObstacles(data.obstacles.obstacles || [])
                }
                break
            case 'calibration':
                if (data.calibration) {
                    uiStore.setCalibrationState(data.calibration.state, data.calibration.message)
                }
                break
            case 'destination':
                if ((data as any).destination) {
                    const dest = (data as any).destination
                    setDestination({
                        id: `dest-${dest.robot_id}-${Date.now()}`,
                        robot_id: dest.robot_id,
                        x: dest.x,
                        y: dest.y
                    })
                }
                break
        }
    }

    function updateTrack(track: Track): void {
        const idx = tracks.value.findIndex(t => t.id === track.id)
        if (idx >= 0) {
            tracks.value[idx] = { ...tracks.value[idx], ...track }
        } else {
            tracks.value.push(track)
        }
    }

    function setTracks(newTracks: Track[]): void {
        tracks.value = newTracks
    }

    function setStatus(newStatus: RobotStatus): void {
        status.value = newStatus
    }

    function clearTracks(): void {
        tracks.value = []
    }

    function selectTrack(id: number): void {
        selectedTrackId.value = id
        destinationMode.value = true
    }

    function clearSelection(): void {
        selectedTrackId.value = null
        destinationMode.value = false
    }

    function cancelDestinationMode(): void {
        destinationMode.value = false
    }

    async function confirmDestination(canvasX: number, canvasY: number): Promise<boolean> {
        if (selectedTrackId.value === null) {
            return false
        }

        const overlay = document.getElementById('overlay') as HTMLCanvasElement | null
        const canvasWidth = overlay?.width || 640
        const canvasHeight = overlay?.height || 480

        const naturalCoords = canvasToNatural(canvasX, canvasY, canvasWidth, canvasHeight)

        try {
            const response = await fetch('/api/destination', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    robot_id: selectedTrackId.value,
                    x: naturalCoords.x,
                    y: naturalCoords.y
                })
            })

            if (response.ok) {
                const result = await response.json()
                destination.value = {
                    id: `${selectedTrackId.value}-${Date.now()}`,
                    robot_id: selectedTrackId.value,
                    x: naturalCoords.x,
                    y: naturalCoords.y
                }
                destinationMode.value = false
                return true
            }
            return false
        } catch (error) {
            console.error('Failed to set destination:', error)
            return false
        }
    }

    function setDestination(dest: Destination | null): void {
        destination.value = dest
    }

    return {
        // State
        tracks,
        status,
        selectedTrackId,
        destinationMode,
        destination,
        // Computed
        confirmedTracks,
        confirmedCount,
        trackCount,
        selectedTrack,
        // Actions
        handleWebSocketMessage,
        updateTrack,
        setTracks,
        setStatus,
        clearTracks,
        selectTrack,
        clearSelection,
        cancelDestinationMode,
        confirmDestination,
        setDestination
    }
})
