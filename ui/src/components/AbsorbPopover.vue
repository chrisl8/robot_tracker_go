<script setup lang="ts">
import { computed, onUnmounted, watch } from 'vue'
import { useTempObstacleStore } from '@/stores/tempObstacleStore'

const AUTO_DISMISS_MS = 6000

const tempStore = useTempObstacleStore()
const prompt = computed(() => tempStore.absorbPrompt)

let timer: ReturnType<typeof setTimeout> | null = null

watch(prompt, next => {
    if (timer !== null) clearTimeout(timer)
    timer = next ? setTimeout(() => tempStore.dismissAbsorb(), AUTO_DISMISS_MS) : null
})

onUnmounted(() => {
    if (timer !== null) clearTimeout(timer)
})

function absorb(): void {
    const p = prompt.value
    if (p) void tempStore.absorb(p.x, p.y)
}
</script>

<template>
    <div
        v-if="prompt"
        class="absorb-popover"
        data-testid="absorb-popover"
        :style="{ left: `${prompt.canvasX}px`, top: `${prompt.canvasY}px` }"
        @click.stop
    >
        <button class="absorb-btn" @click="absorb">Absorb (treat as floor)</button>
        <button class="cancel-btn" aria-label="Dismiss" @click="tempStore.dismissAbsorb()">
            &times;
        </button>
    </div>
</template>

<style scoped>
.absorb-popover {
    position: absolute;
    transform: translate(-50%, 8px);
    display: flex;
    gap: 4px;
    background: rgba(10, 14, 20, 0.95);
    border: 1px solid #ff9100;
    border-radius: 6px;
    padding: 4px;
    z-index: 30;
}

button {
    font-size: 0.75rem;
    border-radius: 4px;
    border: 1px solid var(--border-panel);
    background: rgba(255, 145, 0, 0.15);
    color: var(--text-primary);
    padding: 4px 8px;
    cursor: pointer;
}

button:hover {
    background: rgba(255, 145, 0, 0.3);
}

.cancel-btn {
    background: transparent;
}
</style>
