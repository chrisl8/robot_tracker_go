<script setup lang="ts">
import { ScanSearch } from 'lucide-vue-next'
import { useTempObstacleStore } from '@/stores/tempObstacleStore'

const tempStore = useTempObstacleStore()
</script>

<template>
    <div class="panel" data-testid="temp-obstacles">
        <h3><ScanSearch :size="16" />Temporary obstacles</h3>
        <div class="group">
            <button
                class="btn toggle"
                :class="{ active: tempStore.enabled }"
                data-testid="temp-detection"
                @click="tempStore.setEnabled(!tempStore.enabled)"
            >
                Detection {{ tempStore.enabled ? 'ON' : 'OFF' }}
            </button>
            <button
                class="btn toggle"
                :class="{ active: tempStore.applied }"
                :disabled="!tempStore.enabled"
                data-testid="temp-steer"
                @click="tempStore.setApply(!tempStore.applied)"
            >
                Steer around them {{ tempStore.applied ? 'ON' : 'OFF' }}
            </button>
            <button
                class="btn"
                :disabled="!tempStore.enabled"
                data-testid="temp-reset"
                @click="tempStore.resetBackground()"
            >
                Reset background
            </button>
            <button
                class="btn toggle"
                :class="{ active: tempStore.showMask }"
                data-testid="temp-mask"
                @click="tempStore.toggleMask()"
            >
                {{ tempStore.showMask ? 'Hide' : 'Show' }} mask
            </button>
        </div>
    </div>
</template>

<style scoped>
.panel {
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

.group {
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.btn {
    width: 100%;
    padding: 8px 12px;
    font-family: var(--font-data);
    font-size: 0.8rem;
    background: rgba(0, 217, 255, 0.08);
    color: var(--text-primary);
    border: 1px solid var(--border-panel);
    border-radius: 6px;
    cursor: pointer;
}

.btn:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.18);
}

.btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

.btn.toggle.active {
    background: rgba(0, 217, 255, 0.25);
    border-color: var(--accent-cyan);
    color: var(--accent-cyan);
}
</style>
