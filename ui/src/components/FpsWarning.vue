<script setup lang="ts">
import { computed } from 'vue'
import { TriangleAlert, X } from 'lucide-vue-next'
import { formatFps } from '@/composables/fpsHealth'
import { useFpsHealthStore } from '@/stores/fpsHealthStore'
import { useRobotStore } from '@/stores/robotStore'

const fpsHealth = useFpsHealthStore()
const robotStore = useRobotStore()

const autonomous = computed(() => robotStore.controlMode === 'autonomous')
const stalled = computed(() => fpsHealth.kind === 'stalled')
</script>

<template>
    <div v-if="fpsHealth.bannerVisible" class="fps-warning" :class="{ autonomous }" role="alert">
        <TriangleAlert class="fps-warning-icon" :size="18" />
        <div class="fps-warning-body">
            <template v-if="stalled">
                <div class="fps-warning-title">
                    No video frames for {{ fpsHealth.stalledSec }} s. The camera stream has stalled;
                    the service is trying to reopen it. If this does not recover, restart the
                    service.
                </div>
                <div v-if="autonomous" class="fps-warning-emphasis">
                    Autonomous mode is active; the robot is driving without video.
                </div>
            </template>
            <template v-else>
                <div class="fps-warning-title">
                    Frame rate is {{ formatFps(fpsHealth.fps) }} fps. Below about 5 fps autonomous
                    driving is unreliable.
                </div>
                <div v-if="autonomous" class="fps-warning-emphasis">
                    Autonomous mode is active; expect overshoot.
                </div>
                <div class="fps-warning-hint">
                    Check CPU load, the camera, and the FRAME TIMING lines in the log.
                </div>
            </template>
        </div>
        <button
            class="fps-warning-close"
            type="button"
            aria-label="Dismiss frame rate warning"
            @click="fpsHealth.dismiss()"
        >
            <X :size="14" />
        </button>
    </div>
</template>

<style scoped>
.fps-warning {
    position: absolute;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 20;
    display: flex;
    align-items: flex-start;
    gap: 10px;
    width: max-content;
    max-width: min(560px, calc(100% - 24px));
    padding: 8px 10px 8px 12px;
    background: rgba(24, 8, 4, 0.92);
    border: 1px solid var(--alert-red);
    border-radius: 8px;
    box-shadow: 0 0 14px rgba(255, 61, 0, 0.45);
    color: var(--text-primary);
    font-family: var(--font-body);
    font-size: 0.8rem;
    line-height: 1.35;
    pointer-events: auto;
}

.fps-warning.autonomous {
    box-shadow: 0 0 22px 3px rgba(255, 61, 0, 0.7);
}

.fps-warning-icon {
    flex: none;
    margin-top: 1px;
    color: var(--alert-red);
}

.fps-warning-body {
    flex: 1;
    min-width: 0;
}

.fps-warning-title {
    font-weight: 600;
}

.fps-warning-emphasis {
    margin-top: 2px;
    color: var(--alert-red);
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.4px;
}

.fps-warning-hint {
    margin-top: 2px;
    color: var(--text-secondary);
    font-size: 0.72rem;
}

.fps-warning-close {
    flex: none;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    padding: 0;
    background: transparent;
    border: 1px solid var(--border-panel);
    border-radius: 4px;
    color: var(--text-secondary);
    cursor: pointer;
}

.fps-warning-close:hover {
    color: var(--text-primary);
    border-color: var(--alert-red);
}
</style>
