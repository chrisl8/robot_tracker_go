<script setup lang="ts">
import { computed } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { getTrackColor } from '@/types/robot'

const robotStore = useRobotStore()

const tracks = computed(() => robotStore.confirmedTracks)

function getTrackLabel(track: { id: number; tag_id?: number }): string {
    if (track.tag_id !== undefined) {
        return `Robot ${track.tag_id}`
    }
    return `Track #${track.id}`
}

function selectTrack(track: { id: number }): void {
    robotStore.selectTrack(track.id)
}

function isSelected(trackId: number): boolean {
    return robotStore.selectedTrackId === trackId
}

function hasDestination(trackId: number): boolean {
    return robotStore.destination?.robot_id === trackId
}
</script>

<template>
    <div class="panel">
        <h3>Detected Targets</h3>
        <div v-if="tracks.length === 0" class="no-tracks">
            No targets detected
        </div>
        <div v-else class="track-list">
            <div
                v-for="track in tracks"
                :key="track.id"
                class="track-item"
                :class="{ selected: isSelected(track.id), 'has-destination': hasDestination(track.id) }"
                @click="selectTrack(track)"
            >
                <div
                    class="track-color"
                    :style="{ background: getTrackColor(track.id) }"
                >
                    {{ track.id }}
                </div>
                <div class="track-info">
                    <div class="track-label">
                        {{ getTrackLabel(track) }}
                        <span v-if="track.tag_id" class="track-tag">Tag</span>
                        <span v-if="isSelected(track.id)" class="track-selected">Selected</span>
                        <span v-if="hasDestination(track.id)" class="track-destination">Goal</span>
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
    background: #1a1a2e;
    border-radius: 8px;
    padding: 16px;
}

h3 {
    font-size: 0.85rem;
    text-transform: uppercase;
    color: #888;
    margin-bottom: 12px;
    letter-spacing: 0.5px;
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
    background: #16213e;
    border-radius: 6px;
    margin-bottom: 8px;
    transition: background 0.2s;
}

.track-item:hover {
    background: #1a2a4e;
}

.track-item.selected {
    background: #2a3a5e;
    border: 1px solid #ffffff;
}

.track-item.has-destination {
    border-left: 3px solid #9333ea;
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
    color: #1a1a2e;
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
    font-size: 0.7rem;
    background: #0f3460;
    padding: 2px 6px;
    border-radius: 4px;
    color: #888;
}

.track-selected {
    font-size: 0.7rem;
    background: #ffffff;
    padding: 2px 6px;
    border-radius: 4px;
    color: #1a1a2e;
    font-weight: bold;
}

.track-destination {
    font-size: 0.7rem;
    background: #9333ea;
    padding: 2px 6px;
    border-radius: 4px;
    color: white;
    font-weight: bold;
}

.track-conf {
    font-size: 0.75rem;
    color: #4ecca3;
    margin-top: 2px;
}

.no-tracks {
    color: #555;
    text-align: center;
    padding: 20px;
    font-size: 0.9rem;
}
</style>
