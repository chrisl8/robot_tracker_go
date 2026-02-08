<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'

const robotStore = useRobotStore()

const fpsDisplay = computed(() => robotStore.status.fps)
const trackCount = computed(() => robotStore.confirmedCount)
const arduinoState = computed(() => robotStore.status.arduinoState)
const isConnected = computed(() => robotStore.status.connected)
</script>

<template>
    <div class="panel">
        <h3>Statistics</h3>
        <div class="stats-grid">
            <div class="stat-item">
                <div class="stat-value">{{ fpsDisplay }}</div>
                <div class="stat-label">FPS</div>
            </div>
            <div class="stat-item">
                <div class="stat-value">{{ trackCount }}</div>
                <div class="stat-label">Tracks</div>
            </div>
            <div class="stat-item">
                <div class="stat-value" :style="{ color: isConnected ? '#4ecca3' : '#e94560' }">
                    {{ arduinoState }}
                </div>
                <div class="stat-label">Arduino</div>
            </div>
            <div class="stat-item">
                <div class="stat-value">{{ robotStore.status.robotCount }}</div>
                <div class="stat-label">Robots</div>
            </div>
        </div>
    </div>
</template>

<style scoped>
.panel {
    background: #1a1a2e;
    border-radius: 8px;
    padding: 16px;
}

h3 {
    font-size: 0.85rem;
    text-transform: uppercase;
    color: #888;
    margin-bottom: 12px;
    letter-spacing: 0.5px;
}

.stats-grid {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 10px;
}

.stat-item {
    background: #16213e;
    padding: 12px;
    border-radius: 6px;
    text-align: center;
}

.stat-value {
    font-size: 1.4rem;
    font-weight: 600;
    color: #4ecca3;
}

.stat-label {
    font-size: 0.75rem;
    color: #666;
    margin-top: 2px;
}
</style>
