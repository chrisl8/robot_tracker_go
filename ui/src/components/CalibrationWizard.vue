<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDraggable } from '@vueuse/core'
import { useUIStore } from '@/stores/uiStore'
import type { DetectedTagInfo } from '@/types/api'
import tagDiagramUrl from '@/assets/april-tag-how-to-measure-for-website.png?url'

const uiStore = useUIStore()

const isOpen = computed(() => uiStore.panels.calibrationOpen)
const detectedTags = ref<DetectedTagInfo[]>([])
const selectedTagId = computed({
    get: () => uiStore.selectedCalibrationTagId,
    set: value => uiStore.setSelectedCalibrationTag(value),
})

const dialogRef = ref<HTMLElement | null>(null)
const handleRef = ref<HTMLElement | null>(null)

const initialX = window.innerWidth / 2 - 260
const initialY = window.innerHeight / 2 - 300

const { x, y } = useDraggable(dialogRef, {
    initialValue: { x: initialX, y: initialY },
    handle: handleRef,
    onEnd: () => {
        if (pinned.value) {
            lastPosition.value = { x: x.value, y: y.value }
        }
    },
})

const step = ref(1)
const tagSize = ref(0.15)
const pinned = ref(false)
const lastPosition = ref({ x: initialX, y: initialY })

const tagSizePresets = [
    { label: '10cm', value: 0.1 },
    { label: '15cm', value: 0.15 },
    { label: '20cm', value: 0.2 },
    { label: '25cm', value: 0.25 },
]

function close(): void {
    uiStore.closeCalibration()
    step.value = 1
    selectedTagId.value = null
    uiStore.setDetectedTags([])
}

function openStep2(): void {
    step.value = 2
    fetchDetectedTags()
}

function selectTag(tagId: number): void {
    uiStore.setSelectedCalibrationTag(tagId)
}

function isTagSelected(tagId: number): boolean {
    return uiStore.selectedCalibrationTagId === tagId
}

function getSelectedTagCorners(): [number, number][] | null {
    const selected = detectedTags.value.find(tag => tag.id === uiStore.selectedCalibrationTagId)
    return selected?.corners || null
}

async function fetchDetectedTags(): Promise<void> {
    try {
        const response = await fetch('/api/calibration/detected-tags')
        const data = await response.json()
        detectedTags.value = data.tags || []
        uiStore.setDetectedTags(detectedTags.value)
    } catch (e) {
        console.error('Failed to fetch detected tags:', e)
    }
}

async function computeCalibration(): Promise<void> {
    if (uiStore.selectedCalibrationTagId === null) {
        uiStore.showToast('Please select a tag', 'warning')
        return
    }

    const corners = getSelectedTagCorners()
    if (!corners) {
        uiStore.showToast('Tag corners not available', 'error')
        return
    }

    try {
        const response = await fetch('/api/calibration/compute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                tagId: uiStore.selectedCalibrationTagId,
                tagSize: tagSize.value,
                corners: corners,
            }),
        })

        const data = await response.json()

        if (data.error) {
            uiStore.showToast(data.error, 'error')
            return
        }

        const message =
            data.computedWidth && data.computedHeight
                ? `Calibration complete! Area: ${data.computedWidth.toFixed(2)}m x ${data.computedHeight.toFixed(2)}m`
                : 'Calibration computed successfully'

        uiStore.showToast(message, 'success')
        uiStore.setCalibrationState('calibrated', message)
        close()
    } catch (e) {
        console.error('Failed to compute calibration:', e)
        uiStore.showToast('Failed to compute calibration', 'error')
    }
}

function cancelCalibration(): void {
    fetch('/api/calibration/cancel', { method: 'POST' })
    uiStore.setCalibrationState('not_calibrated', '')
    close()
}

function togglePin(): void {
    pinned.value = !pinned.value
    if (pinned.value) {
        lastPosition.value = { x: x.value, y: y.value }
    }
}

watch(isOpen, open => {
    if (open) {
        step.value = 1
        uiStore.setSelectedCalibrationTag(null)
        if (pinned.value) {
            x.value = lastPosition.value.x
            y.value = lastPosition.value.y
        } else {
            x.value = initialX
            y.value = initialY
        }
        lastPosition.value = { x: x.value, y: y.value }
    }
})
</script>

