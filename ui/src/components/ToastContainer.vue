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
    background: #16213e;
    color: #eee;
    padding: 12px 24px;
    border-radius: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    border: 1px solid #0f3460;
    margin-top: 8px;
}

.toast.success {
    border-color: #4ecca3;
    background: rgba(78, 204, 163, 0.2);
    color: #4ecca3;
}

.toast.error {
    border-color: #e94560;
    background: rgba(233, 69, 96, 0.2);
    color: #e94560;
}

.toast.warning {
    border-color: #ffc107;
    background: rgba(255, 193, 7, 0.2);
    color: #ffc107;
}

.toast.info {
    border-color: #00bcd4;
    background: rgba(0, 188, 212, 0.2);
    color: #00bcd4;
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
