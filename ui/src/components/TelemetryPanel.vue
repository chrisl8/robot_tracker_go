<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { Gauge } from '@lucide/vue'
import CompassHeading from '@/components/CompassHeading.vue'

const robotStore = useRobotStore()

const selectedTrack = computed(() => robotStore.selectedTrack)

const headingDeg = computed(() => {
    // 0 rad (facing +x) is a real heading; only a missing value shows '--'.
    if (selectedTrack.value?.heading == null) return '--'
    return `${((selectedTrack.value.heading * 180) / Math.PI).toFixed(1)}°`
})

const confidence = computed(() => {
    if (!selectedTrack.value) return '--'
    return `${(selectedTrack.value.confidence * 100).toFixed(0)}%`
})

const confidencePct = computed(() => {
    if (!selectedTrack.value) return 0
    return selectedTrack.value.confidence * 100
})

const position = computed(() => {
    if (!selectedTrack.value) return { x: '--', y: '--' }
    const [x1, y1, x2, y2] = selectedTrack.value.bbox
    return {
        x: Math.round((x1 + x2) / 2).toString(),
        y: Math.round((y1 + y2) / 2).toString(),
    }
})

const cameraOk = computed(() => selectedTrack.value?.state === 'confirmed')
const controllerOk = computed(() => robotStore.status?.arduinoState === 'Connected')
</script>

<template>
    <div class="panel telemetry-panel">
        <h3><Gauge :size="14" /> Telemetry</h3>
        <div v-if="!selectedTrack" class="no-selection">Select a robot to view telemetry</div>
        <template v-else>
            <CompassHeading :heading="selectedTrack.heading ?? null" />
            <div class="telemetry-grid">
                <div class="telemetry-row">
                    <span class="telemetry-label">Heading</span>
                    <span class="telemetry-value">{{ headingDeg }}</span>
                </div>
                <div class="telemetry-row-stacked">
                    <div class="telemetry-row">
                        <span class="telemetry-label">Confidence</span>
                        <span class="telemetry-value">{{ confidence }}</span>
                    </div>
                    <div class="progress-bar">
                        <div class="progress-fill" :style="{ width: confidencePct + '%' }"></div>
                    </div>
                </div>
                <div class="telemetry-row">
                    <span class="telemetry-label">Position</span>
                    <span class="telemetry-value">{{ position.x }}, {{ position.y }}</span>
                </div>
                <div class="telemetry-row connection-row">
                    <span class="telemetry-label">Connection</span>
                    <span class="connection-indicators">
                        <span class="connection-dot" :class="cameraOk ? 'ok' : 'err'">●</span>
                        <span class="connection-sublabel">Camera</span>
                        <span class="connection-dot" :class="controllerOk ? 'ok' : 'err'">●</span>
                        <span class="connection-sublabel">Controller</span>
                    </span>
                </div>
            </div>
        </template>
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
    display: flex;
    align-items: center;
    gap: 8px;
}

h3::before {
    content: '';
    display: block;
    width: 3px;
    height: 14px;
    background: var(--accent-cyan);
    border-radius: 2px;
    flex-shrink: 0;
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

.connection-row {
    align-items: center;
}

.connection-indicators {
    display: flex;
    align-items: center;
    gap: 4px;
}

.connection-dot {
    font-size: 0.7rem;
    line-height: 1;
}

.connection-dot.ok {
    color: var(--accent-green, #4ade80);
}

.connection-dot.err {
    color: var(--accent-red, #f87171);
}

.connection-sublabel {
    font-size: 0.7rem;
    color: var(--text-dim);
    margin-right: 6px;
}

.telemetry-row-stacked {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.progress-bar {
    width: 100%;
    height: 4px;
    background: var(--bg-slate);
    border-radius: 2px;
    overflow: hidden;
}

.progress-fill {
    height: 100%;
    background: var(--accent-cyan);
    border-radius: 2px;
    transition: width 0.3s ease;
}
</style>