<template>
    <Teleport to="body">
        <div v-if="isOpen" class="calibration-wizard">
            <div
                class="calibration-overlay"
                :class="{ transparent: step === 2 }"
                @click="close"
            ></div>
            <div
                ref="dialogRef"
                class="calibration-content"
                :class="{ pinned }"
                :style="{
                    left: `${pinned ? lastPosition.x : x}px`,
                    top: `${pinned ? lastPosition.y : y}px`,
                }"
            >
                <div ref="handleRef" class="calibration-header">
                    <h2>Calibration</h2>
                    <div class="calibration-actions">
                        <button
                            class="calibration-btn-icon"
                            :class="{ pinned }"
                            @click="togglePin"
                            title="Pin position"
                        >
                            📌
                        </button>
                        <button class="calibration-btn-icon" @click="close" title="Close">✕</button>
                    </div>
                </div>

                <div class="calibration-body">
                    <div v-if="step === 1" class="calibration-step active">
                        <h2>Step 1: Enter Tag Size</h2>

                        <div class="tag-size-presets">
                            <button
                                v-for="preset in tagSizePresets"
                                :key="preset.value"
                                class="preset-btn"
                                :class="{ selected: tagSize === preset.value }"
                                @click="tagSize = preset.value"
                            >
                                {{ preset.label }}
                            </button>
                        </div>

                        <input
                            v-model="tagSize"
                            type="number"
                            step="0.01"
                            class="tag-size-input"
                            placeholder="Enter tag size in meters"
                        />
                        <div class="tag-size-hint">
                            Enter the measured size of your AprilTag in meters
                        </div>

                        <div class="measurement-guide">
                            <h4>How to Measure</h4>
                            <img
                                :src="tagDiagramUrl"
                                alt="How to measure AprilTag size"
                                class="tag-diagram"
                            />
                            <div class="measurement-note">
                                Measure the black border of the tag (not including white margin).
                            </div>
                        </div>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="cancelCalibration">
                                Cancel
                            </button>
                            <button class="calibration-btn primary" @click="openStep2">
                                Detect Tags
                            </button>
                        </div>
                    </div>

                    <div v-else-if="step === 2" class="calibration-step active">
                        <h2>Step 2: Select Detected Tag</h2>

                        <div v-if="detectedTags.length === 0" class="calibration-status info">
                            No tags detected. Make sure a tag is visible in the camera.
                        </div>

                        <div v-else class="detected-tags-list">
                            <div
                                v-for="tag in detectedTags"
                                :key="tag.id"
                                class="detected-tag-item"
                                :class="{ selected: isTagSelected(tag.id) }"
                                @click="selectTag(tag.id)"
                            >
                                <span class="tag-id">Tag {{ tag.id }}</span>
                                <span class="tag-status">Detected</span>
                            </div>
                        </div>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="step = 1">
                                Back
                            </button>
                            <button
                                class="calibration-btn primary"
                                :disabled="uiStore.selectedCalibrationTagId === null"
                                @click="computeCalibration"
                            >
                                Use Selected Tag
                            </button>
                        </div>
                    </div>
                </div>
            </div>
        </div>
    </Teleport>
</template>

<style scoped>
.calibration-wizard {
    position: fixed;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    z-index: 1000;
}

.calibration-overlay {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
    height: 100%;
    background: rgba(10, 14, 20, 0.8);
    z-index: 1;
}

.calibration-overlay.transparent {
    background: rgba(10, 14, 20, 0.2);
}

.calibration-content {
    position: fixed;
    background: var(--bg-slate);
    border-radius: 12px;
    padding: 0;
    max-width: 520px;
    width: 90%;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.6);
    border: 1px solid var(--border-subtle);
    z-index: 2;
}

.calibration-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    border-bottom: 1px solid var(--border-subtle);
    cursor: move;
    background: var(--panel-dark);
    border-radius: 12px 12px 0 0;
    user-select: none;
}

.calibration-header h2 {
    margin: 0;
    font-family: var(--font-heading);
    color: var(--accent-cyan);
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 1.2rem;
}

.calibration-header h2::before {
    content: '';
    display: inline-block;
    width: 3px;
    height: 18px;
    background: var(--accent-cyan);
    border-radius: 2px;
}

.calibration-actions {
    display: flex;
    gap: 8px;
}

.calibration-btn-icon {
    width: 32px;
    height: 32px;
    border: 1px solid var(--border-subtle);
    border-radius: 6px;
    background: var(--panel-dark);
    color: var(--text-secondary);
    cursor: pointer;
    font-size: 1rem;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
}

