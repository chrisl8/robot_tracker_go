<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'

// Says what the robot is doing now, or why it is not moving. `overlay` is the
// compact chip drawn over the video; without it, a row for the Control panel.
defineProps<{ overlay?: boolean }>()

const robotStore = useRobotStore()
const motion = computed(() => robotStore.motion)
</script>

<template>
    <div
        v-if="motion"
        class="motion"
        :class="[overlay ? 'motion-overlay' : 'motion-row', 'sev-' + motion.severity]"
        :data-code="motion.code"
        role="status"
    >
        <span class="motion-dot"></span>
        <span class="motion-text">{{ motion.text }}</span>
    </div>
</template>

<style scoped>
.motion {
    display: flex;
    align-items: center;
    gap: 8px;
    font-family: var(--font-data);
    font-size: 0.8rem;
    line-height: 1.3;
    color: var(--text-primary);
}

.motion-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    flex-shrink: 0;
    background: var(--text-dim);
}

.sev-ok .motion-dot {
    background: var(--success-green);
    box-shadow: 0 0 4px var(--success-green);
}

.sev-warn .motion-dot {
    background: var(--warning-amber);
}

.sev-error .motion-dot {
    background: var(--alert-red);
}

/* Over the video: never swallows the clicks that set a destination. */
.motion-overlay {
    position: absolute;
    left: 8px;
    bottom: 8px;
    z-index: 15;
    max-width: calc(100% - 16px);
    padding: 6px 10px;
    border-radius: 8px;
    background: rgba(8, 12, 18, 0.85);
    border: 1px solid var(--border-panel);
    pointer-events: none;
}

.motion-overlay.sev-ok {
    border-color: var(--success-green);
}

.motion-overlay.sev-warn {
    border-color: var(--warning-amber);
}

.motion-overlay.sev-error {
    border-color: var(--alert-red);
}

.motion-row {
    padding: 8px 0 4px;
}

.motion-row.sev-warn .motion-text {
    color: var(--warning-amber);
}

.motion-row.sev-error .motion-text {
    color: var(--alert-red);
}
</style>
