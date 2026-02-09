<script setup lang="ts">
import { ref, watch } from 'vue'
import { useUIStore } from '@/stores/uiStore'

const uiStore = useUIStore()

const pressed = ref<string | null>(null)

async function sendCommand(command: string): Promise<void> {
    try {
        await fetch('/api/command', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ command })
        })
    } catch (e) {
        console.error('Failed to send command:', e)
    }
}

function handleMouseDown(key: string, command: string): void {
    pressed.value = key
    sendCommand(command)
}

function handleMouseUp(): void {
    pressed.value = null
    sendCommand('S') // Stop on release
}

// Watch keyboard state
watch(
    () => uiStore.keyboard,
    (keys) => {
        if (keys.w) {
            sendCommand('F')
        } else if (keys.s) {
            sendCommand('B')
        } else if (keys.a) {
            sendCommand('L')
        } else if (keys.d) {
            sendCommand('R')
        } else if (keys.x) {
            sendCommand('S')
        }
    },
    { deep: true }
)
</script>

<template>
    <div class="panel">
        <h3>Controls</h3>
        <div class="controls">
            <button
                class="btn forward"
                @mousedown="handleMouseDown('w', 'F')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'w' }"
            >
                W
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('a', 'L')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'a' }"
            >
                A
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('s', 'B')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 's' }"
            >
                S
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('d', 'R')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'd' }"
            >
                D
            </button>
            <button
                class="btn stop"
                @mousedown="handleMouseDown('x', 'S')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
            >
                X
            </button>
        </div>
        <div class="keyboard-hint">
            Use <kbd>W</kbd> <kbd>A</kbd> <kbd>S</kbd> <kbd>D</kbd> to move, <kbd>X</kbd> to stop
        </div>
        <div class="divider"></div>
        <button
            class="btn toggle"
            :class="{ active: uiStore.showFootprints }"
            @click="uiStore.toggleFootprints()"
        >
            {{ uiStore.showFootprints ? 'Hide' : 'Show' }} Footprints
        </button>
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

.controls {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
    max-width: 180px;
    margin: 0 auto;
}

.btn {
    padding: 14px;
    border: none;
    border-radius: 8px;
    cursor: pointer;
    font-size: 1.3rem;
    transition: all 0.15s ease;
    background: #0f3460;
    color: #eee;
    display: flex;
    align-items: center;
    justify-content: center;
}

.btn:hover {
    background: #1a4a7a;
    transform: translateY(-1px);
}

.btn:active,
.btn.pressed {
    transform: scale(0.95);
}

.btn.stop {
    background: #e94560;
}

.btn.stop:hover {
    background: #ff5a75;
}

.btn.forward {
    background: #4ecca3;
    color: #1a1a2e;
}

.btn.forward:hover {
    background: #5fd9b0;
}

.btn.toggle {
    width: 100%;
    margin-top: 12px;
    font-size: 0.9rem;
    padding: 10px;
    background: #0f3460;
}

.btn.toggle:hover {
    background: #1a4a7a;
}

.btn.toggle.active {
    background: #00bcd4;
    color: #1a1a2e;
}

.keyboard-hint {
    font-size: 0.75rem;
    color: #666;
    text-align: center;
    margin-top: 10px;
}

.keyboard-hint kbd {
    background: #0f3460;
    padding: 3px 8px;
    border-radius: 4px;
    margin: 0 3px;
    font-family: monospace;
    font-size: 0.8rem;
}

.divider {
    height: 1px;
    background: #333;
    margin: 16px 0;
}
</style>
