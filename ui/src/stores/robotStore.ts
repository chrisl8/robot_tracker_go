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
    // The tracker hands out a NEW track id after a track loss, but the robot's
    // AprilTag id is stable, so the selection follows the tag.
    const selectedTagId = ref<number | null>(null)
    const destinationMode = ref(false)
    const destination = ref<Destination | null>(null)
    const paths = ref<PathMessage[]>([])
    const controlMode = ref<'hold' | 'manual' | 'autonomous'>('hold')
    const emergencyStopped = ref(false)

    // Computed
    const confirmedTracks = computed(() => {
        if (!Array.isArray(tracks.value)) {
            return []
        }
        return tracks.value.filter(t => t.state === 'confirmed')
    })

    const configuredRobots = computed(() => confirmedTracks.value.filter(t => t.configured))

    const detectedTags = computed(() =>
        confirmedTracks.value.filter(t => !t.configured && t.tag_id !== undefined)
    )

    const trackCount = computed(() => tracks.value.length)

    const confirmedCount = computed(() => configuredRobots.value.length)

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
                    remapSelection()
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
                    uiStore.setCalibrationState(
                        data.calibration.state,
                        data.calibration.message,
                        data.calibration.resolutionMismatch === true
                    )
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
            case 'destination_clear':
                setDestination(null)
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
        remapSelection()
    }

    // Keep the selection on the same robot when its track id changes.
    function remapSelection(): void {
        if (selectedTrackId.value === null) return

        const current = tracks.value.find(t => t.id === selectedTrackId.value)
        if (current) {
            if (current.tag_id !== undefined) selectedTagId.value = current.tag_id
            return
        }

        // Selected track is gone. If its robot is tracked again under a new id, follow it;
        // otherwise keep the selection so it remaps when the robot reappears.
        if (selectedTagId.value === null) return
        const successor = tracks.value.find(
            t => t.state === 'confirmed' && t.tag_id === selectedTagId.value
        )
        if (successor) selectedTrackId.value = successor.id
    }

    function setTracks(newTracks: Track[]): void {
        tracks.value = newTracks
        remapSelection()
    }

    function setStatus(newStatus: RobotStatus): void {
        status.value = newStatus
    }

    function clearTracks(): void {
        tracks.value = []
    }

    function selectTrack(id: number): void {
        selectedTrackId.value = id
        selectedTagId.value = tracks.value.find(t => t.id === id)?.tag_id ?? null
        destinationMode.value = true
    }

    function clearSelection(): void {
        selectedTrackId.value = null
        selectedTagId.value = null
        destinationMode.value = false
    }

    function cancelDestinationMode(): void {
        destinationMode.value = false
    }

    // Every destination failure is shown: a toast and an activity-log entry.
    function reportDestinationFailure(message: string): false {
        const uiStore = useUIStore()
        uiStore.showToast(message, 'error', 6000)
        uiStore.addLogEntry('error', message)
        return false
    }

    async function serverErrorMessage(response: Response): Promise<string> {
        try {
            const body = (await response.json()) as { error?: unknown }
            if (typeof body.error === 'string' && body.error !== '') return body.error
        } catch {
            // Body was not JSON; fall through to the generic message.
        }
        return `The tracker service rejected the destination (HTTP ${response.status})`
    }

    async function confirmDestination(canvasX: number, canvasY: number): Promise<boolean> {
        if (selectedTrackId.value === null) {
            return reportDestinationFailure('Click the robot first, then click where it should go')
        }

        // The selected track id can be stale after a track loss; fall back to the robot's tag.
        const track =
            tracks.value.find(t => t.id === selectedTrackId.value) ??
            tracks.value.find(t => t.state === 'confirmed' && t.tag_id === selectedTagId.value)
        if (!track) {
            const who = selectedTagId.value === null ? 'The robot' : `Robot ${selectedTagId.value}`
            return reportDestinationFailure(
                `${who} is not visible right now; wait for it to be detected, then try again`
            )
        }
        if (track.tag_id === undefined) {
            return reportDestinationFailure(
                'That object has no AprilTag, so it cannot be sent anywhere'
            )
        }
        const robotId = track.tag_id

        // Use the SAME canvasToNatural function as useCanvas.ts for consistency
        const naturalCoords = canvasToNaturalShared(canvasX, canvasY)

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
                const uiStore = useUIStore()
                uiStore.addLogEntry('success', 'Destination set for robot ' + robotId)
                return true
            }
            return reportDestinationFailure(await serverErrorMessage(response))
        } catch (error) {
            console.error('Failed to set destination:', error)
            return reportDestinationFailure('Could not reach the tracker service')
        }
    }

    function setDestination(dest: Destination | null): void {
        destination.value = dest
    }

    async function clearDestination(): Promise<boolean> {
        const uiStore = useUIStore()
        const robotId = destination.value?.robot_id
        if (robotId === undefined) {
            return false
        }
        try {
            const response = await fetch('/api/destination', {
                method: 'DELETE',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ robot_id: robotId }),
            })
            if (response.ok) {
                destination.value = null
                uiStore.addLogEntry('info', 'Goal cleared for robot ' + robotId)
                return true
            }
            return false
        } catch (error) {
            console.error('Failed to clear destination:', error)
            return false
        }
    }

    function clearPaths(): void {
        paths.value = []
    }

    async function setControlMode(mode: 'hold' | 'manual' | 'autonomous'): Promise<boolean> {
        const uiStore = useUIStore()
        try {
            const response = await fetch('/api/mode', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ mode }),
            })
            if (response.ok) {
                controlMode.value = mode
                uiStore.addLogEntry('info', 'Mode changed to ' + mode)
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
        const uiStore = useUIStore()
        try {
            const response = await fetch('/api/emergency-stop', {
                method: 'POST',
            })
            if (response.ok) {
                emergencyStopped.value = true
                controlMode.value = 'hold'
                uiStore.addLogEntry('error', 'Emergency stop activated')
                return true
            }
            return false
        } catch (error) {
            console.error('Failed to activate emergency stop:', error)
            return false
        }
    }

    async function clearEmergencyStop(): Promise<boolean> {
        const uiStore = useUIStore()
        try {
            const response = await fetch('/api/clear-emergency-stop', {
                method: 'POST',
            })
            if (response.ok) {
                emergencyStopped.value = false
                uiStore.addLogEntry('warning', 'Emergency stop cleared')
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
        selectedTagId,
        destinationMode,
        destination,
        paths,
        controlMode,
        emergencyStopped,
        // Computed
        confirmedTracks,
        configuredRobots,
        detectedTags,
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
        clearDestination,
        clearPaths,
        setControlMode,
        emergencyStop,
        clearEmergencyStop,
        fetchControlState,
    }
})
