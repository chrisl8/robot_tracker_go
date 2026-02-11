# Robot Tracker Go - Implementation Plan

## Overview

Go implementation of the multi-robot tracking and control system, migrated from Python for improved performance and type safety.

## Current Phase Status

| Phase | Status | Description |
|-------|--------|-------------|
| Phase 1-10 | ✅ Complete | Foundation through Linux Migration |
| **Phase 11** | 🔄 **In Progress** | **UI Modernization (Vue 3 + TypeScript)** |
| Phase 12 | ⏳ Pending | Advanced Testing & CI/CD |

---

## Phase 11: UI Modernization - Vue 3 + TypeScript + Pinia

### Executive Summary

**Problem:** The current embedded HTML/CSS/JavaScript in `internal/ui/index.go` (1687 lines) has fundamental architectural flaws causing persistent bugs:

1. **State Fragmentation**: 5+ global variables (`currentDraw`, `obstacles`, `isDrawing`, etc.) scattered across 8 call sites
2. **Event-Driven Rendering**: `redrawOverlay()` called from 8 different places, causing race conditions
3. **Side Effects**: Canvas resize clears drawings silently
4. **Testing Difficulty**: No unit/integration tests possible
5. **Debugging Pain**: No syntax highlighting, DevTools don't work

**Solution:** Migrate to Vue 3 + TypeScript + Pinia with proper state management.

**Technologies Selected:**
- **Framework**: Vue 3 (Composition API + `<script setup>`)
- **Language**: TypeScript
- **State Management**: Pinia
- **Build Tool**: Vite
- **Testing**: Vitest (unit) + Playwright (E2E)

### Current Problems (Analyzed Feb 2026)

#### 1. State Fragmentation

```javascript
// Current: Scattered across file, no single source of truth
var obstacles = [];
var drawMode = false;
var drawStart = null;
var currentDraw = null;
var lastDrawnRect = null;
var isMouseDown = false;
var isDrawing = false;
```

**Problems:**
- Variables modified in 8+ different event handlers
- No clear ownership or lifecycle
- Race conditions between event handlers and WebSocket messages

#### 2. Event-Driven Rendering Bugs

**Bug UI-003 (Feb 7, 2026):** Obstacle drawing box disappeared when user paused mouse.

**Root Cause:**
```
1. User starts dragging → box appears
2. User pauses mouse → mousemove stops firing
3. 1 second later → syncCanvasSize() fires
4. syncCanvasSize() sets overlay.width → Canvas API CLEARS canvas
5. Box disappears
6. User moves mouse → redrawOverlay() restores box
```

**Fix Applied:**
```javascript
function syncCanvasSize() {
    var rect = video.getBoundingClientRect();
    if (rect.width > 0 && rect.height > 0) {
        if (overlay.width !== rect.width || overlay.height !== rect.height) {
            overlay.width = rect.width;
            overlay.height = rect.height;
            if (currentDraw) {
                redrawOverlay();
            }
        }
    }
}
```

**Why This Fix Was Fragile:**
- Added conditional check to prevent unnecessary clears
- But still reactive/event-driven architecture
- Future changes could reintroduce similar bugs

#### 3. No Testing Infrastructure

**Current State:**
- 0 unit tests for UI logic
- 0 integration tests for user flows
- 0 E2E tests for critical paths
- Manual testing only

**Consequence:** Bugs like UI-003, UI-004, UI-005 went undetected until user reported them.

### Recommended Solution: Vue 3 + TypeScript + Pinia

#### Architecture Benefits

| Aspect | Current (Vanilla JS) | Proposed (Vue 3 + TS) |
|--------|----------------------|------------------------|
| **State Management** | 8+ global variables | Single Pinia store |
| **Rendering** | Event-driven (8 call sites) | Reactive (automatic) |
| **Type Safety** | None | Full TypeScript |
| **Testing** | None | Unit + Integration + E2E |
| **Debugging** | Console.log | Vue DevTools |
| **Code Organization** | Single 1687-line file | Modular components |

#### Why Vue 3 Over Alternatives

