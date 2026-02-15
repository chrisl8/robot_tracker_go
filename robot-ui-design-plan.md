# Robot Control Interface - Design Plan
## Sleek Exploration Theme (The Expanse-Inspired)

**Created:** February 14, 2026  
**Project:** Robot Control Web UI Redesign  
**Theme:** Hard Sci-Fi / Space Exploration Aesthetic

---

## Overview

This document outlines the complete design plan for replacing the existing robot control web UI with a sleek, exploration-themed interface inspired by The Expanse, NASA operations, and modern spacecraft controls. The interface prioritizes technical readouts with high data density while maintaining a clean, unobstructed view of the central 1920x1080 video feed.

### Primary Functions
- Visual robot selection via clicking on video feed
- Point-and-click destination setting on video
- Direct remote control fallback via side panel controls
- Real-time telemetry monitoring
- Multi-robot coordination support

---

## Color Palette

### Primary Colors
```css
--bg-deep-space: #0a0e14;
--bg-slate: #141920;
--panel-dark: #1a2332;

--accent-cyan: #00d9ff;
--accent-blue: #0091ea;
--accent-teal: #00bfa5;

--text-primary: #e0e6ed;
--text-secondary: #a8b5c4;
--text-dim: #6b7785;

--warning-amber: #ffab00;
--alert-red: #ff3d00;
--success-green: #00e676;
```

### Usage Guidelines
- **Backgrounds:** Deep space black for main canvas, dark slate for panels
- **Accents:** Cyan for primary interactive elements, teal for active states
- **Status:** Green (operational), Amber (caution), Red (critical/offline)
- **Text:** Cool white primary, desaturated secondary for less critical info

---

## Layout Architecture

### Screen Resolution: 1920x1080+
```
┌──────────────────────────────────────────────────────────────┐
│  [LEFT PANEL - 280px]  │  VIDEO FEED 1920x1080  │  [RIGHT - 280px] │
│                        │                        │                  │
│  ┌─ ROBOT SELECT ───┐  │   ┌──────────────┐    │  ┌─ TELEMETRY ─┐ │
│  │ • Robot A  [87%] │  │   │ Click robot  │    │  │ VEL  12.4cm/s│ │
│  │ • Robot B  [92%] │  │   │ to select    │    │  │ HDG  247° NW │ │
│  │ • Robot C  [45%] │  │   │              │    │  │ BAT  87% ███ │ │
│  └──────────────────┘  │   │ Click ground │    │  │ SIG  -42dBm  │ │
│                        │   │ to set dest  │    │  │ TEMP 42°C    │ │
│  ┌─ STATUS ─────────┐  │   └──────────────┘    │  └──────────────┘ │
│  │ • System Health  │  │                        │                  │
│  │ • Network Stats  │  │   [Robot markers]      │  ┌─ DIRECT CTL ┐ │
│  │ • Active Tasks   │  │   [Path vectors]       │  │   ↑          │ │
│  └──────────────────┘  │   [Dest markers]       │  │  ← ● →       │ │
│                        │                        │  │   ↓          │ │
│  ┌─ SYSTEM LOG ────┐  │                        │  │  [STOP]      │ │
│  │ > Connected A    │  │                        │  └──────────────┘ │
│  │ > Dest set: B    │  │                        │                  │
│  │ > Path complete  │  │                        │  ┌─ MOTORS ────┐ │
│  └──────────────────┘  │                        │  │ M1: 87% ███  │ │
│                        │                        │  │ M2: 91% ███  │ │
└──────────────────────────────────────────────────────────────────┘
│        [BOTTOM BAR - 60px]                                       │
│   Mission: 00:14:32  |  Conn: 4/4  |  Alerts: 0  |  Uptime: 2d  │
└──────────────────────────────────────────────────────────────────┘
```

### Panel Specifications
- **Left Panel:** 280px fixed width
  - Robot selection cards
  - System status indicators
  - Activity/event log
  - Mission parameters
  
- **Center Video:** 1920x1080 (or responsive to maintain aspect ratio)
  - Primary robot camera feed
  - Interactive overlay for selection
  - Path visualization
  - Robot/destination markers
  