.calibration-btn-icon:hover {
    background: rgba(0, 217, 255, 0.1);
    color: var(--text-primary);
}

.calibration-btn-icon.pinned {
    background: var(--accent-cyan);
    color: var(--bg-deep-space);
    border-color: var(--accent-cyan);
}

.calibration-body {
    padding: 20px 24px 24px;
}

.calibration-step {
    display: none;
}

.calibration-step.active {
    display: block;
}

.calibration-step h2 {
    margin-bottom: 16px;
    font-family: var(--font-heading);
    color: var(--accent-cyan);
}

.tag-size-presets {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
    flex-wrap: wrap;
}

.preset-btn {
    padding: 6px 12px;
    border: 1px solid var(--border-subtle);
    background: var(--panel-dark);
    color: var(--text-dim);
    border-radius: 4px;
    font-family: var(--font-data);
    font-size: 0.8rem;
    cursor: pointer;
    transition: all 0.2s;
}

.preset-btn:hover {
    border-color: var(--accent-cyan);
    color: var(--accent-cyan);
}

.preset-btn.selected {
    background: rgba(0, 217, 255, 0.1);
    border-color: var(--accent-cyan);
    color: var(--accent-cyan);
}

.tag-size-input {
    width: 100%;
    padding: 12px;
    border: 2px solid var(--border-subtle);
    border-radius: 8px;
    background: var(--panel-dark);
    color: var(--text-primary);
    font-family: var(--font-data);
    font-size: 1.1rem;
    text-align: center;
    margin-bottom: 16px;
}

.tag-size-input:focus {
    outline: none;
    border-color: var(--accent-cyan);
}

.tag-size-hint {
    font-size: 0.8rem;
    color: var(--text-dim);
    text-align: center;
    margin-top: -8px;
    margin-bottom: 16px;
}

.measurement-guide {
    background: var(--panel-dark);
    border-radius: 8px;
    padding: 16px;
    margin: 16px 0;
    border: 1px solid var(--border-panel);
}

.measurement-guide h4 {
    font-family: var(--font-heading);
    color: var(--text-dim);
    font-size: 0.85rem;
    margin-bottom: 12px;
    text-transform: uppercase;
    letter-spacing: 1px;
}

.tag-diagram {
    width: 100%;
    max-width: 300px;
    height: auto;
    display: block;
    margin: 0 auto 12px;
}

.measurement-note {
    font-size: 0.8rem;
    color: var(--text-dim);
    line-height: 1.5;
    margin-top: 8px;
}

.calibration-buttons {
    display: flex;
    gap: 12px;
    margin-top: 20px;
}

.calibration-btn {
    flex: 1;
    padding: 12px;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 1rem;
    font-weight: 600;
    transition: all 0.2s;
}

.calibration-btn.primary {
    background: var(--accent-cyan);
    color: var(--bg-deep-space);
}

.calibration-btn.primary:hover {
    background: rgba(0, 217, 255, 0.85);
    transform: translateY(-1px);
}

.calibration-btn.primary:disabled {
    background: rgba(0, 217, 255, 0.15);
    color: var(--text-dim);
    cursor: not-allowed;
    transform: none;
}

.calibration-btn.secondary {
    background: var(--panel-dark);
    color: var(--text-primary);
    border: 1px solid var(--border-subtle);
}

.calibration-btn.secondary:hover {
    background: rgba(0, 217, 255, 0.08);
}

.detected-tags-list {
    background: var(--panel-dark);
    border-radius: 8px;
    padding: 12px;
    margin-bottom: 16px;
    max-height: 150px;
    overflow-y: auto;
}

.detected-tag-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: var(--bg-slate);
    border-radius: 6px;
    margin-bottom: 6px;
    cursor: pointer;
    transition: all 0.2s;
    border: 2px solid transparent;
}

.detected-tag-item:hover {
    background: rgba(0, 217, 255, 0.05);
}

.detected-tag-item.selected {
    border-color: var(--accent-cyan);
    background: rgba(0, 217, 255, 0.1);
}

.tag-id {
    font-family: var(--font-data);
    font-weight: bold;
    color: var(--accent-cyan);
}

.tag-status {
    font-family: var(--font-data);
    font-size: 0.8rem;
    color: var(--text-dim);
}

.calibration-status {
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 16px;
    text-align: center;
    font-weight: 500;
}

.calibration-status.info {
    background: rgba(0, 217, 255, 0.1);
    color: var(--accent-cyan);
}
</style>
