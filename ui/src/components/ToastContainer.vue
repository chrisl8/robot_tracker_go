<script setup lang="ts">
import { useUIStore } from '@/stores/uiStore'

const uiStore = useUIStore()
</script>

<template>
    <Teleport to="body">
        <div class="toast-container">
            <TransitionGroup name="toast">
                <div
                    v-for="toast in uiStore.toasts"
                    :key="toast.id"
                    class="toast"
                    :class="toast.type"
                >
                    {{ toast.message }}
                </div>
            </TransitionGroup>
        </div>
    </Teleport>
</template>

<style scoped>
.toast-container {
    position: fixed;
    bottom: 20px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 2000;
    display: flex;
    flex-direction: column;
    gap: 8px;
}

.toast {
    background: var(--panel-dark);
    color: var(--text-primary);
    padding: 12px 24px;
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.5);
    border: 1px solid var(--border-subtle);
    margin-top: 8px;
    font-family: var(--font-body);
}

.toast.success {
    border-color: var(--success-green);
    background: rgba(0, 230, 118, 0.1);
    color: var(--success-green);
}

.toast.error {
    border-color: var(--alert-red);
    background: rgba(255, 61, 0, 0.1);
    color: var(--alert-red);
}

.toast.warning {
    border-color: var(--warning-amber);
    background: rgba(255, 171, 0, 0.1);
    color: var(--warning-amber);
}

.toast.info {
    border-color: var(--accent-cyan);
    background: rgba(0, 217, 255, 0.1);
    color: var(--accent-cyan);
}

.toast-enter-active {
    animation: toastIn 0.3s ease-out;
}

.toast-leave-active {
    animation: toastOut 0.3s ease-in;
}

@keyframes toastIn {
    from {
        opacity: 0;
        transform: translateX(-50%) translateY(20px);
    }
    to {
        opacity: 1;
        transform: translateX(-50%) translateY(0);
    }
}

@keyframes toastOut {
    from {
        opacity: 1;
        transform: translateX(-50%) translateY(0);
    }
    to {
        opacity: 0;
        transform: translateX(-50%) translateY(-20px);
    }
}
</style>
