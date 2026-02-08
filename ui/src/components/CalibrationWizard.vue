<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDraggable } from '@vueuse/core'
import { useUIStore } from '@/stores/uiStore'

const uiStore = useUIStore()

const isOpen = computed(() => uiStore.panels.calibrationOpen)
const detectedTags = computed(() => uiStore.detectedTags)

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
    }
})

const step = ref(1)
const tagSize = ref(0.15)
const selectedTagId = ref<number | null>(null)
const pinned = ref(false)
const lastPosition = ref({ x: initialX, y: initialY })

const tagSizePresets = [
    { label: '10cm', value: 0.10 },
    { label: '15cm', value: 0.15 },
    { label: '20cm', value: 0.20 },
    { label: '25cm', value: 0.25 }
]

function close(): void {
    uiStore.closeCalibration()
    step.value = 1
    selectedTagId.value = null
}

function openStep2(): void {
    step.value = 2
    fetchDetectedTags()
}

function selectTag(tagId: number): void {
    selectedTagId.value = tagId
}

function isTagSelected(tagId: number): boolean {
    return selectedTagId.value === tagId
}

async function fetchDetectedTags(): Promise<void> {
    try {
        const response = await fetch('/api/calibration/detected-tags')
        const data = await response.json()
        uiStore.setDetectedTags(data.tags || [])
    } catch (e) {
        console.error('Failed to fetch detected tags:', e)
    }
}

async function computeCalibration(): Promise<void> {
    if (selectedTagId.value === null) {
        uiStore.showToast('Please select a tag', 'warning')
        return
    }

    try {
        const response = await fetch('/api/calibration/compute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                tag_id: selectedTagId.value,
                tag_size: tagSize.value
            })
        })

        const data = await response.json()

        if (data.error) {
            uiStore.showToast(data.error, 'error')
            return
        }

        uiStore.showToast('Calibration computed successfully', 'success')
        uiStore.setCalibrationState('calibrated', 'Calibration complete')
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

