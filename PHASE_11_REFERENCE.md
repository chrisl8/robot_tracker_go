# Phase 11 Implementation Reference

## Current Go Backend Context

### Go Backend Files (Reference for Vue Integration)

| File | Purpose | Must Keep |
|------|---------|-----------|
| `internal/ui/webserver.go` | REST API + WebSocket server | ✅ Reference |
| `internal/ui/types.go` | API request/response types | ✅ Map to TypeScript |
| `cmd/main.go` | Entry point, calls `webServer.SetObstacles()` | ✅ Callback pattern |

### Current WebSocket Message Types (from Go)

```go
// webserver.go - OverlayMessage struct
type OverlayMessage struct {
    Type        string           `json:"type"`
    BBox        *BBoxMessage     `json:"bbox,omitempty"`
    Track       *TrackMessage    `json:"track,omitempty"`
    Path        *PathMessage     `json:"path,omitempty"`
    Status      *StatusMessage   `json:"status,omitempty"`
    Command     *CommandMessage  `json:"command,omitempty"`
    Calibration *CalibrationStatusMessage `json:"calibration,omitempty"`
    Obstacles   *ObstaclesMessage `json:"obstacles,omitempty"`
}
```

### Message Types to Support

| Type | Go Source | Vue Store | Priority |
|------|-----------|-----------|----------|
| `track` | webserver.go:175 | robotStore | P0 |
| `tracks` | webserver.go:175 | robotStore | P0 |
| `obstacles` | webserver.go:175 | obstacleStore | P0 |
| `status` | webserver.go:175 | robotStore | P0 |
| `bbox` | webserver.go:175 | (legacy) | P1 |
| `calibration` | webserver.go:175 | calibrationStore | P1 |
| `calibration_tags` | webserver.go:175 | calibrationStore | P1 |

### REST API Endpoints

| Method | Path | Purpose |
|--------|------|---------|
| POST | `/api/obstacles` | Add obstacle |
| DELETE | `/api/obstacles/:id` | Delete obstacle |
| POST | `/api/obstacles/clear` | Clear all |
| POST | `/api/obstacles/save` | Save to YAML |
| GET | `/api/obstacles` | List obstacles |
| POST | `/api/command` | Send command |
| POST | `/api/destination` | Set target |

---

## Visual Design Requirements

### Element Plus + Custom CSS

**Decision:** Use Element Plus components with custom SCSS for canvas and dashboard layout.

### Color Palette (Must Preserve)

Extract to `styles/variables.scss`:

```scss
// Element Plus theme overrides
--el-color-primary: #4ecca3;
--el-color-danger: #e94560;
--el-color-warning: #ffc107;
--el-color-info: #00bbd4;

// Dashboard colors
--background: #1a1a2e;
--surface: #16213e;
--obstacle: #ff6b6b;
--drawing: #ffa500;
```

### Element Plus Components to Use

| Component | Usage |
|-----------|-------|
| `el-dialog` | Calibration wizard modal |
| `el-button` | All buttons |
| `el-input` | Tag size input |
| `el-table` | Obstacle list |
| `el-card` | Panel containers |
| `el-message` | Toast notifications |
| `el-switch` | Draw mode toggle |
| `el-tag` | Status indicators |

### Custom Canvas Styles

**Obstacles:**
- Red dashed rectangles (`#ff6b6b`, `[5, 5]` dash)
- Light fill (`rgba(255, 107, 107, 0.1)`)
- "OBSTACLE" label above top-left corner

**Drawing Box:**
- Orange dashed rectangles (`#ffa500`, `[5, 5]` dash)
- Light fill (`rgba(255, 165, 0, 0.2)`)

**Tracks:**
- Colored bounding boxes based on track ID
- Label with ID and confidence
- History trails (optional)

---

## Keyboard Shortcuts (Must Preserve)

| Key | Action | Current Handler |
|-----|--------|-----------------|
| W/A/S/D | Move robot | `sendCommand()` |
| E | Weapon mode | `sendCommand()` |
| X | Emergency stop | `sendCommand()` |
| Z | Cancel drawing | `toggleDrawMode()` |
| C | Clear obstacles | `clearObstacles()` |

---

## Calibration Wizard Flow

### Current Flow (Must Preserve)

1. Click "Not Calibrated" badge → Opens modal
2. Enter tag size (default 0.15m)
3. Click "Detect Tags" → Step 2
4. Select detected tag from list or video
5. Click "Use Selected Tag" → Computes homography
6. Shows width/height results
7. Click "Done" → Closes modal

### WebSocket Calibration Messages

```typescript
// Calibration message types
type CalibrationStatus = {
    state: 'not_calibrated' | 'calibrating' | 'complete' | 'calibrated'
    message?: string
}

type CalibrationTagsMessage = {
    tags: DetectedTagInfo[]
    count: number
}

interface DetectedTagInfo {
    id: number
    center: [number, number]
    corners: [][2]number
}
```

---

## Known Bugs (Reference for Vue Fixes)

