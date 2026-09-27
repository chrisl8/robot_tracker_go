<script setup lang="ts">
import { computed } from 'vue'
import { useTempObstacleStore } from '@/stores/tempObstacleStore'
import { useUIStore } from '@/stores/uiStore'
import { useMaskPreview } from '@/composables/useMaskPreview'

const tempStore = useTempObstacleStore()
const uiStore = useUIStore()

const { url } = useMaskPreview(
    computed(() => tempStore.showMask),
    message => uiStore.showToast(message, 'error')
)
</script>

<template>
    <div v-if="tempStore.showMask" class="mask-preview" data-testid="mask-preview">
        <div class="mask-title">Detector mask</div>
        <img v-if="url" :src="url" alt="Foreground mask" />
        <div v-else class="mask-empty">Waiting for the detector...</div>
        <div class="mask-legend">
            <span class="fg">object</span>
            <span class="robot">robot</span>
            <span class="static">static</span>
        </div>
    </div>
</template>

<style scoped>
.mask-preview {
    position: absolute;
    top: 8px;
    left: 8px;
    width: 240px;
    background: rgba(10, 14, 20, 0.9);
    border: 1px solid var(--border-panel);
    border-radius: 6px;
    padding: 6px;
    z-index: 20;
}

.mask-title {
    font-family: var(--font-heading);
    font-size: 0.7rem;
    text-transform: uppercase;
    color: var(--accent-cyan);
    margin-bottom: 4px;
}

img {
    display: block;
    width: 100%;
    border-radius: 4px;
}

.mask-empty {
    font-size: 0.75rem;
    color: var(--text-dim);
    padding: 16px 0;
    text-align: center;
}

.mask-legend {
    display: flex;
    gap: 8px;
    margin-top: 4px;
    font-size: 0.65rem;
}

.fg {
    color: #ff5252;
}

.robot {
    color: #4d9fff;
}

.static {
    color: #ffd54f;
}
</style>
