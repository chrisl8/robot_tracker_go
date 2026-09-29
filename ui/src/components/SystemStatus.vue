<script setup lang="ts">
import { useRobotStore } from '@/stores/robotStore'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { Activity } from '@lucide/vue'

const robotStore = useRobotStore()
const obstacleStore = useObstacleStore()
const uiStore = useUIStore()
</script>

<template>
    <div class="panel system-panel">
        <h3><Activity :size="14" /> System</h3>
        <div class="status-grid">
            <div class="status-row">
                <span class="status-label">Arduino</span>
                <span
                    class="status-value"
                    :class="robotStore.status.arduinoState === 'Connected' ? 'good' : 'bad'"
                >
                    {{ robotStore.status.arduinoState }}
                </span>
            </div>
            <div class="status-row clickable" @click="uiStore.openCalibration()">
                <span class="status-label">Calibration</span>
                <span
                    class="status-value"
                    :class="uiStore.calibration.state === 'calibrated' ? 'good' : 'warn'"
                >
                    {{ uiStore.calibration.state === 'calibrated' ? 'OK' : 'Needed' }}
                </span>
            </div>
            <div class="status-row clickable" @click="uiStore.toggleObstaclePanel()">
                <span class="status-label">Obstacles</span>
                <span class="status-value">{{ obstacleStore.obstacleCount }}</span>
            </div>
            <div class="status-row">
                <span class="status-label">Robots</span>
                <span class="status-value">{{ robotStore.confirmedTracks.length }}</span>
            </div>
        </div>
    </div>
</template>

<style scoped>
.system-panel {
    background: var(--panel-dark);
    border-radius: 8px;
    padding: 12px 16px;
    border: 1px solid var(--border-panel);
}

h3 {
    font-family: var(--font-heading);
    font-size: 0.85rem;
    text-transform: uppercase;
    color: var(--accent-cyan);
    margin-bottom: 10px;
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

.status-grid {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.status-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 3px 0;
}

.status-row.clickable {
    cursor: pointer;
    border-radius: 4px;
    padding: 3px 4px;
    margin: 0 -4px;
}

.status-row.clickable:hover {
    background: rgba(0, 217, 255, 0.05);
}

.status-label {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.5px;
}

.status-value {
    font-family: var(--font-data);
    font-size: 0.8rem;
    color: var(--text-secondary);
}

.status-value.good {
    color: var(--success-green);
}

.status-value.warn {
    color: var(--warning-amber);
}

.status-value.bad {
    color: var(--alert-red);
}
</style>
