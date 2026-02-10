import { ref, onMounted, onUnmounted } from 'vue'
import { useRobotStore } from '@/stores/robotStore'
import { useUIStore } from '@/stores/uiStore'
import type { WebSocketMessage } from '@/types/api'

interface WebSocketOptions {
    maxAttempts?: number
    baseDelay?: number
    onConnect?: () => void
    onDisconnect?: () => void
    onError?: (error: Event) => void
}

export function useWebSocket(url: string, options: WebSocketOptions = {}) {
    const { maxAttempts = 5, baseDelay = 1000, onConnect, onDisconnect, onError } = options

    const robotStore = useRobotStore()
    const uiStore = useUIStore()

    const ws = ref<WebSocket | null>(null)
    const isConnected = ref(false)
    const attempts = ref(0)

    let reconnectTimeout: ReturnType<typeof setTimeout> | null = null

    function connect(): void {
        if (ws.value?.readyState === WebSocket.OPEN) {
            return
        }

        try {
            ws.value = new WebSocket(url)

            ws.value.onopen = () => {
                isConnected.value = true
                attempts.value = 0
                onConnect?.()
            }

            ws.value.onclose = () => {
                isConnected.value = false
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
        if (attempts.value >= maxAttempts) {
            uiStore.showToast('Connection failed. Please refresh the page.', 'error')
            return
        }

        attempts.value++
        const delay = baseDelay * Math.pow(2, attempts.value - 1)
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

    function send(data: unknown): void {
        if (ws.value?.readyState === WebSocket.OPEN) {
            ws.value.send(JSON.stringify(data))
        }
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
        send,
    }
}