| Technology | Assessment |
|------------|------------|
| **Vue 3 + Pinia** | ✅ Recommended - Best for this use case |
| Svelte 5 | ❌ Less ecosystem, fewer robotics examples |
| React | ✅ Valid alternative, but Vue has simpler learning curve |
| HTMX | ❌ Not suitable - real-time canvas drawing required |
| Vanilla JS + Refactor | ❌ Would still lack type safety and testing |

**Decision Factors:**
- Real-time WebSocket updates → Pinia handles elegantly
- Canvas overlay → Vue refs work well
- Component structure → Vue's 5-7 components map well
- Hiring/maintenance → More Vue developers available
- Learning curve → Composition API familiar to React developers

### Technology Stack Details

#### Vue 3 (Composition API)

```typescript
// Modern Vue 3 pattern with TypeScript
<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRobotStore } from '@/stores/robotStore'

const robotStore = useRobotStore()
const canvasRef = ref<HTMLCanvasElement | null>(null)

const confirmedTracks = computed(() => robotStore.confirmedTracks)

function onMouseDown(event: MouseEvent) {
    const point = getCanvasPoint(event)
    // Handle drawing...
}

onMounted(() => {
    initializeWebSocket()
})
</script>
```

**Benefits:**
- Type inference in templates
- `<script setup>` reduces boilerplate
- Native TypeScript support
- Vue DevTools integration

#### TypeScript

**Type Safety for Robotics:**

```typescript
// Types map directly to Go backend structures
interface Track {
    id: number
    bbox: [number, number, number, number]
    confidence: number
    tag_id?: number
    state: 'pending' | 'confirmed' | 'lost'
    history: [number, number][]
}

interface Obstacle {
    id: string
    name: string
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    world_top_left: [number, number]
    world_bottom_right: [number, number]
    clearance: number
}

interface WebSocketMessage {
    type: 'track' | 'tracks' | 'obstacles' | 'status' | 'calibration'
    // Discriminated union based on type
    track?: Track
    tracks?: Track[]
    obstacles?: { obstacles: Obstacle[]; count: number }
    status?: RobotStatus
    calibration?: CalibrationState
}
```

**Benefits:**
- Compile-time validation
- IDE autocomplete
- Safe refactoring
- Self-documenting code

#### Element Plus (Component Library)

**Why Element Plus:**
- Professional enterprise UI components
- Consistent design language
- Includes: tables, forms, modals, buttons, inputs, dialogs
- Good Vue 3 support
- ~200KB bundle size (lighter than Vuetify)
- Excellent documentation

**Components to Use:**
- `el-dialog` - Calibration wizard modal
- `el-button` - All buttons
- `el-input` - Form inputs (tag size)
- `el-table` - Obstacle list
- `el-card` - Panel containers
- `el-message` - Toast notifications

#### Pinia State Management

```typescript
// stores/robotStore.ts
import { defineStore } from 'pinia'
import { ref, computed } from 'vue'

export interface RobotStatus {
    connected: boolean
    fps: number
    robotCount: number
    arduinoState: string
}

export const useRobotStore = defineStore('robot', () => {
    // State
    const tracks = ref<Track[]>([])
    const status = ref<RobotStatus>({ /*...*/ })

    // Computed
    const confirmedTracks = computed(() =>
        tracks.value.filter(t => t.state === 'confirmed')
    )

    // Actions
    function handleWebSocketMessage(data: WebSocketMessage): void {
        switch(data.type) {
            case 'track':
                updateTrack(data.track!)
                break
            case 'tracks':
                tracks.value = data.tracks!
                break
            // ... other cases
        }
    }

    return { tracks, status, confirmedTracks, handleWebSocketMessage }
})
```

**Benefits:**
- Single source of truth
- DevTools integration
- Type-safe stores
- Easy testing

#### Canvas Rendering (Native API)

**Why Native Canvas API:**
- Lightweight (~0KB overhead)
- Simple operations: rectangles, circles, text
- Vue refs provide reactive binding
- No additional dependencies

