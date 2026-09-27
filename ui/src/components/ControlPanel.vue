<script setup lang="ts">
import { ref, computed, watch, onMounted } from 'vue'
import { useUIStore } from '@/stores/uiStore'
import { useRobotStore } from '@/stores/robotStore'
import { ArrowUp, ArrowDown, ArrowLeft, ArrowRight, Gamepad2 } from 'lucide-vue-next'

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
        } else {
            sendCommand('S')
        }
    },
    { deep: true }
)
</script>

<template>
    <div class="panel">
        <h3><Gamepad2 :size="14" /> Direct Control</h3>

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
                    :class="{ active: robotStore.controlMode === 'hold' }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('hold')"
                >
                    Hold
                </button>
                <button
                    class="btn mode-btn"
                    :class="{ active: robotStore.controlMode === 'manual' }"
                    :disabled="robotStore.emergencyStopped"
                    @click="robotStore.setControlMode('manual')"
                >
                    Pilot
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

        <!-- D-Pad Controls (only visible in Pilot mode) -->
        <template v-if="robotStore.controlMode === 'manual'">
            <div class="dpad">
                <div class="dpad-row">
                    <button
                        class="dpad-btn forward"
                        @mousedown="handleMouseDown('w', 'F')"
                        @mouseup="handleMouseUp"
                        @mouseleave="handleMouseUp"
                        :class="{ pressed: pressed === 'w' }"
                        :disabled="robotStore.emergencyStopped"
                    >
                        <ArrowUp :size="20" :stroke-width="2.5" />
                    </button>
                </div>
                <div class="dpad-row">
                    <button
                        class="dpad-btn"
                        @mousedown="handleMouseDown('a', 'L')"
                        @mouseup="handleMouseUp"
                        @mouseleave="handleMouseUp"
                        :class="{ pressed: pressed === 'a' }"
                        :disabled="robotStore.emergencyStopped"
                    >
                        <ArrowLeft :size="20" :stroke-width="2.5" />
                    </button>
                    <button
                        class="dpad-btn stop-btn"
                        @mousedown="handleMouseDown('x', 'S')"
                        @mouseup="handleMouseUp"
                        @mouseleave="handleMouseUp"
                        :disabled="robotStore.emergencyStopped"
                    >
                        STOP
                    </button>
                    <button
                        class="dpad-btn"
                        @mousedown="handleMouseDown('d', 'R')"
                        @mouseup="handleMouseUp"
                        @mouseleave="handleMouseUp"
                        :class="{ pressed: pressed === 'd' }"
                        :disabled="robotStore.emergencyStopped"
                    >
                        <ArrowRight :size="20" :stroke-width="2.5" />
                    </button>
                </div>
                <div class="dpad-row">
                    <button
                        class="dpad-btn backward"
                        @mousedown="handleMouseDown('s', 'B')"
                        @mouseup="handleMouseUp"
                        @mouseleave="handleMouseUp"
                        :class="{ pressed: pressed === 's' }"
                        :disabled="robotStore.emergencyStopped"
                    >
                        <ArrowDown :size="20" :stroke-width="2.5" />
                    </button>
                </div>
            </div>
            <div class="keyboard-hint">
                <kbd>W</kbd> <kbd>A</kbd> <kbd>S</kbd> <kbd>D</kbd> move · <kbd>X</kbd> stop
            </div>
        </template>
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
    font-family: var(--font-heading);
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
    font-family: var(--font-heading);
    font-size: 0.8rem;
    text-transform: uppercase;
    letter-spacing: 0.5px;
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

.dpad {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    margin: 0 auto;
    transition: opacity 0.2s ease;
}

.dpad-row {
    display: flex;
    gap: 6px;
    justify-content: center;
}

.dpad-btn {
    width: 44px;
    height: 44px;
    border: 1px solid var(--border-subtle);
    border-radius: 8px;
    cursor: pointer;
    font-size: 1rem;
    transition: all 0.15s ease;
    background: var(--panel-dark);
    color: var(--text-secondary);
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 0;
}

.dpad-btn:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.1);
    border-color: rgba(0, 217, 255, 0.3);
    color: var(--text-primary);
    transform: translateY(-1px);
}

.dpad-btn:active:not(:disabled),
.dpad-btn.pressed {
    transform: scale(0.95);
    box-shadow: 0 0 10px rgba(0, 217, 255, 0.4);
}

.dpad-btn:disabled {
    cursor: not-allowed;
}

.dpad-btn.forward,
.dpad-btn.backward {
    background: rgba(0, 217, 255, 0.08);
    border-color: rgba(0, 217, 255, 0.25);
    color: var(--accent-cyan);
}

.dpad-btn.forward:hover:not(:disabled),
.dpad-btn.backward:hover:not(:disabled) {
    background: rgba(0, 217, 255, 0.2);
}

.stop-btn {
    width: 44px;
    height: 44px;
    border-radius: 50%;
    background: rgba(255, 61, 0, 0.7);
    border: 2px solid var(--alert-red);
    color: #fff;
    font-size: 0.6rem;
    font-weight: 700;
    letter-spacing: 0.5px;
    text-transform: uppercase;
}

.stop-btn:hover:not(:disabled) {
    background: rgba(255, 61, 0, 0.9);
    border-color: var(--alert-red);
    color: #fff;
    transform: none;
    box-shadow: 0 0 12px rgba(255, 61, 0, 0.5);
}

.stop-btn:active:not(:disabled) {
    transform: scale(0.95);
    box-shadow: 0 0 16px rgba(255, 61, 0, 0.6);
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
