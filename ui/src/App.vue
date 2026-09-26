<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { useWebSocket } from '@/composables/useWebSocket'
import VideoOverlay from '@/components/VideoOverlay.vue'
import ControlPanel from '@/components/ControlPanel.vue'
import TrackList from '@/components/TrackList.vue'
import SystemStatus from '@/components/SystemStatus.vue'
import ObstaclePanel from '@/components/ObstaclePanel.vue'
import TelemetryPanel from '@/components/TelemetryPanel.vue'
import CommStatus from '@/components/CommStatus.vue'
import ActivityLog from '@/components/ActivityLog.vue'
import CalibrationWizard from '@/components/CalibrationWizard.vue'
import ToastContainer from '@/components/ToastContainer.vue'
import BottomBar from '@/components/BottomBar.vue'

const obstacleStore = useObstacleStore()
const uiStore = useUIStore()

const streamUrl = ref('/stream')
const wasEverConnected = ref(false)

const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`
const { isConnected } = useWebSocket(wsUrl, {
    onConnect() {
        wasEverConnected.value = true
    },
    onReconnect() {
        // Bust the MJPEG stream cache to force a new HTTP connection
        streamUrl.value = `/stream?t=${Date.now()}`
        loadInitialData()
    },
})

async function loadInitialData(): Promise<void> {
    try {
        // Load obstacles
        const obstaclesResponse = await fetch('/api/obstacles')
        const obstaclesData = await obstaclesResponse.json()
        obstacleStore.setObstacles(obstaclesData.obstacles || [])

        // Load calibration status
        const calibrationResponse = await fetch('/api/calibration/status')
        const calibrationData = await calibrationResponse.json()
        uiStore.setCalibrationState(
            calibrationData.state,
            calibrationData.message,
            calibrationData.resolutionMismatch === true
        )
    } catch (e) {
        console.error('Failed to load initial data:', e)
    }
}

// Keyboard controls
function handleKeyDown(event: KeyboardEvent): void {
    const key = event.key.toLowerCase()
    const validKeys = ['w', 'a', 's', 'd', 'x', 'e', 'z', 'c']

    if (validKeys.includes(key)) {
        uiStore.setKey(key as Parameters<typeof uiStore.setKey>[0], true)

        // Prevent default for these keys
        if (['w', 'a', 's', 'd', 'x', 'e'].includes(key)) {
            event.preventDefault()
        }
    }
}

function handleKeyUp(event: KeyboardEvent): void {
    const key = event.key.toLowerCase()
    const validKeys = ['w', 'a', 's', 'd', 'x', 'e', 'z', 'c']

    if (validKeys.includes(key)) {
        uiStore.setKey(key as Parameters<typeof uiStore.setKey>[0], false)
    }
}

// Lifecycle
onMounted(() => {
    loadInitialData()
    window.addEventListener('keydown', handleKeyDown)
    window.addEventListener('keyup', handleKeyUp)
})

onUnmounted(() => {
    window.removeEventListener('keydown', handleKeyDown)
    window.removeEventListener('keyup', handleKeyUp)
})
</script>

<template>
    <div class="app-container">
        <div class="main-layout">
            <aside class="panel-left">
                <TrackList />
                <SystemStatus />
                <ActivityLog />
            </aside>

            <div class="video-container">
                <div class="video-wrapper">
                    <img id="video" :src="streamUrl" alt="Video Stream" />
                    <VideoOverlay />
                </div>
            </div>

            <aside class="panel-right">
                <TelemetryPanel />
                <ControlPanel />
                <CommStatus />
                <ObstaclePanel v-if="uiStore.panels.obstacleOpen" />
            </aside>
        </div>

        <BottomBar :is-connected="isConnected" />

        <CalibrationWizard />
        <ToastContainer />
    </div>

    <Teleport to="body">
        <div v-if="!isConnected" class="standby-overlay">
            <div class="standby-content">
                <div class="standby-text">
                    {{ wasEverConnected ? 'PLEASE STAND BY' : 'Connecting...' }}
                </div>
                <div v-if="wasEverConnected" class="standby-subtext">Reconnecting...</div>
            </div>
        </div>
    </Teleport>
</template>

<style scoped>
.app-container {
    width: 100%;
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: var(--bg-deep-space);
}

.main-layout {
    display: flex;
    flex: 1;
    overflow: hidden;
}

.panel-left,
.panel-right {
    width: var(--panel-width);
    min-width: var(--panel-width);
    background: var(--bg-slate);
    padding: var(--panel-gap);
    display: flex;
    flex-direction: column;
    gap: var(--panel-gap);
    overflow-y: auto;
    overflow-x: hidden;
}

.panel-left {
    border-right: 1px solid var(--border-subtle);
}

.panel-right {
    border-left: 1px solid var(--border-subtle);
}

.video-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: var(--panel-gap);
    background: var(--bg-deep-space);
}

.video-wrapper {
    flex: 1;
    position: relative;
    background: #000;
    border-radius: 8px;
    overflow: hidden;
    display: flex;
    align-items: center;
    justify-content: center;
    border: 1px solid var(--border-panel);
}

#video {
    max-width: 100%;
    max-height: 100%;
    display: block;
}
</style>

<style>
.standby-overlay {
    position: fixed;
    inset: 0;
    z-index: 9999;
    background: rgba(10, 14, 20, 0.92);
    display: flex;
    align-items: center;
    justify-content: center;
}

.standby-content {
    text-align: center;
}

.standby-text {
    color: var(--accent-cyan);
    font-family: var(--font-heading);
    font-size: 3rem;
    font-weight: bold;
    letter-spacing: 0.15em;
    animation: standby-pulse 2s ease-in-out infinite;
}

.standby-subtext {
    color: var(--text-dim);
    font-family: var(--font-data);
    font-size: 1.1rem;
    margin-top: 1rem;
    letter-spacing: 0.05em;
}

@keyframes standby-pulse {
    0%,
    100% {
        opacity: 1;
    }
    50% {
        opacity: 0.4;
    }
}
</style>