**Pattern:**
```typescript
<canvas ref="canvasRef"></canvas>

const canvasRef = ref<HTMLCanvasElement | null>(null)
const ctx = computed(() => canvasRef.value?.getContext('2d'))

function drawObstacle(obs: Obstacle) {
    ctx.value?.strokeStyle = '#ff6b6b'
    ctx.value?.strokeRect(obs.pixel_top_left[0], obs.pixel_top_left[1],
                          width, height)
}
```

#### WebSocket with Reconnection

**Pattern: Exponential Backoff with Max Attempts**
```typescript
function connectWebSocket(url: string) {
    let attempts = 0
    const maxAttempts = 5
    const baseDelay = 1000

    function connect() {
        ws = new WebSocket(url)

        ws.onclose = () => {
            attempts++
            if (attempts < maxAttempts) {
                const delay = baseDelay * Math.pow(2, attempts - 1)
                setTimeout(connect, delay)
            }
        }

        ws.onmessage = (event) => {
            const data = JSON.parse(event.data)
            robotStore.handleWebSocketMessage(data)
        }
    }

    connect()
}
```

#### Server-Based State Persistence

**Pattern: Server is Source of Truth**
- Obstacles saved to server: `POST /api/obstacles/save`
- Obstacles loaded from server: `GET /api/obstacles`
- No localStorage persistence
- Server writes to YAML files (`config/obstacles.yaml`)

**Benefits:**
- Consistent across devices
- Persists beyond browser session
- Already implemented in Go backend

### File Structure

```
robot_tracker_go/
├── cmd/main.go                          # Go entry point (unchanged)
├── internal/                            # Go packages (unchanged)
│   ├── config/
│   ├── camera/
│   ├── detection/
│   ├── tracking/
│   ├── planning/
│   ├── controller/
│   └── ui/
│       ├── webserver.go               # REST + WebSocket server
│       ├── types.go                   # API types
│       └── embed.go                   # Static file embedding
├── ui/                                # NEW: Vue 3 frontend
│   ├── package.json                   # Vue + Element Plus + Pinia
│   ├── tsconfig.json                  # TypeScript config
│   ├── vite.config.ts                 # Vite build config
│   ├── public/
│   │   └── favicon.ico
│   └── src/
│       ├── main.ts                    # App entry point + Element Plus
│       ├── App.vue                    # Root component
│       ├── types/
│       │   ├── api.ts                 # WebSocket API types
│       │   ├── robot.ts               # Track, RobotStatus types
│       │   ├── obstacle.ts            # Obstacle types
│       │   └── ui.ts                  # UI state types
│       ├── stores/
│       │   ├── robotStore.ts          # Pinia: tracks, status
│       │   ├── obstacleStore.ts       # Pinia: obstacles, drawing
│       │   └── uiStore.ts             # Pinia: panels, toasts
│       ├── composables/
│       │   ├── useWebSocket.ts        # WebSocket + exponential backoff
│       │   ├── useCanvas.ts           # Canvas rendering
│       │   └── useDrawing.ts          # Drawing logic
│       ├── components/
│       │   ├── VideoOverlay.vue       # Canvas + WebSocket display
│       │   ├── ObstaclePanel.vue      # Element Plus: obstacle management
│       │   ├── TrackList.vue          # Robot tracking list
│       │   ├── ControlPanel.vue       # Manual control buttons
│       │   ├── CalibrationWizard.vue  # Element Plus dialog
│       │   ├── StatusBar.vue          # FPS, connection status
│       │   └── ToastNotification.vue  # Element Plus el-message
│       ├── styles/
│       │   ├── variables.scss         # CSS variables (colors, spacing)
│       │   └── overrides.scss         # Element Plus customizations
│       └── index.scss                 # Main stylesheet
├── scripts/                           # Build scripts (unchanged)
├── config/                            # YAML configs (unchanged)
└── assets/                            # ONNX models (unchanged)
```

### Technology Decisions Summary

