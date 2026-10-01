<script setup lang="ts">
import { RefreshCw } from '@lucide/vue'
import { useUIStore } from '@/stores/uiStore'

const uiStore = useUIStore()

function reload(): void {
    window.location.reload()
}
</script>

<template>
    <div v-if="uiStore.updateAvailable" class="update-banner" role="alert">
        <RefreshCw :size="16" />
        <span>A new version of the tracker is running. Refresh to load it.</span>
        <button type="button" @click="reload">Refresh</button>
    </div>
</template>

<style scoped>
.update-banner {
    position: fixed;
    top: 8px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 100;
    display: flex;
    align-items: center;
    gap: 10px;
    max-width: calc(100% - 24px);
    padding: 8px 12px;
    background: rgba(24, 8, 4, 0.95);
    border: 1px solid var(--alert-red);
    border-radius: 8px;
    box-shadow: 0 0 14px rgba(255, 61, 0, 0.45);
    color: var(--text-primary);
    font-family: var(--font-body);
    font-size: 0.8rem;
}

.update-banner button {
    padding: 3px 10px;
    background: transparent;
    border: 1px solid var(--alert-red);
    border-radius: 4px;
    color: var(--text-primary);
    cursor: pointer;
}

.update-banner button:hover {
    background: var(--alert-red);
}
</style>
