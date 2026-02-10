<script setup lang="ts">
import { computed } from 'vue'
import { useObstacleStore } from '@/stores/obstacleStore'
import { useUIStore } from '@/stores/uiStore'

const obstacleStore = useObstacleStore()
const uiStore = useUIStore()

const obstacles = computed(() => obstacleStore.obstacles)
const drawingMode = computed(() => obstacleStore.drawingMode)

function toggleDrawingMode(): void {
    obstacleStore.toggleDrawingMode()
    if (obstacleStore.drawingMode) {
        uiStore.showToast('Click and drag on video to draw obstacle', 'info')
    }
}

async function deleteObstacle(id: string): Promise<void> {
    try {
        const response = await fetch(`/api/obstacles/${id}`, {
            method: 'DELETE',
        })

        if (!response.ok) {
            throw new Error('Failed to delete obstacle')
        }

        const data = await response.json()
        obstacleStore.setObstacles(data.obstacles.obstacles || [])
        uiStore.showToast('Obstacle deleted', 'success')
    } catch (e) {
        console.error('Failed to delete obstacle:', e)
        uiStore.showToast('Failed to delete obstacle', 'error')
    }
}

async function clearAllObstacles(): Promise<void> {
    if (!confirm('Are you sure you want to clear all obstacles?')) {
        return
    }

    try {
        const response = await fetch('/api/obstacles/clear', {
            method: 'POST',
        })

        if (!response.ok) {
            throw new Error('Failed to clear obstacles')
        }

        const data = await response.json()
        obstacleStore.setObstacles(data.obstacles.obstacles || [])
        uiStore.showToast('All obstacles cleared', 'success')
    } catch (e) {
        console.error('Failed to clear obstacles:', e)
        uiStore.showToast('Failed to clear obstacles', 'error')
    }
}

async function saveObstacles(): Promise<void> {
    try {
        const response = await fetch('/api/obstacles/save', {
            method: 'POST',
        })

        if (!response.ok) {
            throw new Error('Failed to save obstacles')
        }

        obstacleStore.setSaved(true)
        uiStore.showToast('Obstacles saved', 'success')
    } catch (e) {
        console.error('Failed to save obstacles:', e)
        uiStore.showToast('Failed to save obstacles', 'error')
    }
}
</script>

<template>
    <div class="panel obstacle-panel">
        <h3>Static Obstacles</h3>

        <div v-if="obstacles.length === 0 && !drawingMode" class="no-obstacles">
            No obstacles defined
        </div>

        <div v-else-if="obstacles.length > 0" class="obstacle-list">
            <div v-for="obstacle in obstacles" :key="obstacle.id" class="obstacle-item">
                <span>{{ obstacle.name }}</span>
                <button @click="deleteObstacle(obstacle.id)">×</button>
            </div>
        </div>

        <div v-if="drawingMode" class="drawing-instructions">
            <span class="instruction-icon">📐</span>
            Click and drag on video to draw obstacle
        </div>

        <div class="obstacle-controls">
            <button
                class="btn draw-btn"
                :class="{ active: drawingMode }"
                @click="toggleDrawingMode"
            >
                {{ drawingMode ? '✕ Finish' : '+ Draw Obstacle' }}
            </button>
        </div>

        <div class="obstacle-controls" :class="{ disabled: drawingMode }">
            <button class="btn" @click="clearAllObstacles" :disabled="drawingMode">
                Clear All
            </button>
            <button class="btn" @click="saveObstacles" :disabled="drawingMode">Save</button>
        </div>
    </div>
</template>

<style scoped>
.panel {
    background: #1a1a2e;
    border-radius: 8px;
    padding: 16px;
}

.obstacle-panel {
    border: 1px solid rgba(255, 107, 107, 0.3);
}

h3 {
    font-size: 0.85rem;
    text-transform: uppercase;
    color: #ff6b6b;
    margin-bottom: 12px;
    letter-spacing: 0.5px;
}

.obstacle-list {
    max-height: 150px;
    overflow-y: auto;
}

.obstacle-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 8px;
    background: rgba(255, 107, 107, 0.1);
    border-radius: 4px;
    margin-bottom: 4px;
}

.obstacle-item:hover {
    background: rgba(255, 107, 107, 0.2);
}

.obstacle-item button {
    background: transparent;
    border: none;
    color: #ff6b6b;
    cursor: pointer;
    font-size: 1.2rem;
    padding: 0 4px;
}

.obstacle-item button:hover {
    color: #ff4444;
}

.obstacle-controls {
    display: flex;
    gap: 8px;
    margin-top: 12px;
}

.obstacle-controls .btn {
    flex: 1;
    padding: 8px;
    font-size: 0.8rem;
    background: #0f3460;
    color: #eee;
    border: none;
    border-radius: 6px;
    cursor: pointer;
    transition: all 0.2s;
}

.obstacle-controls .btn:hover {
    background: #1a4a7a;
}

.no-obstacles {
    color: #555;
    text-align: center;
    padding: 20px;
    font-size: 0.9rem;
}

.draw-btn {
    background: #4ecca3;
    color: #1a1a2e;
}

.draw-btn:hover {
    background: #5fd9b0;
}

.draw-btn.active {
    background: #e94560;
    color: #fff;
}

.draw-btn.active:hover {
    background: #ff5a75;
}

.obstacle-controls.disabled {
    opacity: 0.5;
    pointer-events: none;
}

.drawing-instructions {
    background: rgba(78, 204, 163, 0.15);
    border: 1px solid rgba(78, 204, 163, 0.3);
    border-radius: 6px;
    padding: 10px 12px;
    margin-bottom: 12px;
    font-size: 0.85rem;
    color: #4ecca3;
    display: flex;
    align-items: center;
    gap: 8px;
}

.instruction-icon {
    font-size: 1rem;
}
</style>
