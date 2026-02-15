<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useUIStore } from '@/stores/uiStore'
import { useRobotStore } from '@/stores/robotStore'
import { Gamepad2 } from 'lucide-vue-next'

const uiStore = useUIStore()
const robotStore = useRobotStore()

const hasDestinationPending = computed(
    () => robotStore.destination !== null && robotStore.controlMode !== 'autonomous'
)

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
        <h3><Gamepad2 :size="14" /> Controls</h3>

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
                    :class="{
                        active: robotStore.controlMode === 'autonomous',
                        'needs-attention': hasDestinationPending,
                    }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('autonomous')"
                >
                    Auto
                </button>
            </div>
        </div>

        <!-- Hint: destination set but not in auto mode -->
        <div
            v-if="hasDestinationPending"
            class="auto-hint"
            @click="robotStore.setControlMode('autonomous')"
        >
            Destination set — switch to <strong>Auto</strong> to start navigation
        </div>

        <div class="divider"></div>

        <!-- WASD Controls -->
        <div
            class="controls"
            :class="{
                disabled: robotStore.controlMode !== 'manual' || robotStore.emergencyStopped,
            }"
        >
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

.emergency-stop {
    width: 100%;
    padding: 16px;
    font-size: 1.1rem;
    font-weight: bold;
    background: rgba(255, 61, 0, 0.2);
    color: var(--alert-red);
    border: 2px solid var(--alert-red);
    border-radius: 8px;
    cursor: pointer;
    text-transform: uppercase;
    letter-spacing: 1px;
    margin-bottom: 12px;
}

.emergency-stop:hover {
    background: rgba(255, 61, 0, 0.35);
}

.emergency-stop:active {
    transform: scale(0.98);
}

.estop-active {
    margin-bottom: 12px;
}

.estop-banner {
    background: var(--alert-red);
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
    from {
        opacity: 1;
    }
    to {
        opacity: 0.7;
    }
}

.estop-clear {
    width: 100%;
    padding: 10px;
    background: var(--bg-slate);
    color: var(--text-primary);
    border: 1px solid var(--border-subtle);
    border-radius: 0 0 8px 8px;
    cursor: pointer;
    font-size: 0.9rem;
}

.estop-clear:hover {
    background: var(--panel-dark);
}

.mode-selector {
    margin-bottom: 12px;
}

.mode-label {
    font-size: 0.75rem;
    color: var(--text-dim);
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
    min-width: 0;
    padding: 8px 4px;
    font-size: 0.8rem;
    background: var(--bg-slate);
    border: 2px solid var(--border-subtle);
    border-radius: 6px;
    color: var(--text-dim);
    cursor: pointer;
    transition: all 0.15s ease;
    overflow: hidden;
    text-overflow: ellipsis;
}

.mode-btn:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.08);
    color: var(--text-primary);
}

.mode-btn.active {
    background: var(--accent-cyan);
    color: var(--bg-deep-space);
    border-color: var(--accent-cyan);
    font-weight: 600;
    box-shadow: 0 0 8px rgba(0, 217, 255, 0.4);
}

.mode-btn.needs-attention {
    animation: pulse-auto 1.5s ease-in-out infinite;
    border-color: var(--warning-amber);
    color: var(--warning-amber);
}

@keyframes pulse-auto {
    0%,
    100% {
        box-shadow: 0 0 0 0 rgba(255, 171, 0, 0.4);
    }
    50% {
        box-shadow: 0 0 0 4px rgba(255, 171, 0, 0);
    }
}

.mode-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
}

.auto-hint {
    background: rgba(255, 171, 0, 0.1);
    color: var(--warning-amber);
    font-size: 0.75rem;
    padding: 8px 10px;
    border-radius: 6px;
    margin-top: 8px;
    cursor: pointer;
    text-align: center;
    border: 1px solid rgba(255, 171, 0, 0.3);
}

.auto-hint:hover {
    background: rgba(255, 171, 0, 0.2);
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
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    cursor: pointer;
    font-size: 1.3rem;
    transition: all 0.15s ease;
    background: var(--panel-dark);
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
}

.btn:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.1);
    border-color: rgba(0, 217, 255, 0.3);
    color: var(--text-primary);
    transform: translateY(-1px);
}

.btn:active:not(:disabled),
.btn.pressed {
    transform: scale(0.95);
    box-shadow: 0 0 10px rgba(0, 217, 255, 0.4);
}

.btn:disabled {
    cursor: not-allowed;
}

.btn.stop {
    background: rgba(255, 61, 0, 0.15);
    border-color: rgba(255, 61, 0, 0.3);
    color: var(--alert-red);
}

.btn.stop:hover:not(:disabled) {
    background: rgba(255, 61, 0, 0.25);
}

.btn.forward {
    background: rgba(0, 217, 255, 0.15);
    border-color: rgba(0, 217, 255, 0.3);
    color: var(--accent-cyan);
}

.btn.forward:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.25);
}

.btn.toggle {
    width: 100%;
    margin-top: 12px;
    font-size: 0.9rem;
    padding: 10px;
    background: var(--panel-dark);
}

.btn.toggle:hover {
    background: rgba(0, 217, 255, 0.1);
}

.btn.toggle.active {
    background: var(--accent-cyan);
    color: var(--bg-deep-space);
    border-color: var(--accent-cyan);
}

.keyboard-hint {
    font-size: 0.75rem;
    color: var(--text-dim);
    text-align: center;
    margin-top: 10px;
}

.keyboard-hint kbd {
    background: var(--panel-dark);
    border: 1px solid var(--border-subtle);
    padding: 3px 8px;
    border-radius: 4px;
    margin: 0 3px;
    font-family: var(--font-data);
    font-size: 0.8rem;
    color: var(--text-secondary);
}

.divider {
    height: 1px;
    background: var(--border-subtle);
    margin: 16px 0;
}
</style>
