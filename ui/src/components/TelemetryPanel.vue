<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'

const robotStore = useRobotStore()

const selectedTrack = computed(() => robotStore.selectedTrack)

const headingDeg = computed(() => {
    if (!selectedTrack.value?.heading) return '--'
    return `${((selectedTrack.value.heading * 180) / Math.PI).toFixed(1)}°`
})

const confidence = computed(() => {
    if (!selectedTrack.value) return '--'
    return `${(selectedTrack.value.confidence * 100).toFixed(0)}%`
})

const position = computed(() => {
    if (!selectedTrack.value) return { x: '--', y: '--' }
    const [x1, y1, x2, y2] = selectedTrack.value.bbox
    return {
        x: Math.round((x1 + x2) / 2).toString(),
        y: Math.round((y1 + y2) / 2).toString(),
    }
})
</script>

<template>
    <div class="panel telemetry-panel">
        <h3>Telemetry</h3>
        <div v-if="!selectedTrack" class="no-selection">Select a robot to view telemetry</div>
        <div v-else class="telemetry-grid">
            <div class="telemetry-row">
                <span class="telemetry-label">Heading</span>
                <span class="telemetry-value">{{ headingDeg }}</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Confidence</span>
                <span class="telemetry-value">{{ confidence }}</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Position</span>
                <span class="telemetry-value">{{ position.x }}, {{ position.y }}</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Velocity</span>
                <span class="telemetry-value unavailable">--</span>
            </div>
            <div class="telemetry-divider"></div>
            <div class="telemetry-row">
                <span class="telemetry-label">Battery</span>
                <span class="telemetry-value unavailable">--</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Signal</span>
                <span class="telemetry-value unavailable">--</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Temp</span>
                <span class="telemetry-value unavailable">--</span>
            </div>
            <div class="telemetry-row">
                <span class="telemetry-label">Motors</span>
                <span class="telemetry-value unavailable">--</span>
            </div>
        </div>
    </div>
</template>

<style scoped>
.telemetry-panel {
    background: var(--panel-dark);
    border-radius: 8px;
    padding: 16px;
    border: 1px solid var(--border-panel);
}

h3 {
    font-family: var(--font-heading);
    font-size: 0.85rem;
    text-transform: uppercase;
    color: var(--accent-cyan);
    margin-bottom: 12px;
    letter-spacing: 1px;
}

.no-selection {
    color: var(--text-dim);
    text-align: center;
    padding: 12px;
    font-size: 0.85rem;
}

.telemetry-grid {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.telemetry-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 4px 0;
}

.telemetry-label {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.5px;
}

.telemetry-value {
    font-family: var(--font-data);
    font-size: 0.85rem;
    color: var(--text-primary);
}

.telemetry-value.unavailable {
    color: var(--text-dim);
}

.telemetry-divider {
    height: 1px;
    background: var(--border-subtle);
    margin: 4px 0;
}
</style>
