<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
    heading: number | null
}>()

const degText = computed(() => {
    if (props.heading === null) return '--'
    return `${((props.heading * 180) / Math.PI).toFixed(1)}°`
})

const pointerRotation = computed(() => {
    if (props.heading === null) return 0
    return (props.heading * 180) / Math.PI
})
</script>

<template>
    <div class="compass-container">
        <svg viewBox="0 0 80 80" class="compass-svg">
            <!-- Outer ring -->
            <circle
                cx="40"
                cy="40"
                r="36"
                fill="none"
                stroke="var(--border-subtle)"
                stroke-width="1.5"
            />

            <!-- Tick marks -->
            <line
                v-for="i in 12"
                :key="i"
                :x1="40 + 30 * Math.sin(((i - 1) * 30 * Math.PI) / 180)"
                :y1="40 - 30 * Math.cos(((i - 1) * 30 * Math.PI) / 180)"
                :x2="40 + 33 * Math.sin(((i - 1) * 30 * Math.PI) / 180)"
                :y2="40 - 33 * Math.cos(((i - 1) * 30 * Math.PI) / 180)"
                stroke="var(--text-dim)"
                stroke-width="1"
            />

            <!-- Cardinal labels -->
            <text x="40" y="14" class="cardinal">N</text>
            <text x="67" y="43" class="cardinal">E</text>
            <text x="40" y="73" class="cardinal">S</text>
            <text x="13" y="43" class="cardinal">W</text>

            <!-- Pointer (only when heading is available) -->
            <g v-if="heading !== null" :transform="`rotate(${pointerRotation}, 40, 40)`">
                <polygon points="40,14 37,28 43,28" fill="var(--accent-cyan)" opacity="0.9" />
                <line
                    x1="40"
                    y1="28"
                    x2="40"
                    y2="52"
                    stroke="var(--accent-cyan)"
                    stroke-width="2"
                    opacity="0.5"
                />
            </g>

            <!-- Center dot -->
            <circle cx="40" cy="40" r="2" fill="var(--accent-cyan)" opacity="0.6" />

            <!-- Center text -->
            <text x="40" y="44" class="center-text">{{ degText }}</text>
        </svg>
    </div>
</template>

<style scoped>
.compass-container {
    display: flex;
    justify-content: center;
    margin-bottom: 12px;
}

.compass-svg {
    width: 72px;
    height: 72px;
}

.cardinal {
    font-family: var(--font-data);
    font-size: 8px;
    fill: var(--text-dim);
    text-anchor: middle;
    dominant-baseline: central;
}

.center-text {
    font-family: var(--font-data);
    font-size: 7px;
    fill: var(--text-primary);
    text-anchor: middle;
    dominant-baseline: central;
    display: none;
}
</style>
