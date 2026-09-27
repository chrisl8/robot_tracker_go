<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useFpsHealthStore } from '@/stores/fpsHealthStore'
import { Radio } from 'lucide-vue-next'

const robotStore = useRobotStore()
const fpsHealth = useFpsHealthStore()

const fpsPct = computed(() => {
    const fps = robotStore.status.fps || 0
    return Math.min((fps / 30) * 100, 100)
})

const fpsBarColor = computed(() => {
    if (fpsHealth.level === 'critical') return 'var(--alert-red)'
    if (fpsHealth.level === 'low') return 'var(--warning-amber)'
    const fps = robotStore.status.fps || 0
    if (fps > 20) return 'var(--success-green)'
    if (fps > 10) return 'var(--warning-amber)'
    return 'var(--alert-red)'
})
</script>

<template>
    <div class="panel comm-panel">
        <h3><Radio :size="14" /> Communications</h3>
        <div class="comm-grid">
            <div class="comm-row">
                <span class="comm-label">Arduino</span>
                <span
                    class="comm-status"
                    :class="
                        robotStore.status.arduinoState === 'Connected'
                            ? 'connected'
                            : 'disconnected'
                    "
                >
                    <span class="comm-dot"></span>
                    {{ robotStore.status.arduinoState }}
                </span>
            </div>
            <div class="comm-row-stacked">
                <div class="comm-row">
                    <span class="comm-label">FPS</span>
                    <span class="comm-value" :class="fpsHealth.levelClass">{{
                        fpsHealth.stalled
                            ? 'NO VIDEO'
                            : robotStore.status.fps
                              ? robotStore.status.fps.toFixed(1)
                              : '--'
                    }}</span>
                </div>
                <div class="fps-bar" :class="fpsHealth.levelClass && fpsHealth.levelClass + '-bar'">
                    <div
                        class="fps-fill"
                        :style="{ width: fpsPct + '%', background: fpsBarColor }"
                    ></div>
                </div>
            </div>
            <div class="comm-row">
                <span class="comm-label">Memory</span>
                <span class="comm-value">{{
                    robotStore.status.hostMemoryMB
                        ? robotStore.status.hostMemoryMB.toFixed(1) + ' MB'
                        : '--'
                }}</span>
            </div>
        </div>
    </div>
</template>

<style scoped>
.comm-panel {
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

.comm-grid {
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.comm-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.comm-label {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-transform: uppercase;
    letter-spacing: 0.5px;
}

.comm-status {
    display: flex;
    align-items: center;
    gap: 6px;
    font-family: var(--font-data);
    font-size: 0.8rem;
}

.comm-status.connected {
    color: var(--success-green);
}

.comm-status.disconnected {
    color: var(--alert-red);
}

.comm-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
}

.comm-status.connected .comm-dot {
    background: var(--success-green);
    box-shadow: 0 0 4px var(--success-green);
}

.comm-status.disconnected .comm-dot {
    background: var(--alert-red);
}

.comm-value {
    font-family: var(--font-data);
    font-size: 0.85rem;
    color: var(--text-primary);
}

.comm-row-stacked {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.fps-bar {
    width: 100%;
    height: 3px;
    background: var(--bg-slate);
    border-radius: 2px;
    overflow: hidden;
}

.fps-fill {
    height: 100%;
    border-radius: 2px;
    transition:
        width 0.3s ease,
        background 0.3s ease;
}
</style>
