<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDraggable, useIntervalFn } from '@vueuse/core'
import { useUIStore } from '@/stores/uiStore'
import { assessPlacement } from '@/composables/calibrationPlacement'
import type {
    CalibrationComputeRequest,
    CalibrationComputeResponse,
    CalibrationRating,
    CalibrationTagsResponse,
} from '@/types/api'

const SAVED_MESSAGE =
    'Calibration saved. Pick the tags up; you only need them again if the camera moves or its resolution changes.'
const POLL_INTERVAL_MS = 500
const FALLBACK_TAG_SIZE_CM = 15

const uiStore = useUIStore()

const isOpen = computed(() => uiStore.panels.calibrationOpen)
const target = computed(() => uiStore.calibrationTarget)
const assessment = computed(() =>
    target.value ? assessPlacement(uiStore.detectedTags, frame.value, target.value) : null
)
const tagSizeCm = computed(() =>
    target.value ? Math.round(target.value.tagSize * 100) : FALLBACK_TAG_SIZE_CM
)

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
const pinned = ref(false)
const lastPosition = ref({ x: initialX, y: initialY })
const frame = ref({ width: 1280, height: 720 })
const width = ref(1.0)
const depth = ref(0.6)
const layoutInitialized = ref(false)
const busy = ref(false)
const result = ref<{ ok: boolean; data: CalibrationComputeResponse } | null>(null)
let fetching = false

const minSpread = computed(() => (target.value ? target.value.tagSize * 2 + 0.1 : 0.4))
const layoutValid = computed(
    () =>
        Number.isFinite(width.value) &&
        Number.isFinite(depth.value) &&
        width.value >= minSpread.value &&
        depth.value >= minSpread.value
)

const ratingLabels: Record<CalibrationRating, string> = { good: 'Good', ok: 'OK', poor: 'Poor' }

const worstTag = computed(() => {
    const perTag = result.value?.data.perTag
    if (!perTag || perTag.length === 0) return null
    return perTag.reduce((worst, t) => (t.rmsCm > worst.rmsCm ? t : worst))
})

function shortLabel(label: string): string {
    return label.split(' (')[0]
}

function fmtCm(value: number | undefined): string {
    return value === undefined ? '--' : `${value.toFixed(1)} cm`
}

async function fetchDetectedTags(): Promise<void> {
    if (fetching) return
    fetching = true
    try {
        const response = await fetch('/api/calibration/detected-tags')
        const data = (await response.json()) as Partial<CalibrationTagsResponse>
        uiStore.setDetectedTags(data.tags || [])
        if (data.frameWidth && data.frameHeight) {
            frame.value = { width: data.frameWidth, height: data.frameHeight }
        }
        if (data.target) {
            uiStore.setCalibrationTarget(data.target)
        }
    } catch (e) {
        console.error('Failed to fetch detected tags:', e)
    } finally {
        fetching = false
    }
}

const { pause, resume } = useIntervalFn(fetchDetectedTags, POLL_INTERVAL_MS, { immediate: false })

async function computeCalibration(): Promise<void> {
    const t = target.value
    if (!t || !assessment.value?.canCalibrate || !layoutValid.value || busy.value) return

    const ids = new Set(t.tags.map(tag => tag.id))
    const body: CalibrationComputeRequest = {
        width: width.value,
        depth: depth.value,
        tags: uiStore.detectedTags
            .filter(tag => ids.has(tag.id))
            .map(tag => ({ id: tag.id, corners: tag.corners })),
    }

    busy.value = true
    try {
        const response = await fetch('/api/calibration/compute', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body),
        })
        const data = (await response.json()) as CalibrationComputeResponse
        const ok = response.ok && !data.error
        result.value = { ok, data }
        step.value = 4
        if (ok) {
            uiStore.setCalibrationState('calibrated', data.message ?? SAVED_MESSAGE)
            uiStore.showToast('Calibration saved', 'success')
        } else {
            uiStore.showToast(data.error ?? 'Calibration was not saved', 'error')
        }
    } catch (e) {
        console.error('Failed to compute calibration:', e)
        uiStore.showToast('Failed to compute calibration', 'error')
    } finally {
        busy.value = false
    }
}

