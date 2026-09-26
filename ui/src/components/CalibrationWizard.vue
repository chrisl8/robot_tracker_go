<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDraggable, useIntervalFn } from '@vueuse/core'
import { useUIStore } from '@/stores/uiStore'
import { assessPlacement, PLACEMENT_THRESHOLDS } from '@/composables/calibrationPlacement'
import type {
    CalibrationComputeRequest,
    CalibrationComputeResponse,
    CalibrationRating,
    CalibrationTagsResponse,
} from '@/types/api'
import type { CalibrationStep } from '@/types/ui'

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

const step = ref<CalibrationStep>('print')
const docked = computed(() => step.value === 'place')
const dockMoved = ref(false)
const pinned = ref(false)
const lastPosition = ref({ x: initialX, y: initialY })

const { x, y } = useDraggable(dialogRef, {
    initialValue: { x: initialX, y: initialY },
    handle: handleRef,
    onStart: () => {
        // The docked panel is anchored to the bottom; switch to free positioning on first drag
        if (docked.value && dialogRef.value) {
            const rect = dialogRef.value.getBoundingClientRect()
            x.value = rect.left
            y.value = rect.top
            dockMoved.value = true
        }
    },
    onEnd: () => {
        if (pinned.value && !docked.value) {
            lastPosition.value = { x: x.value, y: y.value }
        }
    },
})

const contentStyle = computed(() => {
    if (docked.value && !dockMoved.value) return {}
    if (docked.value) return { left: `${x.value}px`, top: `${y.value}px` }
    return {
        left: `${pinned.value ? lastPosition.value.x : x.value}px`,
        top: `${pinned.value ? lastPosition.value.y : y.value}px`,
    }
})

const frame = ref({ width: 1280, height: 720 })
const busy = ref(false)
const result = ref<{ ok: boolean; data: CalibrationComputeResponse } | null>(null)
let fetching = false

const ratingLabels: Record<CalibrationRating, string> = { good: 'Good', ok: 'OK', poor: 'Poor' }

const worstTag = computed(() => {
    const data = result.value?.data
    const perTag = data?.perTag
    if (!data || !perTag || perTag.length === 0) return null
    const byId = perTag.find(t => t.id === data.worstTagId)
    if (byId) return byId
    return perTag.reduce((worst, t) => (t.rmsCm > worst.rmsCm ? t : worst))
})

const hints = computed(() => (assessment.value ? assessment.value.issues : []))
const visibleHints = computed(() => hints.value.slice(0, PLACEMENT_THRESHOLDS.maxVisibleHints))
const hiddenHintCount = computed(() => hints.value.length - visibleHints.value.length)

