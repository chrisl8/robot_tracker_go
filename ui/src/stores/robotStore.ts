import { defineStore } from 'pinia'
import { ref, computed } from 'vue'
import type { Track, RobotStatus, WebSocketMessage } from '@/types/api'
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

    // Computed
    const confirmedTracks = computed(() =>
        tracks.value.filter(t => t.state === 'confirmed')
    )

    const trackCount = computed(() => tracks.value.length)

    const confirmedCount = computed(() => confirmedTracks.value.length)

    // Actions
    function handleWebSocketMessage(data: WebSocketMessage): void {
        const uiStore = useUIStore()
        const obstacleStore = useObstacleStore()
        switch (data.type) {
            case 'track':
                updateTrack(data.track)
                break
            case 'tracks':
                tracks.value = data.tracks
                break
            case 'status':
                status.value = data.status
                break
            case 'obstacles':
                if (data.obstacles) {
                    obstacleStore.setObstacles(data.obstacles.obstacles || [])
                }
                break
            case 'calibration':
                if (data.calibration) {
                    uiStore.setCalibrationState(data.calibration.state, data.calibration.message)
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

    return {
        // State
        tracks,
        status,
        // Computed
        confirmedTracks,
        trackCount,
        confirmedCount,
        // Actions
        handleWebSocketMessage,
        updateTrack,
        setTracks,
        setStatus,
        clearTracks
    }
})