- **Right Panel:** 280px fixed width
  - Real-time telemetry data
  - Direct control interface
  - Motor/system health
  - Communication status
  
- **Bottom Bar:** 60px fixed height
  - Mission timer
  - Connection status
  - Alert summary
  - System uptime

---

## Component Specifications

### 1. Robot Selection Cards (Left Panel)

**Visual Design:**
- Dark panel background (#1a2332)
- 1px cyan border with subtle glow
- Hexagonal thumbnail icon for each robot
- Status ring around thumbnail (color-coded)

**Information Displayed:**
```
┌────────────────────────┐
│  [HEX]  ROBOT-A        │
│   ◯     Battery: 87%   │
│         Signal: ████▌  │
│         Status: ACTIVE │
│         Pos: 12.4, 8.7 │
└────────────────────────┘
```

**States:**
- **Idle:** Dim cyan border
- **Selected:** Bright cyan glow, expanded
- **Active/Moving:** Pulsing teal accent
- **Low Battery:** Amber border
- **Offline:** Red border, desaturated

**Interactions:**
- Click to select (multi-select with Shift/Ctrl)
- Hover shows expanded telemetry
- Double-click to center camera on robot

---

### 2. Video Overlay Elements

**Robot Markers:**
```css
/* Floating circular marker */
- Outer ring: 40px diameter, cyan stroke
- Inner dot: 8px diameter, solid cyan
- Label: Robot ID above marker
- Connection line to actual position (if offset)
```

**Destination Markers:**
```css
/* Pulsing waypoint pin */
- Pin shape: 30px tall
- Pulse animation: 1.5s ease-in-out infinite
- Color: Bright blue (#00d9ff)
- Drop shadow for depth
```

**Path Visualization:**
```css
/* Animated path line */
- Dotted line: 2px stroke
- Color: Cyan with 70% opacity
- Animation: Dash offset moving forward
- Arrow at destination end
```

**Selection Highlight:**
```css
/* Expanding ring on click */
- Initial: 0px
- Final: 60px diameter
- Duration: 400ms
- Opacity: 1.0 → 0.0
- Color: Cyan
```

**Overlay Panels (for text over video):**
```css
/* Semi-transparent info boxes */
- Background: rgba(26, 35, 50, 0.85)
- Backdrop filter: blur(8px)
- Border: 1px solid rgba(0, 217, 255, 0.3)
- Padding: 12px
- Border radius: 4px
```

---

### 3. Telemetry Readouts (Right Panel)

**Layout Pattern:**
```
LABEL    VALUE    VISUALIZATION
──────────────────────────────
VEL      12.4cm/s  ▂▄▆█▆▄▂
HDG      247° NW   [compass]
BAT      87%       ████▌░░░░
SIG      -42dBm    ████▌░░░░
TEMP     42°C      ▂▃▄▅▆▅▄
CPU      23%       ███░░░░░░
MEM      1.2GB     ████▌░░░░
```

**Typography:**
- Label: 10px uppercase, letter-spacing 1px
- Value: 16px monospace (JetBrains Mono)
- Unit: 12px, dimmed text

**Color Coding:**
- Green zone: Normal operation
- Amber zone: Approaching limits
- Red zone: Critical/dangerous
- Blue: Neutral/informational

**Update Frequency:**
- Display update rate: 10Hz (100ms)
- Visual transition: 150ms fade
- No jarring jumps in values

---

### 4. Direct Control Interface

**Circular D-Pad Design:**
```
       ┌───┐
    ┌──┤ ↑ ├──┐
    │  └───┘  │
┌───┤         ├───┐
│ ← │    ●    │ → │
└───┤         ├───┘
    │  ┌───┐  │
    └──┤ ↓ ├──┘
       └───┘
```

**Specifications:**
- Total diameter: 140px
- Center emergency stop: 40px diameter (red when armed)
- Directional segments: 30° arc each
- Speed ring: Outer 20px band
- Transparent with cyan borders
- Fill color on press with haptic-style pulse

**Interaction:**
- Click/tap for discrete movement
- Click-hold for continuous movement
- Outer ring drag for speed adjustment
- Center double-click for emergency stop

---

### 5. System Status Panel

**Components:**
```
┌─ SYSTEM STATUS ──────────┐
│ ● Network    ████▌  89%  │
│ ● Compute    ███░░  62%  │
│ ● Storage    █████  91%  │
│ ● Power      ████░  78%  │
│                          │
│ Active Robots: 3/4       │
│ Tasks Running: 7         │
│ Uptime: 2d 14h 32m       │
└──────────────────────────┘
```

**Visual Elements:**
- Status dots: Green (good), Amber (warning), Red (critical)
- Progress bars with percentage
- Monospaced alignment for numbers
- Auto-scrolling if content overflows

---

### 6. Activity Log

**Format:**
```
┌─ ACTIVITY LOG ───────────┐
│ 14:32:18 > Robot A: OK   │
│ 14:31:45 > Dest set: B   │
│ 14:30:12 > Path complete │
│ 14:29:03 > Robot C: LOW  │
│ 14:28:30 > Conn restored │
└──────────────────────────┘
```

**Features:**
- Timestamp in HH:MM:SS format
- Color-coded by message type
- Auto-scroll to bottom (newest first)
- Click to expand details
- Filter buttons for log levels
- Max 100 messages, then rotate

---

## Typography System

### Font Families
```css
--font-heading: 'Rajdhani', 'Orbitron', sans-serif;
--font-body: 'Inter', 'Source Sans Pro', sans-serif;
--font-mono: 'JetBrains Mono', 'IBM Plex Mono', monospace;
```

### Font Scale
```css
--text-xs: 10px;   /* Labels, metadata */
--text-sm: 12px;   /* Secondary info */
--text-base: 14px; /* Body text */
--text-lg: 16px;   /* Data values */
--text-xl: 20px;   /* Section headers */
--text-2xl: 24px;  /* Page title */
```

### Font Weights
- Regular: 400 (body text)
- Medium: 500 (emphasis)
- Semibold: 600 (headings)
- Bold: 700 (critical alerts)

---

## Animation & Transitions

### Timing Functions
```css
--ease-smooth: cubic-bezier(0.4, 0.0, 0.2, 1);
--ease-snap: cubic-bezier(0.0, 0.0, 0.2, 1);
--ease-bounce: cubic-bezier(0.68, -0.55, 0.265, 1.55);
```

### Duration Guidelines
- Micro interactions: 100-150ms (hover, focus)
- Standard transitions: 200-300ms (panels, overlays)
- Data updates: 150ms (value changes)
- Loading states: 1000ms (pulse cycle)
- Page transitions: 400ms (route changes)

### Animation Principles
1. **Smooth, not jarring** - No sudden jumps in position or values
2. **Purposeful** - Every animation communicates state change
3. **Performance first** - Use transform/opacity, avoid layout thrashing
4. **Respect motion preferences** - `prefers-reduced-motion` support

### Key Animations

**Panel Slide-In:**
```css
@keyframes slideInLeft {
  from {
    transform: translateX(-100%);
    opacity: 0;
  }
  to {
    transform: translateX(0);
    opacity: 1;
  }
}
```

**Glow Pulse (Active State):**
```css
@keyframes glowPulse {
  0%, 100% {
    box-shadow: 0 0 4px rgba(0, 217, 255, 0.5);
  }
  50% {
    box-shadow: 0 0 12px rgba(0, 217, 255, 0.9);
  }
}
```

**Data Stream (Loading):**
```css
@keyframes dataStream {
  0% {
    background-position: 0% 0%;
  }
  100% {
    background-position: 100% 100%;
  }
}
```

**Selection Ring Expand:**
```css
@keyframes ringExpand {
  0% {
    transform: scale(0);
    opacity: 1;
  }
  100% {
    transform: scale(1.5);
    opacity: 0;
  }
}
```

---

## Visual Effects

### Glow Effects (Subtle, Non-Obscuring)
```css
/* Active panel border */
box-shadow: 0 0 4px rgba(0, 217, 255, 0.4);

/* Hover state */
box-shadow: 0 0 8px rgba(0, 217, 255, 0.6);

/* Critical alert */
box-shadow: 0 0 12px rgba(255, 61, 0, 0.8);
```

### Backdrop Blur (Floating Panels Over Video)
```css
background: rgba(26, 35, 50, 0.85);
backdrop-filter: blur(8px);
-webkit-backdrop-filter: blur(8px);
```

### Border Treatments
```css
/* Standard panel */
border: 1px solid rgba(0, 217, 255, 0.3);

/* Active/selected */
border: 1px solid rgba(0, 217, 255, 0.8);

/* Disabled */
border: 1px solid rgba(168, 181, 196, 0.2);
```

### Gradient Accents (Sparingly)
```css
/* Header bars, emphasis areas */
background: linear-gradient(
  90deg,
  rgba(0, 145, 234, 0.1) 0%,
  rgba(0, 217, 255, 0.05) 100%
);
```

---

## Interactive States

### Button States
```css
/* Default */
background: rgba(0, 217, 255, 0.1);
border: 1px solid rgba(0, 217, 255, 0.4);
color: #00d9ff;

/* Hover */
background: rgba(0, 217, 255, 0.2);
border: 1px solid rgba(0, 217, 255, 0.8);
box-shadow: 0 0 8px rgba(0, 217, 255, 0.4);

/* Active/Pressed */
background: rgba(0, 217, 255, 0.3);
transform: scale(0.98);

/* Disabled */
background: rgba(107, 119, 133, 0.1);
border: 1px solid rgba(107, 119, 133, 0.3);
color: #6b7785;
opacity: 0.5;
cursor: not-allowed;
```

### Selection States
- **Selected:** Cyan glow + filled background
- **Multi-select:** Multiple cyan glows
- **Hover:** Teal accent appears
- **Focus:** Keyboard navigation ring (accessibility)

---

## Icon System

### Icon Library
Recommended: **Lucide Icons** or **Material Symbols** (outlined variant)

### Icon Sizes
```css
--icon-xs: 12px;  /* Inline indicators */
--icon-sm: 16px;  /* Buttons, labels */
--icon-md: 20px;  /* Standard UI */
--icon-lg: 24px;  /* Headers */
--icon-xl: 32px;  /* Feature icons */
```

### Icon Categories Needed
- **Navigation:** Arrow keys, waypoint, home, target
- **Status:** Check, warning, error, info, offline
- **Control:** Play, pause, stop, power, refresh
- **Robot:** Bot icon, battery, signal, GPS, motors
- **System:** Settings, logs, network, CPU, storage
- **Media:** Camera, video, screenshot, record

### SVG Styling
```css
.icon {
  stroke: currentColor;
  fill: none;
  stroke-width: 2px;
  stroke-linecap: round;
  stroke-linejoin: round;
}
```

---

## Responsive Behavior

### Breakpoints
```css
--screen-sm: 1366px;  /* Small desktop */
--screen-md: 1920px;  /* Standard desktop */
--screen-lg: 2560px;  /* Large desktop */
--screen-xl: 3840px;  /* 4K displays */
```

### Layout Adaptations

**< 1920px (Laptop screens):**
- Reduce video feed to maintain aspect ratio
- Stack some telemetry panels
- Reduce panel widths to 240px
- Slightly smaller fonts

**1920px - 2560px (Standard):**
- Full layout as designed
- All panels visible
- Optimal spacing

**> 2560px (Large displays):**
- Scale up proportionally
- Consider adding additional panels
- Larger fonts for readability at distance
- More data density possible

---

## Data Visualization

### Progress Bars
```css
/* Container */
width: 100%;
height: 8px;
background: rgba(107, 119, 133, 0.2);
border-radius: 4px;

/* Fill */
background: linear-gradient(90deg, #00bfa5, #00d9ff);
height: 100%;
transition: width 150ms ease;
```

### Sparkline Graphs (Inline)
- Canvas-based or SVG
- Height: 20-30px
- Width: 60-80px
- Stroke width: 1.5px
- Color: Cyan (#00d9ff)
- Show last 20-30 data points
- Auto-scale Y-axis

### Status Indicators
```css
/* Dot indicator */
width: 8px;
height: 8px;
border-radius: 50%;

/* Colors */
.status-good { background: #00e676; }
.status-warn { background: #ffab00; }
.status-crit { background: #ff3d00; }
.status-offline { background: #6b7785; }

/* Pulse animation for active */
animation: pulse 2s ease-in-out infinite;
```

### Compass/Heading Display
- Circular design, 80px diameter
- Cardinal directions marked
- Current heading: Bright pointer
- Rotation animation on heading change
- 360° gradations

---

## Accessibility Considerations

### Keyboard Navigation
- Tab through all interactive elements
- Arrow keys for directional controls
- Escape to cancel/deselect
- Space/Enter to activate
- Visible focus indicators (cyan outline)

### Screen Reader Support
- Semantic HTML elements
- ARIA labels for custom controls
- Live regions for dynamic updates
- Alt text for all icons
- Status announcements

### Color Contrast
- All text meets WCAG AA standards (4.5:1)
- Critical alerts meet AAA standards (7:1)
- Test with color blindness simulators
- Don't rely on color alone for information

### Motion Preferences
```css
@media (prefers-reduced-motion: reduce) {
  * {
    animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
  }
}
```

---

## Performance Optimization

### Video Feed
- Use native `<video>` element with hardware acceleration
- WebRTC for low-latency streaming
- Fallback to HLS/DASH for compatibility
- Canvas overlay for markers (separate layer)

### Animation Performance
- Use `transform` and `opacity` only (GPU accelerated)
- Avoid `width`, `height`, `top`, `left` animations
- `will-change` for frequently animated elements
- RequestAnimationFrame for smooth updates

### Data Updates
- WebSocket for real-time telemetry
- Throttle update rate to display refresh (60fps max)
- Batch DOM updates
- Virtual scrolling for long logs

### Bundle Optimization
- Code splitting by route/panel
- Lazy load non-critical components
- Tree-shake unused libraries
- Minify and compress assets
- Use CDN for common libraries

---

## Technology Stack Recommendations

### Frontend Framework
**Recommended:** React or Vue 3
- Component-based architecture
- Reactive data binding
- Strong ecosystem
- Good performance

**Alternative:** Svelte (lighter weight, faster)

### State Management
- **React:** Zustand or Redux Toolkit
- **Vue:** Pinia or Vuex
- **Vanilla:** Custom event system

### Canvas/Graphics
- **Markers/Overlays:** Fabric.js or Konva.js
- **Data Viz:** Chart.js or D3.js (for sparklines)
- **3D (if needed):** Three.js

### Communication
- **WebSocket:** Socket.io or native WebSocket API
- **Video:** WebRTC (MediaStream API)
- **HTTP:** Axios or Fetch API

### Styling
- **CSS-in-JS:** Styled Components or Emotion
- **Utility:** Tailwind CSS (with custom theme)
- **Preprocessor:** SCSS/Sass (if needed)

### Build Tools
- **Bundler:** Vite (fast, modern)
- **Alternative:** Webpack 5
- **Task Runner:** npm scripts

---

## File Structure

```
/robot-control-ui/
├── public/
│   ├── index.html
│   ├── favicon.ico
│   └── assets/
│       ├── icons/
│       └── fonts/
├── src/
│   ├── components/
│   │   ├── LeftPanel/
│   │   │   ├── RobotSelector.jsx
│   │   │   ├── SystemStatus.jsx
│   │   │   └── ActivityLog.jsx
│   │   ├── VideoFeed/
│   │   │   ├── VideoPlayer.jsx
│   │   │   ├── RobotMarker.jsx
│   │   │   ├── DestinationMarker.jsx
│   │   │   └── PathOverlay.jsx
│   │   ├── RightPanel/
│   │   │   ├── Telemetry.jsx
│   │   │   ├── DirectControl.jsx
│   │   │   └── MotorStatus.jsx
│   │   ├── BottomBar/
│   │   │   └── StatusBar.jsx
│   │   └── common/
│   │       ├── Button.jsx
│   │       ├── ProgressBar.jsx
│   │       ├── StatusDot.jsx
│   │       └── Sparkline.jsx
│   ├── hooks/
│   │   ├── useWebSocket.js
│   │   ├── useRobotSelection.js
│   │   └── useTelemetry.js
│   ├── services/
│   │   ├── robotAPI.js
│   │   ├── videoStream.js
│   │   └── telemetryService.js
│   ├── styles/
│   │   ├── theme.css
│   │   ├── animations.css
│   │   └── global.css
│   ├── utils/
│   │   ├── formatters.js
│   │   └── validators.js
│   ├── App.jsx
│   └── main.jsx
├── package.json
├── vite.config.js
└── README.md
```

---

## Implementation Phases

### Phase 1: Foundation (Week 1)
- [ ] Set up project structure and build system
- [ ] Implement color system and CSS variables
- [ ] Create base layout (panels + video area)
- [ ] Set up WebSocket connection
- [ ] Basic video feed integration

### Phase 2: Core Components (Week 2)
- [ ] Robot selection cards
- [ ] Telemetry display panels
- [ ] Direct control interface
- [ ] Bottom status bar
- [ ] Activity log

### Phase 3: Video Interactions (Week 3)
- [ ] Robot markers on video
- [ ] Destination markers
- [ ] Click-to-select functionality
- [ ] Path visualization
- [ ] Selection animations

### Phase 4: Polish & Effects (Week 4)
- [ ] Glow effects and animations
- [ ] State transitions
- [ ] Loading states
- [ ] Error handling UI
- [ ] Responsive adjustments

### Phase 5: Testing & Optimization (Week 5)
- [ ] Performance profiling
- [ ] Cross-browser testing
- [ ] Accessibility audit
- [ ] User testing
- [ ] Bug fixes and refinements

---

## Code Examples & Snippets

### CSS Theme Variables
```css
:root {
  /* Colors */
  --bg-deep-space: #0a0e14;
  --bg-slate: #141920;
  --panel-dark: #1a2332;
  --accent-cyan: #00d9ff;
  --accent-blue: #0091ea;
  --accent-teal: #00bfa5;
  --text-primary: #e0e6ed;
  --text-secondary: #a8b5c4;
  --text-dim: #6b7785;
  --warning-amber: #ffab00;
  --alert-red: #ff3d00;
  --success-green: #00e676;
  
  /* Typography */
  --font-heading: 'Rajdhani', sans-serif;
  --font-body: 'Inter', sans-serif;
  --font-mono: 'JetBrains Mono', monospace;
  
  /* Spacing */
  --space-xs: 4px;
  --space-sm: 8px;
  --space-md: 16px;
  --space-lg: 24px;
  --space-xl: 32px;
  
  /* Borders */
  --border-radius: 4px;
  --border-width: 1px;
  
  /* Shadows */
  --shadow-glow: 0 0 8px rgba(0, 217, 255, 0.4);
  --shadow-panel: 0 4px 12px rgba(0, 0, 0, 0.3);
  
  /* Timing */
  --duration-fast: 150ms;
  --duration-normal: 300ms;
  --ease-smooth: cubic-bezier(0.4, 0.0, 0.2, 1);
}
```

### Panel Component Template
```jsx
// Panel.jsx
const Panel = ({ title, children, className }) => {
  return (
    <div className={`panel ${className}`}>
      <div className="panel-header">
        <h3>{title}</h3>
      </div>
      <div className="panel-content">
        {children}
      </div>
    </div>
  );
};

// panel.css
.panel {
  background: var(--panel-dark);
  border: var(--border-width) solid rgba(0, 217, 255, 0.3);
  border-radius: var(--border-radius);
  overflow: hidden;
}

.panel-header {
  padding: var(--space-md);
  border-bottom: var(--border-width) solid rgba(0, 217, 255, 0.2);
  background: linear-gradient(
    90deg,
    rgba(0, 145, 234, 0.1) 0%,
    transparent 100%
  );
}

.panel-header h3 {
  font-family: var(--font-heading);
  font-size: 14px;
  font-weight: 600;
  letter-spacing: 1px;
  text-transform: uppercase;
  color: var(--accent-cyan);
  margin: 0;
}

.panel-content {
  padding: var(--space-md);
}
```

### WebSocket Hook Example
```javascript
// useWebSocket.js
import { useEffect, useRef, useState } from 'react';

export const useWebSocket = (url) => {
  const [data, setData] = useState(null);
  const [status, setStatus] = useState('disconnected');
  const ws = useRef(null);

  useEffect(() => {
    ws.current = new WebSocket(url);

    ws.current.onopen = () => {
      setStatus('connected');
      console.log('WebSocket connected');
    };

    ws.current.onmessage = (event) => {
      const parsedData = JSON.parse(event.data);
      setData(parsedData);
    };

    ws.current.onerror = (error) => {
      console.error('WebSocket error:', error);
      setStatus('error');
    };

    ws.current.onclose = () => {
      setStatus('disconnected');
      console.log('WebSocket disconnected');
    };

    return () => {
      ws.current?.close();
    };
  }, [url]);

  const send = (message) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify(message));
    }
  };

  return { data, status, send };
};
```

---

## Design References

### Visual Inspiration Sources
1. **The Expanse** - Rocinante bridge, Tycho Station operations
2. **SpaceX** - Dragon 2 touchscreen interface, mission control
3. **NASA** - ISS control panels, mission control displays
4. **Elite Dangerous** - Ship cockpit HUD and interfaces
5. **Prometheus** - Engineer technology, holographic displays
6. **Westworld** - Tablet interfaces, lab control systems

### UI/UX Patterns
- F-pattern layout for information hierarchy
- Progressive disclosure for complex controls
- Confirmation dialogs for destructive actions
- Toast notifications for system events
- Contextual help tooltips on hover

---

## Testing Checklist

### Functional Testing
- [ ] Robot selection (single and multi)
- [ ] Destination setting via click
- [ ] Direct control responsiveness
- [ ] Telemetry data accuracy
- [ ] Video feed stability
- [ ] WebSocket reconnection
- [ ] Error state handling

### Visual Testing
- [ ] Color contrast ratios
- [ ] Animation smoothness
- [ ] Responsive layouts
- [ ] Cross-browser rendering
- [ ] Dark theme consistency
- [ ] Icon clarity at all sizes

### Performance Testing
- [ ] Initial load time < 2s
- [ ] Time to interactive < 3s
- [ ] Smooth 60fps animations
- [ ] Memory usage stable over time
- [ ] Network efficiency (minimal data)
- [ ] Video latency < 200ms

### Accessibility Testing
- [ ] Keyboard navigation complete
- [ ] Screen reader compatibility
- [ ] Focus indicators visible
- [ ] Color blindness simulation
- [ ] Motion reduction respected
- [ ] Text scalability

---

## Future Enhancements

### Potential Additions
1. **Multi-camera support** - Picture-in-picture for additional robot cameras
2. **3D visualization** - Top-down map view with robot positions
3. **Historical playback** - Replay recorded missions
4. **Collaborative mode** - Multiple operators viewing/controlling
5. **Voice commands** - Natural language control integration
6. **AR overlay** - Augmented reality for on-site operators
7. **Advanced telemetry** - Predictive analytics, anomaly detection
8. **Custom dashboards** - User-configurable panel layouts
9. **Mobile companion app** - Monitoring on tablets/phones
10. **Mission planning** - Pre-program complex waypoint sequences

---

## Maintenance Notes

### Regular Updates Needed
- Update dependencies monthly (security patches)
- Monitor WebSocket library for updates
- Review browser compatibility quarterly
- Refresh design tokens as needed
- Performance profiling after major features

### Documentation
- Keep component API docs current
- Document WebSocket message formats
- Maintain changelog for releases
- Update README with setup instructions

---

## Contact & Support

For questions or clarifications on this design plan, consult the original conversation or reach out to the design team.

**Design System Version:** 1.0  
**Last Updated:** February 14, 2026  
**Status:** Ready for implementation

---

*End of Design Plan*