### Bug UI-003: Drawing Box Disappears on Pause

**Root Cause:** `syncCanvasSize()` sets `overlay.width` which clears canvas.

**Fix Pattern (Vue):** Don't manually manage canvas. Let Vue reactivity handle it:
```typescript
// WRONG (current approach):
overlay.width = newWidth  // Clears canvas!

// RIGHT (Vue approach):
// Use reactive state, canvas renders on every change automatically
```

### Bug UI-004: Obstacles Disappear After Adding

**Root Cause:** `redrawOverlay()` returned early when `currentDraw` was set, skipping obstacle rendering.

**Fix Pattern (Vue):** Layer-based rendering, no early returns:
```typescript
function render() {
    clearCanvas()
    renderObstacles()      // Always runs
    renderDrawingBox()     // If active
    renderTracks()         // Always runs
}
```

### Bug UI-005: Float Coordinates Rejected

**Root Cause:** Go expects `[2]int`, JavaScript sent floats.

**Fix Pattern (Vue):** TypeScript ensures integers:
```typescript
interface Point {
    x: number
    y: number
}

function addObstacle(topLeft: [number, number], bottomRight: [number, number]) {
    // TypeScript ensures these are numbers, validation ensures integers
    const x1 = Math.round(topLeft[0])
    // ...
}
```

---

## Current HTML Structure (For Migration)

### Main Layout

```html
<body>
    <header>
        <h1>Robot Tracker</h1>
        <div class="status">
            <span>FPS: <span id="fps">0</span></span>
            <span>Tracks: <span id="trackCount">0</span></span>
            <span id="calibrationBadge">Not Calibrated</span>
            <button class="obstacle-toggle">Obstacles</button>
            <span id="arduinoStatus">Arduino: Disconnected</span>
        </div>
    </header>
    
    <div class="main">
        <div class="video-container">
            <div class="video-wrapper">
                <img id="video" src="/stream">
                <canvas id="overlay"></canvas>
            </div>
        </div>
        
        <div class="sidebar">
            <div class="panel">Manual Control</div>
            <div class="panel">Statistics</div>
            <div class="panel">Detected Targets</div>
            <div class="panel" id="obstaclePanel">Static Obstacles</div>
            <div class="panel" id="instructionsPanel">Instructions</div>
        </div>
    </div>
    
    <div id="calibrationWizard">...</div>
    <div id="toast">...</div>
</body>
```

---

## Current JavaScript State (For Migration)

### State Variables to Migrate

```javascript
// GLOBAL STATE (Scattered - migrate to Pinia)
var obstacles = [];           // → obstacleStore.obstacles
var drawMode = false;         // → obstacleStore.drawingMode
var drawStart = null;         // → obstacleStore.drawing.startPoint
var currentDraw = null;       // → obstacleStore.drawing.currentPoint
var lastDrawnRect = null;    // → obstacleStore.drawing.lastRect
var isMouseDown = false;      // → obstacleStore.drawing.active (use window listeners)
var isDrawing = false;       // → obstacleStore.drawing.active

// TRACKING STATE
var tracks = [];               // → robotStore.tracks
var destinations = [];         // → robotStore.destinations
var detectedTags = [];        // → calibrationStore.detectedTags
var selectedTagId = null;    // → calibrationStore.selectedTagId

// SYSTEM STATE
var calibrationMode = false;   // → calibrationStore.mode
var isCalibrationPinned = false; // → calibrationStore.pinned
var arduinoConnected = false;  // → robotStore.status.arduinoState

// UI STATE
var obstaclesPanelOpen = false; // → uiStore.panels.obstacleOpen
```

### Event Handlers to Migrate

| Handler | Current Location | Vue Migration |
|---------|-----------------|---------------|
| `mousedown` (canvas) | index.go:1159 | VideoOverlay.vue |
| `mousemove` (canvas) | index.go:1175 | VideoOverlay.vue |
| `mouseup` (canvas) | index.go:1185 | VideoOverlay.vue |
| `click` (canvas) | index.go:1277 | VideoOverlay.vue |
| `mousedown` (window) | index.go:1219 | App.vue / composable |
| `mouseup` (window) | index.go:1222 | App.vue / composable |
| `keydown` (document) | index.go:1246 | useKeyboard.ts composable |
| `resize` (window) | index.go:1216 | useCanvas.ts composable |
| `click` (obstacle toggle) | index.go:1135 | ObstaclePanel.vue |
| `click` (draw button) | index.go:1145 | ObstaclePanel.vue |
| `click` (calibration badge) | index.go:1332 | Header.vue / CalibrationWizard.vue |

---

## Go → Vue Type Mapping

### Track Type

```go
// Go (internal/tracking/types.go)
type Track struct {
    ID         int
    Bbox       [4]int
    Confidence float64
    TagID      *int
    State      TrackState
    History    [][2]int
}

// Vue TypeScript
interface Track {
    id: number
    bbox: [number, number, number, number]
    confidence: number
    tag_id?: number
    state: 'pending' | 'confirmed' | 'lost'
    history: [number, number][]
}
```

