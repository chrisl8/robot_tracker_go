<script setup lang="ts">
import { ref, watch, onMounted } from 'vue'
import { useUIStore } from '@/stores/uiStore'
import { useRobotStore } from '@/stores/robotStore'

const uiStore = useUIStore()
const robotStore = useRobotStore()

const pressed = ref<string | null>(null)

onMounted(() => {
    robotStore.fetchControlState()
})

async function sendCommand(command: string): Promise<void> {
    try {
        await fetch('/api/command', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ command }),
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
    keys => {
        if (robotStore.controlMode !== 'manual' || robotStore.emergencyStopped) return
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

        <!-- Emergency Stop -->
        <button
            v-if="!robotStore.emergencyStopped"
            class="btn emergency-stop"
            @click="robotStore.emergencyStop()"
        >
            EMERGENCY STOP
        </button>
        <div v-else class="estop-active">
            <div class="estop-banner">E-STOP ACTIVE</div>
            <button class="btn estop-clear" @click="robotStore.clearEmergencyStop()">
                Clear E-Stop
            </button>
        </div>

        <!-- Mode Selector -->
        <div class="mode-selector">
            <span class="mode-label">Mode:</span>
            <div class="mode-buttons">
                <button
                    class="btn mode-btn"
                    :class="{ active: robotStore.controlMode === 'idle' }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('idle')"
                >
                    Idle
                </button>
                <button
                    class="btn mode-btn"
                    :class="{ active: robotStore.controlMode === 'manual' }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('manual')"
                >
                    Manual
                </button>
                <button
                    class="btn mode-btn"
                    :class="{ active: robotStore.controlMode === 'autonomous' }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('autonomous')"
                >
                    Auto
                </button>
            </div>
        </div>

        <div class="divider"></div>

        <!-- WASD Controls -->
        <div class="controls" :class="{ disabled: robotStore.controlMode !== 'manual' || robotStore.emergencyStopped }">
            <button
                class="btn forward"
                @mousedown="handleMouseDown('w', 'F')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'w' }"
                :disabled="robotStore.controlMode !== 'manual' || robotStore.emergencyStopped"
            >
                W
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('a', 'L')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'a' }"
                :disabled="robotStore.controlMode !== 'manual' || robotStore.emergencyStopped"
            >
                A
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('s', 'B')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 's' }"
                :disabled="robotStore.controlMode !== 'manual' || robotStore.emergencyStopped"
            >
                S
            </button>
            <button
                class="btn"
                @mousedown="handleMouseDown('d', 'R')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :class="{ pressed: pressed === 'd' }"
                :disabled="robotStore.controlMode !== 'manual' || robotStore.emergencyStopped"
            >
                D
            </button>
            <button
                class="btn stop"
                @mousedown="handleMouseDown('x', 'S')"
                @mouseup="handleMouseUp"
                @mouseleave="handleMouseUp"
                :disabled="robotStore.controlMode !== 'manual' || robotStore.emergencyStopped"
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

.emergency-stop {
    width: 100%;
    padding: 16px;
    font-size: 1.1rem;
    font-weight: bold;
    background: #c0392b;
    color: #fff;
    border: 2px solid #e74c3c;
    border-radius: 8px;
    cursor: pointer;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 12px;
}

.emergency-stop:hover {
    background: #e74c3c;
}

.emergency-stop:active {
    transform: scale(0.98);
}

.estop-active {
    margin-bottom: 12px;
}

.estop-banner {
    background: #e74c3c;
    color: #fff;
    text-align: center;
    padding: 10px;
    font-weight: bold;
    font-size: 1rem;
    border-radius: 8px 8px 0 0;
    letter-spacing: 1px;
    animation: pulse-estop 1s ease-in-out infinite alternate;
}

@keyframes pulse-estop {
    from { opacity: 1; }
    to { opacity: 0.7; }
}

.estop-clear {
    width: 100%;
    padding: 10px;
    background: #2c3e50;
    color: #ecf0f1;
    border: 1px solid #7f8c8d;
    border-radius: 0 0 8px 8px;
    cursor: pointer;
    font-size: 0.9rem;
}

.estop-clear:hover {
    background: #34495e;
}

.mode-selector {
    margin-bottom: 12px;
}

.mode-label {
    font-size: 0.75rem;
    color: #888;
    text-transform: uppercase;
    letter-spacing: 0.5px;
    display: block;
    margin-bottom: 6px;
}

.mode-buttons {
    display: flex;
    gap: 6px;
}

.mode-btn {
    flex: 1;
    padding: 8px 4px;
    font-size: 0.8rem;
    background: #16213e;
    border: 1px solid #333;
    border-radius: 6px;
    color: #aaa;
    cursor: pointer;
    transition: all 0.15s ease;
}

.mode-btn:hover:not(:disabled) {
    background: #1a3a5c;
    color: #ddd;
}

.mode-btn.active {
    background: #0f3460;
    color: #fff;
    border-color: #4ecca3;
}

.mode-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

.controls {
    display: grid;
    grid-template-columns: repeat(3, 1fr);
    gap: 8px;
    max-width: 180px;
    margin: 0 auto;
    transition: opacity 0.2s ease;
}

.controls.disabled {
    opacity: 0.4;
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

.btn:hover:not(:disabled) {
    background: #1a4a7a;
    transform: translateY(-1px);
}

.btn:active:not(:disabled),
.btn.pressed {
    transform: scale(0.95);
}

.btn:disabled {
    cursor: not-allowed;
}

.btn.stop {
    background: #e94560;
}

.btn.stop:hover:not(:disabled) {
    background: #ff5a75;
}

.btn.forward {
    background: #4ecca3;
    color: #1a1a2e;
}

.btn.forward:hover:not(:disabled) {
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