watch(isOpen, (open) => {
    if (open) {
        step.value = 1
        selectedTagId.value = null
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
            <div class="calibration-overlay" @click="close"></div>
            <div
                ref="dialogRef"
                class="calibration-content"
                :class="{ pinned }"
                :style="{ left: `${pinned ? lastPosition.x : x}px`, top: `${pinned ? lastPosition.y : y}px` }"
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
                        <button class="calibration-btn-icon" @click="close" title="Close">
                            ✕
                        </button>
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
                            <div class="tag-diagram">
                                <div class="tag-preview"></div>
                                <div class="measurement-arrow"></div>
                                <div class="measurement-label">15cm</div>
                            </div>
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

                        <div
                            v-if="detectedTags.length === 0"
                            class="calibration-status info"
                        >
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
                                :disabled="selectedTagId === null"
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
    background: rgba(0, 0, 0, 0.7);
    z-index: 1;
}

.calibration-content {
    position: fixed;
    background: #16213e;
    border-radius: 12px;
    padding: 0;
    max-width: 520px;
    width: 90%;
    max-height: 90vh;
    overflow-y: auto;
    box-shadow: 0 10px 40px rgba(0, 0, 0, 0.5);
    z-index: 2;
}

.calibration-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 24px;
    border-bottom: 1px solid #0f3460;
    cursor: move;
    background: #1a1a2e;
    border-radius: 12px 12px 0 0;
    user-select: none;
}

.calibration-header h2 {
    margin: 0;
    color: #4ecca3;
    display: flex;
    align-items: center;
    gap: 10px;
    font-size: 1.2rem;
}

.calibration-header h2::before {
    content: '📐';
}

.calibration-actions {
    display: flex;
    gap: 8px;
}

.calibration-btn-icon {
    width: 32px;
    height: 32px;
    border: none;
    border-radius: 6px;
    background: #0f3460;
    color: #aaa;
    cursor: pointer;
    font-size: 1rem;
    display: flex;
    align-items: center;
    justify-content: center;
    transition: all 0.2s;
}

.calibration-btn-icon:hover {
    background: #1a4a7a;
    color: #eee;
}

.calibration-btn-icon.pinned {
    background: #4ecca3;
    color: #1a1a2e;
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
    color: #4ecca3;
}

.tag-size-presets {
    display: flex;
    gap: 8px;
    margin-bottom: 12px;
    flex-wrap: wrap;
}

.preset-btn {
    padding: 6px 12px;
    border: 1px solid #0f3460;
    background: #1a1a2e;
    color: #888;
    border-radius: 4px;
    font-size: 0.8rem;
    cursor: pointer;
    transition: all 0.2s;
}

.preset-btn:hover {
    border-color: #4ecca3;
    color: #4ecca3;
}

.preset-btn.selected {
    background: rgba(78, 204, 163, 0.2);
    border-color: #4ecca3;
    color: #4ecca3;
}

.tag-size-input {
    width: 100%;
    padding: 12px;
    border: 2px solid #0f3460;
    border-radius: 8px;
    background: #1a1a2e;
    color: #eee;
    font-size: 1.1rem;
    text-align: center;
    margin-bottom: 16px;
}

.tag-size-input:focus {
    outline: none;
    border-color: #4ecca3;
}

.tag-size-hint {
    font-size: 0.8rem;
    color: #666;
    text-align: center;
    margin-top: -8px;
    margin-bottom: 16px;
}

.measurement-guide {
    background: #1a1a2e;
    border-radius: 8px;
    padding: 16px;
    margin: 16px 0;
}

.measurement-guide h4 {
    color: #888;
    font-size: 0.85rem;
    margin-bottom: 12px;
    text-transform: uppercase;
}

.tag-diagram {
    display: flex;
    align-items: center;
    gap: 16px;
    margin-bottom: 12px;
}

.tag-preview {
    width: 100px;
    height: 100px;
    position: relative;
    border: 8px solid white;
    background: black;
    flex-shrink: 0;
}

.tag-preview::after {
    content: 'DATA';
    position: absolute;
    top: 50%;
    left: 50%;
    transform: translate(-50%, -50%);
    color: white;
    font-size: 10px;
    font-weight: bold;
}

.measurement-arrow {
    flex: 1;
    position: relative;
    height: 30px;
}

.measurement-arrow::before {
    content: '';
    position: absolute;
    left: 0;
    right: 0;
    top: 50%;
    height: 2px;
    background: #4ecca3;
}

.measurement-arrow::after {
    content: '▼';
    position: absolute;
    left: 50%;
    bottom: 0;
    transform: translateX(-50%);
    color: #4ecca3;
    font-size: 12px;
}

.measurement-label {
    text-align: center;
    color: #4ecca3;
    font-weight: bold;
    font-size: 0.9rem;
}

.measurement-note {
    font-size: 0.8rem;
    color: #888;
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
    background: #4ecca3;
    color: #1a1a2e;
}

.calibration-btn.primary:hover {
    background: #5fd9b0;
    transform: translateY(-1px);
}

.calibration-btn.primary:disabled {
    background: #2a4a3a;
    color: #666;
    cursor: not-allowed;
    transform: none;
}

.calibration-btn.secondary {
    background: #0f3460;
    color: #eee;
}

.calibration-btn.secondary:hover {
    background: #1a4a7a;
}

.detected-tags-list {
    background: #1a1a2e;
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
    background: #16213e;
    border-radius: 6px;
    margin-bottom: 6px;
    cursor: pointer;
    transition: all 0.2s;
    border: 2px solid transparent;
}

.detected-tag-item:hover {
    background: #1a2a4e;
}

.detected-tag-item.selected {
    border-color: #00bcd4;
    background: rgba(0, 188, 212, 0.1);
}

.tag-id {
    font-weight: bold;
    color: #4ecca3;
}

.tag-status {
    font-size: 0.8rem;
    color: #888;
}

.calibration-status {
    padding: 12px;
    border-radius: 8px;
    margin-bottom: 16px;
    text-align: center;
    font-weight: 500;
}

.calibration-status.info {
    background: rgba(0, 188, 212, 0.2);
    color: #00bcd4;
}
</style>
