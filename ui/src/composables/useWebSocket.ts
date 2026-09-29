import { ref, onMounted, onUnmounted } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useUIStore } from '@/stores/uiStore'
import type { WebSocketMessage } from '@/types/api'

interface WebSocketOptions {
    baseDelay?: number
    maxDelay?: number
    onConnect?: () => void
    onDisconnect?: () => void
    onReconnect?: () => void
    onError?: (error: Event) => void
}

export function useWebSocket(url: string, options: WebSocketOptions = {}) {
    const {
        baseDelay = 1000,
        maxDelay = 5000,
        onConnect,
        onDisconnect,
        onReconnect,
        onError,
    } = options

    const robotStore = useRobotStore()
    const uiStore = useUIStore()

    const ws = ref<WebSocket | null>(null)
    const isConnected = ref(false)
    const attempts = ref(0)

    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null
    let hasConnectedOnce = false

    function connect(): void {
        if (ws.value?.readyState === WebSocket.OPEN) {
            return
        }

        try {
            ws.value = new WebSocket(url)

            ws.value.onopen = () => {
                const wasReconnect = hasConnectedOnce
                isConnected.value = true
                attempts.value = 0
                hasConnectedOnce = true
                if (wasReconnect) {
                    uiStore.addLogEntry('system', 'Reconnected to server')
                } else {
                    uiStore.addLogEntry('success', 'Connected to server')
                }
                onConnect?.()
                if (wasReconnect) {
                    onReconnect?.()
                }
            }

            ws.value.onclose = () => {
                isConnected.value = false
                uiStore.addLogEntry('error', 'Disconnected from server')
                onDisconnect?.()
                scheduleReconnect()
            }

            ws.value.onerror = error => {
                onError?.(error)
            }

            ws.value.onmessage = event => {
                try {
                    const data = JSON.parse(event.data) as WebSocketMessage
                    robotStore.handleWebSocketMessage(data)
                } catch (e) {
                    console.error('Failed to parse WebSocket message:', e)
                }
            }
        } catch (e) {
            console.error('WebSocket connection error:', e)
            scheduleReconnect()
        }
    }

    function scheduleReconnect(): void {
        attempts.value++
        const delay = Math.min(baseDelay * Math.pow(2, attempts.value - 1), maxDelay)
        reconnectTimeout = setTimeout(() => {
            connect()
        }, delay)
    }

    function disconnect(): void {
        if (reconnectTimeout) {
            clearTimeout(reconnectTimeout)
            reconnectTimeout = null
        }
        if (ws.value) {
            ws.value.close()
            ws.value = null
        }
        isConnected.value = false
    }

    onMounted(() => {
        connect()
    })

    onUnmounted(() => {
        disconnect()
    })

    return {
        ws,
        isConnected,
        connect,
        disconnect,
    }
}