| Aspect | Decision | Rationale |
|--------|----------|-----------|
| **Framework** | Vue 3 (Composition API) | Best real-time WebSocket patterns |
| **Language** | TypeScript | Catches bugs like float→int issue |
| **State Management** | Pinia | Official Vue store, type-safe |
| **Component Library** | Element Plus | Professional enterprise UI, consistent design |
| **Canvas Rendering** | Native API + Vue refs | Lightweight, sufficient for rectangles/text |
| **WebSocket** | Exponential backoff | Reliable reconnection with max attempts |
| **State Persistence** | Server only (YAML files) | Already implemented, single source of truth |
| **CSS Strategy** | Extract current + SCSS | Preserve look, add variables for theming |
| **Router** | None (single-page) | Only one main view, modals for workflows |

### Implementation Tasks

#### Phase 11.1: Project Setup (2-3 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.1.1 | Create Vue 3 + TypeScript + Vite project | 30 min |
| 11.1.2 | Install Element Plus + Pinia | 15 min |
| 11.1.3 | Configure TypeScript (tsconfig.json) | 15 min |
| 11.1.4 | Set up Pinia store infrastructure | 30 min |
| 11.1.5 | Configure Vite build options | 15 min |
| 11.1.6 | Extract current CSS to SCSS files | 30 min |
| 11.1.7 | Set up Vitest for unit tests | 30 min |
| 11.1.8 | Set up Playwright for E2E tests | 30 min |

#### Phase 11.2: Type Definitions (2-3 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.2.1 | Create api.ts (WebSocket message types) | 30 min |
| 11.2.2 | Create robot.ts (Track, Status types) | 30 min |
| 11.2.3 | Create obstacle.ts (Obstacle types) | 30 min |
| 11.2.4 | Create ui.ts (Panel, Toast types) | 30 min |
| 11.2.5 | Validate types against Go backend | 30 min |

#### Phase 11.3: State Management (3-4 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.3.1 | Implement robotStore.ts | 45 min |
| 11.3.2 | Implement obstacleStore.ts | 45 min |
| 11.3.3 | Implement uiStore.ts | 30 min |
| 11.3.4 | Create useWebSocket composable with exponential backoff | 45 min |
| 11.3.5 | Create useCanvas composable | 30 min |
| 11.3.6 | Test stores with Vitest | 30 min |

#### Phase 11.4: Components (5-7 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.4.1 | Create VideoOverlay.vue | 60 min |
| 11.4.2 | Create ObstaclePanel.vue | 45 min |
| 11.4.3 | Create TrackList.vue | 30 min |
| 11.4.4 | Create ControlPanel.vue | 30 min |
| 11.4.5 | Create CalibrationWizard.vue | 45 min |
| 11.4.6 | Create StatusBar.vue | 15 min |
| 11.4.7 | Create ToastNotification.vue | 15 min |
| 11.4.8 | Assemble App.vue | 30 min |
| 11.4.9 | Test components with Vue Test Utils | 60 min |

#### Phase 11.5: Integration (2-3 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.5.1 | Connect WebSocket to Pinia stores | 30 min |
| 11.5.2 | Implement obstacle drawing flow | 45 min |
| 11.5.3 | Implement calibration wizard flow | 30 min |
| 11.5.4 | Add keyboard shortcuts | 15 min |
| 11.5.5 | Verify all user flows work | 30 min |

#### Phase 11.6: Build Integration (1-2 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.6.1 | Configure Vite for static output | 15 min |
| 11.6.2 | Create embed.go for Go serving | 30 min |
| 11.6.3 | Update webserver.go to serve Vue build | 15 min |
| 11.6.4 | Test full build process | 30 min |
| 11.6.5 | Verify demo mode works | 15 min |

#### Phase 11.7: Testing (10-15 hours)

| Task | Description | Est. Time |
|------|-------------|-----------|
| 11.7.1 | Write unit tests for stores | 60 min |
| 11.7.2 | Write unit tests for composables | 60 min |
| 11.7.3 | Write component tests | 120 min |
| 11.7.4 | Write integration tests (Playwright) | 180 min |
| 11.7.5 | Write E2E critical path tests | 120 min |
| 11.7.6 | Configure CI/CD pipeline | 60 min |

### Testing Strategy

#### Testing Pyramid