function tagState(id: number, found: boolean): string {
    if (!found) return 'Not detected'
    const state = assessment.value?.guides.find(g => g.id === id)?.state
    return state === 'inside' ? 'In box' : 'Near box'
}

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
    if (!t || !assessment.value?.canCalibrate || busy.value) return

    const ids = new Set(t.tags.map(tag => tag.id))
    const body: CalibrationComputeRequest = {
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
        step.value = 'result'
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
    step.value = 'print'
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

watch(
    step,
    (current, previous) => {
        uiStore.setCalibrationStep(current)
        if (current === 'place') {
            dockMoved.value = false
        } else if (previous === 'place') {
            x.value = pinned.value ? lastPosition.value.x : initialX
            y.value = pinned.value ? lastPosition.value.y : initialY
        }
    },
    { immediate: true }
)

watch([isOpen, step], ([open, currentStep]) => {
    if (open && currentStep === 'place') {
        fetchDetectedTags()
        resume()
    } else {
        pause()
    }
})

watch(isOpen, open => {
    if (open) {
        step.value = 'print'
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
        <div v-if="isOpen" class="calibration-wizard" :class="{ docked }">
            <div v-if="!docked" class="calibration-overlay" @click="close"></div>
            <div
                ref="dialogRef"
                class="calibration-content"
                :class="{ pinned, moved: dockMoved }"
                :style="contentStyle"
            >
                <div ref="handleRef" class="calibration-header">
                    <h2>Calibration</h2>
                    <div class="calibration-actions">
                        <button
                            v-if="!docked"
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
                    <div v-if="step === 'print'" class="calibration-step active">
                        <h2>Step 1: Print the Calibration Tags</h2>

                        <p class="wizard-text">
                            Calibration uses five special tags: one <strong>Center</strong> tag and
                            four <strong>Corner</strong> tags. Print them once and keep them; you
                            only need them again if the camera moves. There is nothing to measure:
                            in the next step you lay each tag roughly inside its box on the video.
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
                                100%. The sheet has a scale bar to check. The tag size is how the
                                camera learns real distances, so this is the one thing that has to
                                be right.
                            </li>
                            <li>Matte paper works best, since glossy paper glares under lights.</li>
                        </ul>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="cancelCalibration">
                                Cancel
                            </button>
                            <button class="calibration-btn primary" @click="step = 'place'">
                                Next: Place Tags
                            </button>
                        </div>
                    </div>

                    <div v-else-if="step === 'place'" class="calibration-step active">
                        <h2>Step 2: Place the Tags</h2>

                        <p class="wizard-text">
                            Lay each tag flat inside its dashed box on the video. Rough is fine, no
                            measuring. Turn the <strong>Center</strong> tag so its UP arrow points
                            toward the top of the video.
                        </p>

                        <div v-if="!assessment" class="calibration-status info">
                            Waiting for the camera...
                        </div>

                        <template v-else>
                            <div class="detected-tags-list compact">
                                <div
                                    v-for="tag in assessment.tags"
                                    :key="tag.id"
                                    class="detected-tag-item"
                                    :class="`sev-${tag.severity}`"
                                >
                                    <span class="tag-id">{{ shortLabel(tag.label) }}</span>
                                    <span class="tag-status">
                                        {{ tag.found ? `${tag.sizePx} px` : '' }}
                                        {{ tagState(tag.id, tag.found) }}
                                    </span>
                                </div>
                            </div>

                            <ul v-if="hints.length > 0" class="placement-issues compact">
                                <li
                                    v-for="(issue, i) in visibleHints"
                                    :key="i"
                                    :class="issue.tip ? 'tip' : issue.severity"
                                >
                                    {{ issue.message }}
                                </li>
                                <li v-if="hiddenHintCount > 0" class="more">
                                    +{{ hiddenHintCount }} more
                                </li>
                            </ul>
                            <div v-else class="calibration-status success">
                                Placement looks good. Press Calibrate.
                            </div>
                        </template>

                        <div class="calibration-buttons">
                            <button class="calibration-btn secondary" @click="step = 'print'">
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

                    <div v-else-if="step === 'result' && result" class="calibration-step active">
                        <h2>Step 3: Result</h2>

                        <div
                            class="calibration-status"
                            :class="result.ok ? 'success' : 'error'"
                            data-testid="calibration-result"
                        >
                            <template v-if="result.ok">{{ SAVED_MESSAGE }}</template>
                            <template v-else>
                                Nothing was saved. Every tag must lie flat and be printed at exactly
                                {{ tagSizeCm }} cm.
                                <template v-if="worstTag">
                                    {{ shortLabel(worstTag.label) }} looks the most out of place.
                                </template>
                            </template>
                        </div>
                        <p
                            v-if="!result.ok && result.data.error"
                            class="error-detail"
                            data-testid="calibration-error-detail"
                        >
                            {{ result.data.error }}
                        </p>

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
                            v-if="result.data.checks && result.data.checks.length > 0"
                            data-testid="calibration-checks"
                        >
                            <div class="section-title">Optional sanity check</div>
                            <ul class="check-list">
                                <li
                                    v-for="check in result.data.checks"
                                    :key="`${check.fromId}-${check.toId}`"
                                >
                                    {{ shortLabel(check.fromLabel) }} to
                                    {{ shortLabel(check.toLabel) }} should be
                                    {{ check.meters.toFixed(2) }} m apart.
                                </li>
                            </ul>
                            <p class="check-note">
                                If a tape measure disagrees by more than about 3 cm, check that the
                                tags printed at {{ tagSizeCm }} cm.
                            </p>
                        </div>

                        <div class="calibration-buttons">
                            <button
                                v-if="!result.ok"
                                class="calibration-btn secondary"
                                @click="step = 'place'"
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

.calibration-wizard.docked {
    pointer-events: none;
}

.calibration-wizard.docked .calibration-content {
    pointer-events: auto;
    display: flex;
    flex-direction: column;
    left: 50%;
    bottom: 16px;
    top: auto;
    transform: translateX(-50%);
    width: min(780px, 94vw);
    max-width: none;
    max-height: 46vh;
    background: rgba(20, 27, 38, 0.96);
}

.calibration-wizard.docked .calibration-content.moved {
    bottom: auto;
    transform: none;
}

/* Wide screens: live in the right sidebar column, never over the video */
@media (min-width: 1200px) {
    .calibration-wizard.docked .calibration-content:not(.moved) {
        left: auto;
        right: 12px;
        top: 12px;
        bottom: calc(var(--bottom-bar-height, 56px) + 12px);
        transform: none;
        width: 270px;
        max-height: none;
    }

    .calibration-wizard.docked .calibration-content.moved {
        width: 270px;
        max-height: 80vh;
    }

    .calibration-wizard.docked .calibration-body {
        display: flex;
        flex-direction: column;
        flex: 1;
        min-height: 0;
        overflow-y: auto;
    }

    .calibration-wizard.docked .calibration-step.active {
        display: flex;
        flex-direction: column;
        flex: 1;
    }

    .calibration-wizard.docked .calibration-buttons {
        flex-direction: column;
        margin-top: auto;
        padding-top: 10px;
    }

    .calibration-wizard.docked .detected-tags-list.compact {
        flex-direction: column;
        flex-wrap: nowrap;
    }

    .calibration-wizard.docked .detected-tags-list.compact .detected-tag-item {
        flex: none;
        flex-direction: row;
        align-items: center;
        justify-content: space-between;
        gap: 6px;
    }
}

.calibration-wizard.docked .calibration-header {
    padding: 8px 16px;
    border-radius: 12px 12px 0 0;
}

.calibration-wizard.docked .calibration-header h2 {
    font-size: 1rem;
}

.calibration-wizard.docked .calibration-body {
    padding: 10px 16px 14px;
}

.calibration-wizard.docked .calibration-step h2 {
    margin-bottom: 8px;
    font-size: 1rem;
}

.calibration-wizard.docked .wizard-text {
    margin-bottom: 8px;
    font-size: 0.82rem;
}

.calibration-wizard.docked .calibration-buttons {
    margin-top: 10px;
}

.calibration-wizard.docked .calibration-btn {
    padding: 8px;
    font-size: 0.9rem;
}

.detected-tags-list.compact {
    display: flex;
    flex-wrap: wrap;
    gap: 6px;
    padding: 8px;
    margin-bottom: 8px;
}

.detected-tags-list.compact .detected-tag-item {
    flex: 1 1 130px;
    flex-direction: column;
    align-items: flex-start;
    gap: 2px;
    margin-bottom: 0;
    padding: 4px 8px;
    font-size: 0.8rem;
}

.placement-issues.compact li {
    font-size: 0.8rem;
    padding: 5px 10px;
    margin-bottom: 4px;
}

.placement-issues li.tip {
    border-left-color: var(--accent-cyan);
}

.placement-issues li.more {
    font-family: var(--font-data);
    color: var(--text-dim);
    border-left-color: transparent;
}

.check-list {
    list-style: none;
    margin: 0 0 6px;
    padding: 0;
    font-family: var(--font-data);
    font-size: 0.8rem;
    color: var(--text-secondary);
}

.check-list li {
    padding: 3px 0;
}

.check-note,
.error-detail {
    font-size: 0.8rem;
    color: var(--text-dim);
    line-height: 1.4;
    margin-bottom: 12px;
}

.section-title {
    font-family: var(--font-heading);
    font-size: 0.85rem;
    color: var(--text-secondary);
    margin-bottom: 6px;
}
</style>
