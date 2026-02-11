import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type {
    Track,
    RobotStatus,
    WebSocketMessage,
    Destination,
    TracksNestedResponse,
    PathMessage,
} from '@/types/api'
import { canvasToNaturalShared } from '@/composables/useCanvas'
import { useUIStore } from './uiStore'
import { useObstacleStore } from './obstacleStore'

export const useRobotStore = defineStore('robot', () => {
    // State
    const tracks = ref<Track[]>([])
    const status = ref<RobotStatus>({
        connected: false,
        fps: 0,
        robotCount: 0,
        arduinoState: 'Disconnected',
    })
    const selectedTrackId = ref<number | null>(null)
    const destinationMode = ref(false)
    const destination = ref<Destination | null>(null)
    const paths = ref<PathMessage[]>([])
    const controlMode = ref<'idle' | 'manual' | 'autonomous'>('idle')
    const emergencyStopped = ref(false)

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
            case 'tracks': {
                const nestedTracks = (data as { tracks: TracksNestedResponse }).tracks
                if (nestedTracks && Array.isArray(nestedTracks.tracks)) {
                    tracks.value = nestedTracks.tracks
                }
                break
            }
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
                if (data.destination) {
                    const dest = data.destination
                    setDestination({
                        id: `dest-${dest.robot_id}-${Date.now()}`,
                        robot_id: dest.robot_id,
                        x: dest.x,
                        y: dest.y,
                    })
                }
                break
            case 'paths':
                if (data.paths && Array.isArray(data.paths.paths)) {
                    paths.value = data.paths.paths
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

        // Look up the selected track to get its AprilTag ID (used by the planner)
        const track = tracks.value.find(t => t.id === selectedTrackId.value)
        if (!track || track.tag_id === undefined) {
            console.error('[DEST] Selected track has no tag_id, cannot set destination')
            return false
        }
        const robotId = track.tag_id

        // Use the SAME canvasToNatural function as useCanvas.ts for consistency
        const naturalCoords = canvasToNaturalShared(canvasX, canvasY)

        console.log('[DEST DEBUG] Click (canvas):', { canvasX, canvasY })
        console.log('[DEST DEBUG] Stored (natural):', naturalCoords)
        console.log('[DEST DEBUG] Robot tag_id:', robotId)

        try {
            const response = await fetch('/api/destination', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    robot_id: robotId,
                    x: naturalCoords.x,
                    y: naturalCoords.y,
                }),
            })

            if (response.ok) {
                await response.json()
                destination.value = {
                    id: `${robotId}-${Date.now()}`,
                    robot_id: robotId,
                    x: naturalCoords.x,
                    y: naturalCoords.y,
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

    function clearPaths(): void {
        paths.value = []
    }

    async function setControlMode(mode: 'idle' | 'manual' | 'autonomous'): Promise<boolean> {
        try {
            const response = await fetch('/api/mode', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ mode }),
            })
            if (response.ok) {
                controlMode.value = mode
                return true
            }
            const data = await response.json()
            console.error('Failed to set mode:', data.error)
            return false
        } catch (error) {
            console.error('Failed to set mode:', error)
            return false
        }
    }

    async function emergencyStop(): Promise<boolean> {
        try {
            const response = await fetch('/api/emergency-stop', {
                method: 'POST',
            })
            if (response.ok) {
                emergencyStopped.value = true
                controlMode.value = 'idle'
                return true
            }
            return false
        } catch (error) {
            console.error('Failed to activate emergency stop:', error)
            return false
        }
    }

    async function clearEmergencyStop(): Promise<boolean> {
        try {
            const response = await fetch('/api/clear-emergency-stop', {
                method: 'POST',
            })
            if (response.ok) {
                emergencyStopped.value = false
                return true
            }
            return false
        } catch (error) {
            console.error('Failed to clear emergency stop:', error)
            return false
        }
    }

    async function fetchControlState(): Promise<void> {
        try {
            const response = await fetch('/api/control-state')
            if (response.ok) {
                const data = await response.json()
                controlMode.value = data.mode
                emergencyStopped.value = data.emergency_stopped
            }
        } catch {
            // ignore fetch errors
        }
    }

    return {
        // State
        tracks,
        status,
        selectedTrackId,
        destinationMode,
        destination,
        paths,
        controlMode,
        emergencyStopped,
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
        setDestination,
        clearPaths,
        setControlMode,
        emergencyStop,
        clearEmergencyStop,
        fetchControlState,
    }
})