```
                    ┌─────────────┐
                   /   E2E Tests   \        ~10% (Critical user flows)
                  /    Playwright   \
                 /───────────────────\
                /   Integration Tests  \    ~30% (Component interactions)
               /   Vitest + Vue Test    \
              /     Utils + Pinia        \
             /────────────────────────────\
            /       Unit Tests             \  ~60% (Store logic, composables)
           /      Vitest + TypeScript      \
          /────────────────────────────────────\
```

#### Recommended Testing Stack

| Layer | Tool | Purpose |
|-------|------|---------|
| **Unit Tests** | Vitest | Store logic, composables, utilities |
| **Component Tests** | Vue Test Utils + Vitest | Individual component behavior |
| **Integration Tests** | Playwright | Full user flows |
| **E2E Tests** | Playwright | Critical paths |

#### Why Playwright Over Cypress?

| Factor | Playwright | Cypress |
|--------|-----------|---------|
| **Multi-tab support** | ✅ Yes | ❌ Limited |
| **WebSocket testing** | ✅ Native support | ⚠️ Requires plugins |
| **Mobile testing** | ✅ Built-in | ⚠️ Paid feature |
| **Vue 3 support** | ✅ Excellent | ✅ Good |
| **Auto-waiting** | ✅ Intelligent | ✅ Good |
| **NPM downloads (2024)** | ✅ 4M+ weekly | ~2M weekly |
| **Test parallelization** | ✅ Free, native | ⚠️ Paid |

#### Unit Test Examples

**Store Test (Vitest):**

```typescript
// tests/stores/robotStore.spec.ts
import { describe, it, expect, vi, beforeEach } from 'vitest'
import { setActivePinia, createPinia } from 'pinia'
import { useRobotStore } from '@/stores/robotStore'

describe('RobotStore', () => {
    beforeEach(() => {
        setActivePinia(createPinia())
    })
    
    it('should update track when received via WebSocket', () => {
        const store = useRobotStore()
        
        const track = {
            id: 1,
            bbox: [100, 100, 200, 200] as [number, number, number, number],
            confidence: 0.95,
            state: 'confirmed' as const,
            history: []
        }
        
        store.handleWebSocketMessage({ type: 'track', track })
        
        expect(store.tracks).toHaveLength(1)
        expect(store.tracks[0].id).toBe(1)
        expect(store.confirmedTracks).toHaveLength(1)
    })
    
    it('should replace all tracks on tracks message', () => {
        const store = useRobotStore()
        
        const tracks = [
            { id: 1, bbox: [0, 0, 50, 50] as [number, number, number, number], confidence: 0.9, state: 'confirmed' as const, history: [] },
            { id: 2, bbox: [100, 100, 150, 150] as [number, number, number, number], confidence: 0.8, state: 'pending' as const, history: [] }
        ]
        
        store.handleWebSocketMessage({ type: 'tracks', tracks })
        
        expect(store.tracks).toHaveLength(2)
        expect(store.trackCount).toBe(2)
    })
    
    it('should update status correctly', () => {
        const store = useRobotStore()
        
        const status = {
            connected: true,
            fps: 30,
            robotCount: 5,
            arduinoState: 'Connected'
        }
        
        store.handleWebSocketMessage({ type: 'status', status })
        
        expect(store.status.robotCount).toBe(5)
        expect(store.status.arduinoState).toBe('Connected')
    })
})
```

**Composable Test (Vitest):**

```typescript
// tests/composables/useObstacleDrawing.spec.ts
import { describe, it, expect, vi } from 'vitest'
import { useObstacleDrawing } from '@/composables/useObstacleDrawing'

describe('useObstacleDrawing', () => {
    it('should calculate drawRect correctly', () => {
        const obstacleStore = { addObstacle: vi.fn() }
        const { drawRect, onMouseDown, onMouseMove } = useObstacleDrawing(obstacleStore)
        
        onMouseDown({ x: 100, y: 100 })
        onMouseMove({ x: 200, y: 200 })
        
        expect(drawRect.value).toEqual({
            x1: 100, y1: 100,
            x2: 200, y2: 200
        })
    })
    
    it('should handle reverse drawing direction', () => {
        const obstacleStore = { addObstacle: vi.fn() }
        const { drawRect, onMouseDown, onMouseMove } = useObstacleDrawing(obstacleStore)
        
        onMouseDown({ x: 200, y: 200 })
        onMouseMove({ x: 100, y: 100 })
        
        expect(drawRect.value).toEqual({
            x1: 100, y1: 100,
            x2: 200, y2: 200
        })
    })
})
```