### Obstacle Type

```go
// Go (internal/planning/static_obstacle.go)
type Obstacle struct {
    Name              string
    WorldTopLeft      [2]float64
    WorldBottomRight  [2]float64
    PixelsTopLeft    [2]int
    PixelsBottomRight [2]int
}

// Vue TypeScript
interface Obstacle {
    id: string
    name: string
    world_top_left: [number, number]
    world_bottom_right: [number, number]
    pixel_top_left: [number, number]
    pixel_bottom_right: [number, number]
    clearance: number
}
```

---

## Testing Reference

### Critical User Flows (For E2E Tests)

1. **Draw Obstacle Flow**
   - Open obstacle panel
   - Click "Draw Obstacle"
   - Click and drag on canvas
   - Release mouse
   - Verify obstacle appears in panel
   - Verify obstacle drawn on video

2. **Obstacle Persistence Flow**
   - Draw multiple obstacles
   - Wait for WebSocket updates
   - Verify obstacles remain visible
   - Click "Save"
   - Reload page
   - Verify obstacles persist

3. **Calibration Flow**
   - Click "Not Calibrated" badge
   - Enter tag size
   - Click "Detect Tags"
   - Select tag from list
   - Click "Use Selected Tag"
   - Verify calibration complete

### Unit Test Patterns

```typescript
// Store tests should verify:
// 1. WebSocket messages update state correctly
// 2. Computed values derive from state
// 3. Actions update state and trigger side effects

// Composable tests should verify:
// 1. State calculations are correct
// 2. Event handlers update state
// 3. Computed values update reactively
```

---

## Quick Start for Build Session

### Prerequisites

```bash
# Node.js 18+ required
node --version

# Create Vue project
npm create vite@latest ui -- --template vue-ts

# Install dependencies
cd ui
npm install element-plus pinia @vueuse/core
npm install -D vitest @vue/test-utils @playwright/test sass
```

### First Steps

1. **Copy types** from `internal/ui/types.go` to `ui/src/types/api.ts`
2. **Create stores** using Pinia pattern
3. **Create WebSocket composable** with exponential backoff
4. **Extract current CSS** to SCSS variables
5. **Build components** with Element Plus, test integration

---


## Files Modified During Implementation

| Phase | File | Change |
|-------|------|--------|
| 11.1 | `ui/package.json` | Created (Vue + Element Plus + Pinia) |
| 11.1 | `ui/tsconfig.json` | Created |
| 11.1 | `ui/vite.config.ts` | Created |
| 11.1 | `ui/src/styles/variables.scss` | Created (extracted CSS) |
| 11.1 | `ui/src/styles/overrides.scss` | Created (Element Plus overrides) |
| 11.2 | `ui/src/types/*.ts` | Created |
| 11.3 | `ui/src/stores/*.ts` | Created |
| 11.4 | `ui/src/components/*.vue` | Created (Element Plus components) |
| 11.4 | `ui/src/composables/*.ts` | Created |
| 11.6 | `internal/ui/embed.go` | Created |
| 11.6 | `internal/ui/webserver.go` | Modified |
| TBD | `internal/ui/index.go` | **DELETE** (after migration) |

---

## Questions Resolved (Feb 2026)

### Q1: CSS Strategy
**Answer:** Extract current CSS + SCSS variables

**Decision:**
- Extract current inline CSS to `styles/variables.scss` and `styles/overrides.scss`
- Add CSS custom properties for theming
- Use Element Plus for components, custom CSS for canvas/dashboard layout

### Q2: Router
**Answer:** No router (single-page application)

**Decision:**
- Single view dashboard
- Modals (Element Plus `el-dialog`) for calibration wizard
- Panels toggle visibility (don't navigate)
- Simpler architecture, no routing complexity

### Q3: Component Library
**Answer:** **Element Plus**

**Decision:**
- Professional enterprise UI components
- Consistent design language
- Includes all needed: dialogs, buttons, tables, inputs, cards
- ~200KB (lighter than Vuetify)
- Good Vue 3 support

### Q4: Canvas Rendering
**Answer:** Native Canvas API + Vue refs

**Decision:**
- Lightweight, no overhead
- Simple operations: rectangles, circles, text
- Vue refs provide reactive binding
- No additional dependencies needed

### Q5: WebSocket Reconnection
**Answer:** Exponential backoff with max attempts

**Decision:**
- Attempts: 5 max
- Base delay: 1000ms
- Formula: `delay = baseDelay * 2^(attempts-1)`
- Prevents server overload on reconnection
- Automatically reconnects on disconnect

### Q6: State Persistence
**Answer:** Server only (no localStorage)

**Decision:**
- Obstacles saved to server: `POST /api/obstacles/save`
- Obstacles loaded from server: `GET /api/obstacles`
- Server writes to YAML files (`config/obstacles.yaml`)
- Single source of truth, consistent across devices
