<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { getTrackColor } from '@/types/robot'
import { Crosshair } from 'lucide-vue-next'

const robotStore = useRobotStore()

const tracks = computed(() => [...robotStore.confirmedTracks].sort((a, b) => a.id - b.id))

function getTrackLabel(track: { id: number; tag_id?: number }): string {
    if (track.tag_id !== undefined) {
        return `Robot ${track.tag_id}`
    }
    return `Object #${track.id}`
}

function selectTrack(track: { id: number }): void {
    robotStore.selectTrack(track.id)
}

function isSelected(trackId: number): boolean {
    return robotStore.selectedTrackId === trackId
}

function hasDestination(track: { id: number; tag_id?: number }): boolean {
    return track.tag_id !== undefined && robotStore.destination?.robot_id === track.tag_id
}
</script>

<template>
    <div class="panel">
        <h3><Crosshair :size="14" /> Detected Targets</h3>
        <div v-if="tracks.length === 0" class="no-tracks">No targets detected</div>
        <div v-else class="track-list">
            <div
                v-for="track in tracks"
                :key="track.id"
                class="track-item"
                :class="{
                    selected: isSelected(track.id),
                    'has-destination': hasDestination(track),
                    'has-tag': track.tag_id !== undefined,
                }"
                @click="selectTrack(track)"
            >
                <div class="track-color" :style="{ background: getTrackColor(track.id) }">
                    {{ track.id }}
                </div>
                <div class="track-info">
                    <div class="track-label">
                        {{ getTrackLabel(track) }}
                        <span v-if="track.tag_id" class="track-tag">Tag</span>
                        <span v-if="isSelected(track.id)" class="track-selected">Selected</span>
                        <span v-if="hasDestination(track)" class="track-destination">Goal</span>
                    </div>
                    <div class="track-conf">
                        {{ (track.confidence * 100).toFixed(0) }}% confidence
                    </div>
                </div>
            </div>
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

.track-list {
    max-height: 250px;
    overflow-y: auto;
}

.track-item {
    display: flex;
    align-items: center;
    gap: 12px;
    padding: 10px;
    background: var(--bg-slate);
    border-radius: 6px;
    margin-bottom: 8px;
    border: 1px solid transparent;
    cursor: pointer;
    transition: all 0.2s;
}

.track-item:hover {
    border-color: var(--border-subtle);
    background: rgba(0, 217, 255, 0.05);
}

.track-item.selected {
    background: rgba(0, 217, 255, 0.1);
    border: 1px solid var(--accent-cyan);
    box-shadow: 0 0 8px rgba(0, 217, 255, 0.25);
}

.track-item.has-tag {
    border-left: 3px solid rgba(0, 217, 255, 0.3);
}

.track-item.has-destination {
    border-left: 3px solid var(--accent-blue);
}

.track-color {
    width: 36px;
    height: 36px;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: 600;
    font-size: 0.9rem;
    color: var(--bg-deep-space);
}

.track-info {
    flex: 1;
}

.track-label {
    font-size: 0.9rem;
    font-weight: 500;
    display: flex;
    align-items: center;
    gap: 8px;
}

.track-tag {
    font-family: var(--font-data);
    font-size: 0.65rem;
    background: rgba(0, 217, 255, 0.1);
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--accent-cyan);
    letter-spacing: 0.5px;
}

.track-selected {
    font-family: var(--font-data);
    font-size: 0.65rem;
    background: var(--accent-cyan);
    padding: 2px 6px;
    border-radius: 4px;
    color: var(--bg-deep-space);
    font-weight: bold;
}

.track-destination {
    font-family: var(--font-data);
    font-size: 0.65rem;
    background: var(--accent-blue);
    padding: 2px 6px;
    border-radius: 4px;
    color: white;
    font-weight: bold;
}

.track-conf {
    font-family: var(--font-data);
    font-size: 0.75rem;
    color: var(--text-dim);
    margin-top: 2px;
}

.no-tracks {
    color: var(--text-dim);
    text-align: center;
    padding: 20px;
    font-size: 0.9rem;
}
</style>