**Component Test (Vue Test Utils):**

```typescript
// tests/components/VideoOverlay.spec.ts
import { describe, it, expect } from 'vitest'
import { mount } from '@vue/test-utils'
import { createTestingPinia } from '@pinia/testing'
import VideoOverlay from '@/components/VideoOverlay.vue'

describe('VideoOverlay', () => {
    it('should render obstacles from store', () => {
        const wrapper = mount(VideoOverlay, {
            global: {
                plugins: [createTestingPinia({
                    initialState: {
                        obstacle: {
                            obstacles: [
                                {
                                    id: 'obs_1',
                                    pixel_top_left: [100, 100] as [number, number],
                                    pixel_bottom_right: [200, 200] as [number, number]
                                }
                            ]
                        }
                    }
                })]
            }
        })
        
        const canvas = wrapper.find('canvas')
        expect(canvas.exists()).toBe(true)
    })
    
    it('should start drawing on mousedown when in draw mode', async () => {
        const wrapper = mount(VideoOverlay, {
            global: {
                plugins: [createTestingPinia({
                    initialState: {
                        obstacle: { mode: true }
                    }
                })]
            }
        })
        
        const canvas = wrapper.find('canvas')
        await canvas.trigger('mousedown', { clientX: 150, clientY: 150 })
    })
})
```

**E2E Test (Playwright):**

```typescript
// tests/e2e/obstacle-drawing.spec.ts
import { test, expect, beforeEach, afterEach } from '@playwright/test'

test.describe('Obstacle Drawing Flow', () => {
    let wsServer: any
    
    beforeEach(async () => {
        wsServer = await startMockServer(3765)
    })
    
    afterEach(async () => {
        await wsServer.stop()
    })
    
    test('should draw obstacle and add to list', async ({ page }) => {
        await page.goto('http://localhost:9086')
        
        await page.click('button:has-text("Obstacles")')
        await page.click('button:has-text("Draw Obstacle")')
        
        const canvas = page.locator('#overlay')
        await canvas.boundingBox()
        
        await page.mouse.move(100, 100)
        await page.mouse.down()
        await page.mouse.move(200, 200, { steps: 10 })
        await page.mouse.up()
        
        await expect(page.locator('.obstacle-item').first()).toBeVisible()
    })
    
    test('should persist obstacles after WebSocket update', async ({ page }) => {
        await page.goto('http://localhost:9086')
        await page.click('button:has-text("Obstacles")')
        
        await page.waitForTimeout(2000)
        
        await expect(page.locator('.obstacle-item').first()).toBeVisible()
    })
})
```

#### Coverage Targets

| Type | Target | Focus Areas |
|------|--------|-------------|
| **Unit Tests** | 80% | Store actions, composables, utilities |
| **Component Tests** | 70% | Critical rendering, event handling |
| **Integration Tests** | 100% | All user workflows |

#### Critical Test Cases

| Priority | Test Case | Type |
|----------|-----------|------|
| P0 | Obstacle drawing and persistence | Integration |
| P0 | WebSocket reconnection | E2E |
| P0 | Drawing box doesn't disappear on pause | Integration |
| P1 | Obstacle panel toggle | Component |
| P1 | Track list updates | Integration |
| P1 | Calibration wizard flow | E2E |
| P2 | Keyboard shortcuts | Integration |
| P2 | Toast notifications | Component |

#### Effort Estimate