function close(): void {
    uiStore.closeCalibration()
    step.value = 1
    result.value = null
    uiStore.resetCalibrationWizard()
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

watch(
    assessment,
    value => {
        uiStore.setCalibrationPlacement(value)
    },
    { immediate: true }
)

watch(target, t => {
    if (t && !layoutInitialized.value) {
        width.value = t.defaultWidth
        depth.value = t.defaultDepth
        layoutInitialized.value = true
    }
})

watch([isOpen, step], ([open, currentStep]) => {
    if (open && currentStep === 3) {
        fetchDetectedTags()
        resume()
    } else {
        pause()
    }
})

watch(isOpen, open => {
    if (open) {
        step.value = 1
        result.value = null
        fetchDetectedTags()
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
                :class="{ transparent: step === 3 }"
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
                        <h2>Step 1: Print the Calibration Tags</h2>

                        <p class="wizard-text">
                            Calibration uses five special tags: one <strong>Center</strong> tag and
                            four <strong>Corner</strong> tags. Print them once and keep them; you
                            only need to lay them out again if the camera moves.
                        </p>

                        <div class="link-list">
                            <a
                                class="calibration-link primary-link"
                                href="/calibration-tags/print.html"
                                target="_blank"
                                rel="noopener"
                            >
                                Open the printable tag sheet
                            </a>
                            <div v-if="target" class="download-links">
                                Or download individually:
                                <a
                                    v-for="tag in target.tags"
                                    :key="tag.id"
                                    class="calibration-link"
                                    :href="`/calibration-tags/tag-${tag.id}.svg`"
                                    :download="`tag-${tag.id}.svg`"
                                >
                                    {{ shortLabel(tag.label) }}
                                </a>
                            </div>
                        </div>

                        <ul class="wizard-list">
                            <li>
                                Print at <strong>100% / "Actual size"</strong>, not "fit to page".
                            </li>
                            <li>
                                Each tag's black square must measure
                                <strong>{{ tagSizeCm }} cm</strong>. If it does not, reprint at
                                100%. The sheet has a scale bar to check.
                            </li>
                            <li>Matte paper works best, since glossy paper glares under lights.</li>
                        </ul>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="cancelCalibration">
                                Cancel
                            </button>
                            <button class="calibration-btn primary" @click="step = 2">
                                Next: Lay Out
                            </button>
                        </div>
                    </div>

                    <div v-else-if="step === 2" class="calibration-step active">
                        <h2>Step 2: Lay Out the Tags</h2>

                        <svg
                            class="layout-diagram"
                            viewBox="0 0 300 190"
                            role="img"
                            aria-label="Tag layout as seen by the camera"
                        >
                            <text x="150" y="12" class="diagram-note" text-anchor="middle">
                                top of the camera view
                            </text>
                            <rect x="40" y="26" width="220" height="140" class="diagram-area" />
                            <g class="diagram-tag">
                                <rect x="28" y="38" width="24" height="24" />
                                <rect x="248" y="38" width="24" height="24" />
                                <rect x="248" y="130" width="24" height="24" />
                                <rect x="28" y="130" width="24" height="24" />
                                <rect x="138" y="84" width="24" height="24" class="center" />
                            </g>
                            <g class="diagram-text" text-anchor="middle">
                                <text x="40" y="54">1</text>
                                <text x="260" y="54">2</text>
                                <text x="260" y="146">3</text>
                                <text x="40" y="146">4</text>
                                <text x="150" y="100">C</text>
                                <text x="150" y="184">Width</text>
                            </g>
                            <text x="6" y="100" class="diagram-note">Depth</text>
                        </svg>

                        <ul class="wizard-list">
                            <li>
                                Put the <strong>Center</strong> tag in the middle of the area the
                                robot will drive in.
                            </li>
                            <li>
                                Put the four <strong>Corner</strong> tags on a rectangle around it,
                                as far apart as the robot will travel. Corner 1 is top-left as seen
                                in the camera view, then clockwise.
                            </li>
                            <li>
                                Turn every tag so its <strong>UP</strong> arrow points toward the
                                top of the camera view, and lay them flat.
                            </li>
                            <li>
                                Measure the distance between the tags with a tape measure and enter
                                it below (corner to corner, tag centers).
                            </li>
                        </ul>

                        <div class="layout-inputs">
                            <label>
                                Width (m)
                                <input
                                    v-model.number="width"
                                    type="number"
                                    step="0.05"
                                    :min="minSpread"
                                    class="spread-input"
                                />
                            </label>
                            <label>
                                Depth (m)
                                <input
                                    v-model.number="depth"
                                    type="number"
                                    step="0.05"
                                    :min="minSpread"
                                    class="spread-input"
                                />
                            </label>
                        </div>
                        <div v-if="!layoutValid" class="calibration-status warning">
                            Width and depth must each be at least {{ minSpread.toFixed(2) }} m.
                        </div>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="step = 1">
                                Back
                            </button>
                            <button
                                class="calibration-btn primary"
                                :disabled="!layoutValid"
                                @click="step = 3"
                            >
                                Next: Check Placement
                            </button>
                        </div>
                    </div>

                    <div v-else-if="step === 3" class="calibration-step active">
                        <h2>Step 3: Adjust Placement</h2>

                        <div v-if="!assessment" class="calibration-status info">
                            Waiting for the camera...
                        </div>

                        <template v-else>
                            <div class="detected-tags-list">
                                <div
                                    v-for="tag in assessment.tags"
                                    :key="tag.id"
                                    class="detected-tag-item"
                                    :class="`sev-${tag.severity}`"
                                >
                                    <span class="tag-id">{{ tag.label }}</span>
                                    <span class="tag-status">
                                        {{ tag.found ? `${tag.sizePx} px` : 'Not detected' }}
                                    </span>
                                </div>
                            </div>

                            <ul v-if="assessment.issues.length > 0" class="placement-issues">
                                <li
                                    v-for="(issue, i) in assessment.issues"
                                    :key="i"
                                    :class="issue.severity"
                                >
                                    {{ issue.message }}
                                </li>
                            </ul>
                            <div v-else class="calibration-status success">
                                Placement looks good. Press Calibrate.
                            </div>
                        </template>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="step = 2">
                                Back
                            </button>
                            <button
                                class="calibration-btn primary"
                                :disabled="!assessment?.canCalibrate || busy"
                                @click="computeCalibration"
                            >
                                {{ busy ? 'Calibrating...' : 'Calibrate' }}
                            </button>
                        </div>
                    </div>

                    <div v-else-if="step === 4 && result" class="calibration-step active">
                        <h2>Step 4: Result</h2>

                        <div
                            class="calibration-status"
                            :class="result.ok ? 'success' : 'error'"
                            data-testid="calibration-result"
                        >
                            {{ result.ok ? SAVED_MESSAGE : result.data.error }}
                        </div>

                        <div v-if="result.data.rmsCm !== undefined" class="result-summary">
                            <span
                                v-if="result.data.rating"
                                class="rating-badge"
                                :class="`rating-${result.data.rating}`"
                            >
                                {{ ratingLabels[result.data.rating] }}
                            </span>
                            <span class="result-numbers">
                                Average error {{ fmtCm(result.data.rmsCm) }}, worst
                                {{ fmtCm(result.data.maxCm) }}
                            </span>
                        </div>

                        <table
                            v-if="result.data.perTag && result.data.perTag.length > 0"
                            class="result-table"
                        >
                            <thead>
                                <tr>
                                    <th>Tag</th>
                                    <th>Average</th>
                                    <th>Worst</th>
                                </tr>
                            </thead>
                            <tbody>
                                <tr
                                    v-for="tag in result.data.perTag"
                                    :key="tag.id"
                                    :class="{
                                        worst:
                                            worstTag?.id === tag.id &&
                                            result.data.rating !== 'good',
                                    }"
                                >
                                    <td>{{ shortLabel(tag.label) }}</td>
                                    <td>{{ fmtCm(tag.rmsCm) }}</td>
                                    <td>{{ fmtCm(tag.maxCm) }}</td>
                                </tr>
                            </tbody>
                        </table>

                        <div
                            v-if="worstTag && result.data.rating && result.data.rating !== 'good'"
                            class="calibration-status warning"
                        >
                            {{ shortLabel(worstTag.label) }} has the largest error. Check its
                            position and the tape measurements, then calibrate again.
                        </div>

                        <div class="calibration-buttons">
                            <button
                                v-if="!result.ok"
                                class="calibration-btn secondary"
                                @click="step = 3"
                            >
                                Back to Placement
                            </button>
                            <button class="calibration-btn primary" @click="close">Done</button>
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

.wizard-text {
    font-size: 0.9rem;
    color: var(--text-secondary);
    line-height: 1.5;
    margin-bottom: 12px;
}

.wizard-list {
    margin: 0 0 16px 18px;
    padding: 0;
    font-size: 0.85rem;
    color: var(--text-secondary);
    line-height: 1.5;
}

.wizard-list li {
    margin-bottom: 6px;
}

.link-list {
    background: var(--panel-dark);
    border: 1px solid var(--border-panel);
    border-radius: 8px;
    padding: 12px 16px;
    margin-bottom: 16px;
}

.download-links {
    margin-top: 10px;
    font-size: 0.8rem;
    color: var(--text-dim);
    display: flex;
    flex-wrap: wrap;
    gap: 6px 12px;
    align-items: center;
}

.calibration-link {
    color: var(--accent-cyan);
    text-decoration: underline;
    font-family: var(--font-data);
}

.calibration-link.primary-link {
    font-size: 1rem;
    font-weight: 600;
}

.layout-diagram {
    width: 100%;
    max-width: 320px;
    display: block;
    margin: 0 auto 12px;
}

.diagram-area {
    fill: none;
    stroke: var(--border-subtle);
    stroke-dasharray: 4 4;
}

.diagram-tag rect {
    fill: var(--panel-dark);
    stroke: var(--accent-cyan);
    stroke-width: 1.5;
}

.diagram-tag rect.center {
    fill: rgba(0, 217, 255, 0.2);
}

.diagram-text text,
.diagram-note {
    fill: var(--text-dim);
    font-family: var(--font-data);
    font-size: 11px;
}

.layout-inputs {
    display: flex;
    gap: 12px;
    margin-bottom: 12px;
}

.layout-inputs label {
    flex: 1;
    font-size: 0.8rem;
    color: var(--text-dim);
    display: flex;
    flex-direction: column;
    gap: 6px;
}

.spread-input {
    width: 100%;
    padding: 10px;
    border: 2px solid var(--border-subtle);
    border-radius: 8px;
    background: var(--panel-dark);
    color: var(--text-primary);
    font-family: var(--font-data);
    font-size: 1.1rem;
    text-align: center;
}

.spread-input:focus {
    outline: none;
    border-color: var(--accent-cyan);
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
    margin-bottom: 12px;
}

.detected-tag-item {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 12px;
    background: var(--bg-slate);
    border-radius: 6px;
    margin-bottom: 6px;
    border: 2px solid transparent;
}

.detected-tag-item:last-child {
    margin-bottom: 0;
}

.detected-tag-item.sev-ok {
    border-color: #00e676;
}

.detected-tag-item.sev-warning {
    border-color: #ffab00;
}

.detected-tag-item.sev-blocking {
    border-color: #ff3d00;
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

.placement-issues {
    list-style: none;
    margin: 0 0 12px;
    padding: 0;
}

.placement-issues li {
    font-size: 0.85rem;
    line-height: 1.4;
    padding: 8px 12px;
    border-radius: 6px;
    margin-bottom: 6px;
    background: var(--panel-dark);
    border-left: 3px solid #ffab00;
    color: var(--text-secondary);
}

.placement-issues li.blocking {
    border-left-color: #ff3d00;
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

.calibration-status.success {
    background: rgba(0, 230, 118, 0.12);
    color: #00e676;
}

.calibration-status.warning {
    background: rgba(255, 171, 0, 0.12);
    color: #ffab00;
}

.calibration-status.error {
    background: rgba(255, 61, 0, 0.12);
    color: #ff3d00;
}

.result-summary {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 12px;
}

.rating-badge {
    padding: 4px 12px;
    border-radius: 999px;
    font-family: var(--font-heading);
    font-weight: 700;
    color: var(--bg-deep-space);
}

.rating-good {
    background: #00e676;
}

.rating-ok {
    background: #ffab00;
}

.rating-poor {
    background: #ff3d00;
}

.result-numbers {
    font-family: var(--font-data);
    font-size: 0.85rem;
    color: var(--text-secondary);
}

.result-table {
    width: 100%;
    border-collapse: collapse;
    margin-bottom: 12px;
    font-family: var(--font-data);
    font-size: 0.8rem;
    color: var(--text-secondary);
}

.result-table th,
.result-table td {
    text-align: left;
    padding: 6px 8px;
    border-bottom: 1px solid var(--border-subtle);
}

.result-table tr.worst td {
    color: #ffab00;
    font-weight: bold;
}
</style>
