<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { useFpsHealthStore } from '@/stores/fpsHealthStore'

defineProps<{
    isConnected: boolean
}>()

const robotStore = useRobotStore()
const obstacleStore = useObstacleStore()
const uiStore = useUIStore()
const fpsHealth = useFpsHealthStore()

const calibrationBadge = computed(() => {
    if (uiStore.calibration.state !== 'calibrated') {
        return { text: 'Not Calibrated', cls: 'not-calibrated' }
    }
    if (uiStore.calibration.resolutionMismatch) {
        return { text: 'Recalibrate (resolution changed)', cls: 'not-calibrated' }
    }
    return { text: 'Calibrated', cls: 'calibrated' }
})

const missionTime = ref('00:00:00')
const startTime = Date.now()
let timerInterval: ReturnType<typeof setInterval> | null = null

function updateMissionTime(): void {
    const elapsed = Math.floor((Date.now() - startTime) / 1000)
    const h = String(Math.floor(elapsed / 3600)).padStart(2, '0')
    const m = String(Math.floor((elapsed % 3600) / 60)).padStart(2, '0')
    const s = String(elapsed % 60).padStart(2, '0')
    missionTime.value = `${h}:${m}:${s}`
}

onMounted(() => {
    timerInterval = setInterval(updateMissionTime, 1000)
})

onUnmounted(() => {
    if (timerInterval) clearInterval(timerInterval)
})
</script>

<template>
    <div class="bottom-bar">
        <div class="bottom-bar-group">
            <span class="connection-indicator" :class="isConnected ? 'connected' : 'disconnected'">
                <span class="connection-dot"></span>
                {{ isConnected ? 'Online' : 'Offline' }}
            </span>
        </div>

        <div class="bar-divider"></div>

        <div class="bottom-bar-group">
            <span class="bar-label">T+ {{ missionTime }}</span>
        </div>

        <div class="bar-divider"></div>

        <div class="bottom-bar-group">
            <span class="bar-label">Robots: {{ robotStore.confirmedTracks.length }}</span>
        </div>

        <div class="bar-divider"></div>

        <div class="bottom-bar-group">
            <span class="bar-label" :class="fpsHealth.levelClass">{{
                fpsHealth.stalled
                    ? 'NO VIDEO'
                    : `FPS: ${robotStore.status.fps ? robotStore.status.fps.toFixed(1) : '--'}`
            }}</span>
        </div>

        <div class="bottom-bar-spacer"></div>

        <div v-if="robotStore.destinationMode" class="bottom-bar-group">
            <div class="destination-badge">DEST MODE — ESC to cancel</div>
        </div>

        <div class="bottom-bar-group">
            <div
                class="calibration-badge"
                :class="calibrationBadge.cls"
                @click="uiStore.openCalibration()"
            >
                {{ calibrationBadge.text }}
            </div>
        </div>

        <div class="bottom-bar-group">
            <button class="obstacle-toggle" @click="uiStore.toggleObstaclePanel">
                Obstacles ({{ obstacleStore.obstacleCount }})
            </button>
        </div>
    </div>
</template>

<style scoped>
.bottom-bar {
    height: var(--bottom-bar-height);
    background: var(--bg-slate);
    border-top: 1px solid var(--border-subtle);
    box-shadow: 0 -1px 8px rgba(0, 217, 255, 0.08);
    display: flex;
    align-items: center;
    padding: 0 var(--panel-padding);
    gap: 12px;
}

.bottom-bar-group {
    display: flex;
    align-items: center;
    gap: 8px;
}

.bottom-bar-spacer {
    flex: 1;
}

.bar-divider {
    width: 1px;
    height: 20px;
    background: var(--border-subtle);
}

.bar-label {
    font-family: var(--font-data);
    font-size: 0.75rem;
    color: var(--text-dim);
    letter-spacing: 0.5px;
}

.connection-indicator {
    display: flex;
    align-items: center;
    gap: 6px;
    font-family: var(--font-data);
    font-size: 0.75rem;
    letter-spacing: 0.5px;
}

.connection-indicator.connected {
    color: var(--success-green);
}

.connection-indicator.disconnected {
    color: var(--alert-red);
}

.connection-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
}

.connection-indicator.connected .connection-dot {
    background: var(--success-green);
    box-shadow: 0 0 6px var(--success-green);
}

.connection-indicator.disconnected .connection-dot {
    background: var(--alert-red);
    animation: pulse-dot 2s infinite;
}

@keyframes pulse-dot {
    0%,
    100% {
        opacity: 1;
    }
    50% {
        opacity: 0.4;
    }
}

.destination-badge {
    background: rgba(0, 230, 118, 0.15);
    color: var(--success-green);
    border: 1px solid rgba(0, 230, 118, 0.3);
    padding: 3px 10px;
    border-radius: 4px;
    font-family: var(--font-data);
    font-size: 0.7rem;
    font-weight: 600;
    letter-spacing: 0.5px;
    animation: pulse-badge 2s infinite;
}

@keyframes pulse-badge {
    0%,
    100% {
        opacity: 1;
    }
    50% {
        opacity: 0.5;
    }
}
</style>
