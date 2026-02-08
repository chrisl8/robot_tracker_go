<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { useWebSocket } from '@/composables/useWebSocket'
import StatusBar from '@/components/StatusBar.vue'
import ControlPanel from '@/components/ControlPanel.vue'
import TrackList from '@/components/TrackList.vue'
import ObstaclePanel from '@/components/ObstaclePanel.vue'
import CalibrationWizard from '@/components/CalibrationWizard.vue'
import ToastContainer from '@/components/ToastContainer.vue'

const robotStore = useRobotStore()
const obstacleStore = useObstacleStore()
const uiStore = useUIStore()

const videoRef = ref<HTMLImageElement | null>(null)
void videoRef // videoRef is only used for Vue template binding
const overlayRef = ref<HTMLCanvasElement | null>(null)
const streamUrl = '/stream'

const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws`
const { isConnected } = useWebSocket(wsUrl)

import { useCanvas } from '@/composables/useCanvas'
useCanvas(overlayRef)

// Fetch initial data
async function loadInitialData(): Promise<void> {
    try {
        // Load obstacles
        const obstaclesResponse = await fetch('/api/obstacles')
        const obstaclesData = await obstaclesResponse.json()
        obstacleStore.setObstacles(obstaclesData.obstacles || [])

        // Load calibration status
        const calibrationResponse = await fetch('/api/calibration/status')
        const calibrationData = await calibrationResponse.json()
        uiStore.setCalibrationState(calibrationData.state, calibrationData.message)
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
        <header class="app-header">
            <h1>Robot Tracker</h1>
            <div class="header-right">
                <div class="status">
                    <span>FPS: <span class="value">{{ robotStore.status.fps }}</span></span>
                    <span>Tracks: <span class="value">{{ robotStore.confirmedCount }}</span></span>
                    <span :class="isConnected ? 'connected' : 'disconnected'">
                        Arduino: <span class="value">{{ robotStore.status.arduinoState }}</span>
                    </span>
                </div>

                <div
                    class="calibration-badge"
                    :class="uiStore.calibration.state === 'calibrated' ? 'calibrated' : 'not-calibrated'"
                    @click="uiStore.openCalibration"
                >
                    {{ uiStore.calibration.state === 'calibrated' ? 'Calibrated' : 'Not Calibrated' }}
                </div>

                <button
                    class="obstacle-toggle"
                    @click="uiStore.toggleObstaclePanel"
                >
                    Obstacles ({{ obstacleStore.obstacleCount }})
                </button>
            </div>
        </header>

        <div class="main-layout">
            <div class="video-container">
                <div class="video-wrapper">
                    <img ref="videoRef" id="video" :src="streamUrl" alt="Video Stream" />
                    <canvas ref="overlayRef" id="overlay"></canvas>
                    <div v-if="!isConnected" class="loading">Connecting...</div>
                </div>
            </div>

            <aside class="sidebar">
                <StatusBar />
                <ControlPanel />
                <TrackList />
                <ObstaclePanel v-if="uiStore.panels.obstacleOpen" />
            </aside>
        </div>

        <CalibrationWizard />
        <ToastContainer />
    </div>
</template>

<style scoped>
.app-container {
    width: 100%;
    height: 100vh;
    display: flex;
    flex-direction: column;
    background: #1a1a2e;
}

.main-layout {
    display: flex;
    flex: 1;
    overflow: hidden;
}

.video-container {
    flex: 1;
    display: flex;
    flex-direction: column;
    padding: 16px;
    background: #0a0a0f;
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
}

#video {
    max-width: 100%;
    max-height: 100%;
    display: block;
}

#overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    pointer-events: auto;
}

.loading {
    position: absolute;
    color: #666;
    font-size: 1rem;
}

.sidebar {
    width: 300px;
    background: #16213e;
    padding: 16px;
    border-left: 1px solid #0f3460;
    display: flex;
    flex-direction: column;
    gap: 16px;
    overflow-y: auto;
}
</style>