| Test Type | Initial Setup | Tests (Estimated) | Maintenance |
|-----------|---------------|-------------------|-------------|
| **Unit Tests** | 4 hours | 30 tests @ 0.5h each = 15 hours | Low |
| **Component Tests** | 2 hours | 20 tests @ 0.5h each = 10 hours | Medium |
| **Integration Tests** | 4 hours | 15 tests @ 1h each = 15 hours | Medium |
| **Total** | **10 hours** | **60 tests** | **Ongoing** |

### Rollout Strategy

#### Phase 11.7: Deployment Options

**Option A: Static Build + Go Embed (Recommended)**

```
1. npm run build → generates static/ directory
2. Go embeds static/ into binary
3. Single binary deployment
```

**Option B: Separate Frontend Server**

```
1. npm run dev → Vite dev server (localhost:5173)
2. Go server proxies / → Vite dev server
3. Production: npm run build → nginx serves static
```

**Recommended: Option A** - Maintains single-binary deployment.

### Migration Checklist

| Item | Status | Notes |
|------|--------|-------|
| Create Vue 3 + TypeScript project | ⏳ Pending | Phase 11.1 |
| Set up Pinia stores | ⏳ Pending | Phase 11.3 |
| Migrate HTML to Vue components | ⏳ Pending | Phase 11.4 |
| Migrate CSS to scoped/CSS modules | ⏳ Pending | Phase 11.4 |
| Implement WebSocket in Pinia | ⏳ Pending | Phase 11.5 |
| Add obstacle drawing with Vue | ⏳ Pending | Phase 11.5 |
| Add calibration wizard Vue | ⏳ Pending | Phase 11.5 |
| Configure Vite + Go embed | ⏳ Pending | Phase 11.6 |
| Write unit tests (Vitest) | ⏳ Pending | Phase 11.7 |
| Write E2E tests (Playwright) | ⏳ Pending | Phase 11.7 |
| CI/CD pipeline setup | ⏳ Pending | Phase 11.7 |
| Production deployment test | ⏳ Pending | Phase 11.7 |

### Effort Summary

| Phase | Tasks | Time |
|-------|-------|------|
| 11.1 | Project Setup | 2-3 hours |
| 11.2 | Type Definitions | 2-3 hours |
| 11.3 | State Management | 3-4 hours |
| 11.4 | Components | 5-7 hours |
| 11.5 | Integration | 2-3 hours |
| 11.6 | Build Integration | 1-2 hours |
| 11.7 | Testing | 10-15 hours |
| **Total** | - | **25-37 hours** |

---

## Phase 12: Advanced Testing & CI/CD (Future)

### Planned Improvements

| Item | Description |
|------|-------------|
| GitHub Actions CI | Automated test on every push |
| Docker multi-stage build | Optimized container builds |
| Automated browser testing | Cross-browser verification |
| Performance benchmarks | Track FPS, latency over time |
| Visual regression tests | Detect UI changes |

---

## Historical Phases (Complete)

### Phase 1-10: Completed

See PLAN.md sections 1-10 for complete documentation of:
- Foundation (Config, Serial Protocol, Arduino Controller, Command Queue)
- Core Mathematics (Position types, Homography, Position Estimator)
- Detection Pipeline (AprilTag + YOLO)
- Tracking (ByteTrack, Kalman Filter, Hungarian Algorithm)
- Path Planning (A*, Local Planner, Coordinator, Collision Detection)
- Web UI (Gin Web Server, MJPEG Streaming, WebSocket Overlay)
- Integration & Testing
- Web-Based Calibration
- Obstacle Detection (YOLO + Static Obstacles + Path Planning)
- Linux Migration

---

## References

- Vue 3 Documentation: https://vuejs.org/
- TypeScript Documentation: https://www.typescriptlang.org/
- Pinia Documentation: https://pinia.vuejs.org/
- Vitest Documentation: https://vitest.dev/
- Playwright Documentation: https://playwright.dev/
- Vite Documentation: https://vitejs.dev/
- Go Embed Package: https://pkg.go.dev/embed
- Original Python implementation: `C:\Dev\robot_tracker\`
- GoCV documentation: https://gocv.io/
- AprilTag library: https://github.com/AprilRobotics/apriltag
