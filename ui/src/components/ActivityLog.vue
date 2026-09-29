<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import { useUIStore } from '@/stores/uiStore'
import { ScrollText } from '@lucide/vue'
import type { LogEntryType } from '@/types/ui'

const uiStore = useUIStore()
const listRef = ref<HTMLElement | null>(null)

function formatTime(date: Date): string {
    const h = String(date.getHours()).padStart(2, '0')
    const m = String(date.getMinutes()).padStart(2, '0')
    const s = String(date.getSeconds()).padStart(2, '0')
    return `${h}:${m}:${s}`
}

function dotClass(type: LogEntryType): string {
    return `log-dot log-dot-${type}`
}

watch(
    () => uiStore.activityLog.length,
    async () => {
        await nextTick()
        if (listRef.value) {
            listRef.value.scrollTop = listRef.value.scrollHeight
        }
    }
)
</script>

<template>
    <div class="panel activity-panel">
        <h3><ScrollText :size="14" /> Activity Log</h3>
        <div v-if="uiStore.activityLog.length === 0" class="no-activity">No activity yet</div>
        <div v-else ref="listRef" class="log-list">
            <div v-for="entry in uiStore.activityLog" :key="entry.id" class="log-entry">
                <span class="log-time">{{ formatTime(entry.timestamp) }}</span>
                <span :class="dotClass(entry.type)"></span>
                <span class="log-message">{{ entry.message }}</span>
            </div>
        </div>
    </div>
</template>

<style scoped>
.activity-panel {
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

.log-list {
    max-height: 200px;
    overflow-y: auto;
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.log-entry {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    font-size: 0.75rem;
}

.log-time {
    font-family: var(--font-data);
    color: var(--text-dim);
    flex-shrink: 0;
    font-size: 0.7rem;
}

.log-dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    flex-shrink: 0;
}

.log-dot-info {
    background: var(--accent-cyan);
}

.log-dot-success {
    background: var(--success-green);
}

.log-dot-warning {
    background: var(--warning-amber);
}

.log-dot-error {
    background: var(--alert-red);
}

.log-dot-system {
    background: var(--text-dim);
}

.log-message {
    color: var(--text-secondary);
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
}

.no-activity {
    color: var(--text-dim);
    text-align: center;
    padding: 12px;
    font-size: 0.85rem;
}
</style>
