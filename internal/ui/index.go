//go:build gocv

package ui

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Robot Tracker - Multi-Robot Tracking System</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
            background: #1a1a2e;
            color: #eee;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
        }
        header {
            background: #16213e;
            padding: 12px 24px;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid #0f3460;
        }
        header h1 {
            font-size: 1.4rem;
            font-weight: 500;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .header-right {
            display: flex;
            align-items: center;
            gap: 16px;
        }
        .status { display: flex; gap: 20px; font-size: 0.9rem; }
        .status span { color: #888; }
        .status .value { color: #4ecca3; font-weight: 500; }
        .status .connected .value { color: #4ecca3; }
        .status .disconnected .value { color: #e94560; }
        header h1::before {
            content: '';
            display: inline-block;
            width: 8px;
            height: 8px;
            background: #4ecca3;
            border-radius: 50%;
            animation: pulse 2s infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
        .calibration-badge {
            padding: 6px 12px;
            border-radius: 6px;
            font-size: 0.85rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
            border: 2px solid transparent;
        }
        .calibration-badge.calibrated {
            background: rgba(78, 204, 163, 0.2);
            color: #4ecca3;
            border-color: #4ecca3;
        }
        .calibration-badge.calibrated:hover {
            background: rgba(78, 204, 163, 0.3);
        }
        .calibration-badge.not-calibrated {
            background: rgba(233, 69, 96, 0.2);
            color: #e94560;
            border-color: #e94560;
            animation: pulse-warning 2s infinite;
        }
        .calibration-badge.not-calibrated:hover {
            background: rgba(233, 69, 96, 0.3);
        }
        @keyframes pulse-warning {
            0%, 100% { box-shadow: 0 0 0 0 rgba(233, 69, 96, 0.4); }
            50% { box-shadow: 0 0 0 6px rgba(233, 69, 96, 0); }
        }
        .obstacle-toggle {
            padding: 6px 12px;
            border-radius: 6px;
            font-size: 0.85rem;
            font-weight: 600;
            cursor: pointer;
            transition: all 0.2s;
            border: 2px solid #ff6b6b;
            background: rgba(255, 107, 107, 0.2);
            color: #ff6b6b;
        }
        .obstacle-toggle:hover {
            background: rgba(255, 107, 107, 0.3);
        }
        .obstacle-list {
            max-height: 150px;
            overflow-y: auto;
        }
        .obstacle-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 6px 8px;
            background: rgba(255, 107, 107, 0.1);
            border-radius: 4px;
            margin-bottom: 4px;
        }
        .obstacle-item:hover {
            background: rgba(255, 107, 107, 0.2);
        }
        .obstacle-item button {
            background: transparent;
            border: none;
            color: #ff6b6b;
            cursor: pointer;
            font-size: 1.2rem;
            padding: 0 4px;
        }
        .obstacle-item button:hover {
            color: #ff4444;
        }
        .obstacle-controls {
            display: flex;
            gap: 8px;
            margin-top: 8px;
        }
        .obstacle-controls .btn {
            flex: 1;
            padding: 6px 8px;
            font-size: 0.8rem;
        }
        .main { display: flex; flex: 1; overflow: hidden; }
        .video-container {
            flex: 1;
            display: flex;
            flex-direction: column;
            padding: 16px;
            background: #0a0a0f;
        }
        .video-wrapper {
            flex: 1;
            position: relative;
            background: #000;
            border-radius: 8px;
            overflow: hidden;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        #video {
            max-width: 100%;
            max-height: 100%;
            display: block;
        }
        #overlay {
            position: absolute;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            pointer-events: auto;
        }
        .loading {
            position: absolute;
            color: #666;
            font-size: 1rem;
        }
        .sidebar {
            width: 300px;
            background: #16213e;
            padding: 16px;
            border-left: 1px solid #0f3460;
            display: flex;
            flex-direction: column;
            gap: 16px;
            overflow-y: auto;
        }
        .panel {
            background: #1a1a2e;
            border-radius: 8px;
            padding: 16px;
        }
        .panel h3 {
            font-size: 0.85rem;
            text-transform: uppercase;
            color: #888;
            margin-bottom: 12px;
            letter-spacing: 0.5px;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .panel h3::before {
            content: '';
            display: block;
            width: 3px;
            height: 12px;
            background: #4ecca3;
            border-radius: 2px;
        }
        .controls {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 8px;
            max-width: 180px;
            margin: 0 auto;
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
        .btn:hover { background: #1a4a7a; transform: translateY(-1px); }
        .btn:active { transform: scale(0.95); }
        .btn.wide { grid-column: span 2; }
        .btn.stop { background: #e94560; }
        .btn.stop:hover { background: #ff5a75; }
        .btn.forward { background: #4ecca3; color: #1a1a2e; }
        .btn.forward:hover { background: #5fd9b0; }
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
        .track-list {
            max-height: 250px;
            overflow-y: auto;
        }
        .track-item {
            display: flex;
            align-items: center;
            gap: 12px;
            padding: 10px;
            background: #16213e;
            border-radius: 6px;
            margin-bottom: 8px;
            transition: background 0.2s;
        }
        .track-item:hover { background: #1a2a4e; }
        .track-color {
            width: 36px;
            height: 36px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 600;
            font-size: 0.9rem;
            color: #1a1a2e;
        }
        .track-info { flex: 1; }
        .track-label {
            font-size: 0.9rem;
            font-weight: 500;
            display: flex;
            align-items: center;
            gap: 8px;
        }
        .track-tag {
            font-size: 0.7rem;
            background: #0f3460;
            padding: 2px 6px;
            border-radius: 4px;
            color: #888;
        }
        .track-conf {
            font-size: 0.75rem;
            color: #4ecca3;
            margin-top: 2px;
        }
        .no-tracks {
            color: #555;
            text-align: center;
            padding: 20px;
            font-size: 0.9rem;
        }
        .instructions {
            font-size: 0.85rem;
            color: #aaa;
            line-height: 1.6;
        }
        .instructions li { margin-bottom: 6px; }
        .instructions.calibration-needed {
            border: 1px solid #e94560;
            border-radius: 8px;
            padding: 12px;
            background: rgba(233, 69, 96, 0.1);
        }
        .instructions.calibration-needed li {
            color: #ff8a9b;
        }
        .instructions.calibration-needed .warning {
            color: #e94560;
            font-weight: bold;
            margin-bottom: 8px;
        }
        .hidden { display: none !important; }
        .stats-grid {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 10px;
        }
        .stat-item {
            background: #16213e;
            padding: 12px;
            border-radius: 6px;
            text-align: center;
        }
        .stat-value {
            font-size: 1.4rem;
            font-weight: 600;
            color: #4ecca3;
        }
        .stat-label {
            font-size: 0.75rem;
            color: #666;
            margin-top: 2px;
        }
        .calibration-wizard {
            display: none;
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            height: 100%;
            z-index: 1000;
        }
        .calibration-wizard.active { display: block; }
        .calibration-content {
            position: absolute;
            left: 50%;
            top: 50%;
            transform: translate(-50%, -50%);
            background: #16213e;
            border-radius: 12px;
            padding: 0;
            max-width: 520px;
            width: 90%;
            max-height: 90vh;
            overflow-y: auto;
            box-shadow: 0 10px 40px rgba(0,0,0,0.5);
            transition: transform 0.2s, left 0.1s, top 0.1s;
        }
        .calibration-content.pinned {
            transform: none;
            left: 20px;
            top: 20px;
        }
        .calibration-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            padding: 16px 24px;
            border-bottom: 1px solid #0f3460;
            cursor: move;
            background: #1a1a2e;
            border-radius: 12px 12px 0 0;
        }
        .calibration-header h2 {
            margin: 0;
            color: #4ecca3;
            display: flex;
            align-items: center;
            gap: 10px;
            font-size: 1.2rem;
        }
        .calibration-header h2::before {
            content: '📐';
        }
        .calibration-actions {
            display: flex;
            gap: 8px;
        }
        .calibration-btn-icon {
            width: 32px;
            height: 32px;
            border: none;
            border-radius: 6px;
            background: #0f3460;
            color: #aaa;
            cursor: pointer;
            font-size: 1rem;
            display: flex;
            align-items: center;
            justify-content: center;
            transition: all 0.2s;
        }
        .calibration-btn-icon:hover {
            background: #1a4a7a;
            color: #eee;
        }
        .calibration-btn-icon.pinned {
            background: #4ecca3;
            color: #1a1a2e;
        }
        .calibration-body {
            padding: 20px 24px 24px;
        }
        .calibration-content h2 {
            margin-bottom: 16px;
            color: #4ecca3;
            display: flex;
            align-items: center;
            gap: 10px;
        }
        .calibration-content h2::before {
            content: '📐';
        }
        .calibration-content p {
            color: #aaa;
            margin-bottom: 12px;
            line-height: 1.5;
        }
        .calibration-step {
            display: none;
        }
        .calibration-step.active { display: block; }
        .tag-size-input {
            width: 100%;
            padding: 12px;
            border: 2px solid #0f3460;
            border-radius: 8px;
            background: #1a1a2e;
            color: #eee;
            font-size: 1.1rem;
            text-align: center;
            margin-bottom: 16px;
        }
        .tag-size-input:focus {
            outline: none;
            border-color: #4ecca3;
        }
        .tag-size-hint {
            font-size: 0.8rem;
            color: #666;
            text-align: center;
            margin-top: -8px;
            margin-bottom: 16px;
        }
        .calibration-buttons {
            display: flex;
            gap: 12px;
            margin-top: 20px;
        }
        .calibration-btn {
            flex: 1;
            padding: 12px;
            border: none;
            border-radius: 8px;
            cursor: pointer;
            font-size: 1rem;
            font-weight: 600;
            transition: all 0.2s;
        }
        .calibration-btn.primary {
            background: #4ecca3;
            color: #1a1a2e;
        }
        .calibration-btn.primary:hover {
            background: #5fd9b0;
            transform: translateY(-1px);
        }
        .calibration-btn.primary:disabled {
            background: #2a4a3a;
            color: #666;
            cursor: not-allowed;
            transform: none;
        }
        .calibration-btn.secondary {
            background: #0f3460;
            color: #eee;
        }
        .calibration-btn.secondary:hover {
            background: #1a4a7a;
        }
        .detected-tags-list {
            background: #1a1a2e;
            border-radius: 8px;
            padding: 12px;
            margin-bottom: 16px;
            max-height: 150px;
            overflow-y: auto;
        }
        .detected-tag-item {
            display: flex;
            align-items: center;
            justify-content: space-between;
            padding: 8px 12px;
            background: #16213e;
            border-radius: 6px;
            margin-bottom: 6px;
            cursor: pointer;
            transition: all 0.2s;
            border: 2px solid transparent;
        }
        .detected-tag-item:hover {
            background: #1a2a4e;
        }
        .detected-tag-item.selected {
            border-color: #00bcd4;
            background: rgba(0, 188, 212, 0.1);
        }
        .detected-tag-item .tag-id {
            font-weight: bold;
            color: #4ecca3;
        }
        .detected-tag-item .tag-status {
            font-size: 0.8rem;
            color: #888;
        }
        .calibration-status {
            padding: 12px;
            border-radius: 8px;
            margin-bottom: 16px;
            text-align: center;
            font-weight: 500;
        }
        .calibration-status.success {
            background: rgba(78, 204, 163, 0.2);
            color: #4ecca3;
        }
        .calibration-status.warning {
            background: rgba(255, 193, 7, 0.2);
            color: #ffc107;
        }
        .calibration-status.info {
            background: rgba(0, 188, 212, 0.2);
            color: #00bcd4;
        }
        .calibration-status.error {
            background: rgba(233, 69, 96, 0.2);
            color: #e94560;
        }
        .dimension-results {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 12px;
            margin: 16px 0;
        }
        .dimension-box {
            background: #1a1a2e;
            padding: 16px;
            border-radius: 8px;
            text-align: center;
        }
        .dimension-value {
            font-size: 1.8rem;
            font-weight: bold;
            color: #4ecca3;
        }
        .dimension-label {
            font-size: 0.8rem;
            color: #888;
            margin-top: 4px;
        }
        .measurement-guide {
            background: #1a1a2e;
            border-radius: 8px;
            padding: 16px;
            margin: 16px 0;
        }
        .measurement-guide h4 {
            color: #888;
            font-size: 0.85rem;
            margin-bottom: 12px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .tag-diagram {
            display: flex;
            align-items: center;
            gap: 16px;
            margin-bottom: 12px;
        }
        .tag-preview {
            width: 100px;
            height: 100px;
            position: relative;
            border: 8px solid white;
            background: black;
            flex-shrink: 0;
        }
        .tag-preview::after {
            content: 'DATA';
            position: absolute;
            top: 50%;
            left: 50%;
            transform: translate(-50%, -50%);
            color: white;
            font-size: 10px;
            font-weight: bold;
        }
        .measurement-arrow {
            flex: 1;
            position: relative;
            height: 30px;
        }
        .measurement-arrow::before {
            content: '';
            position: absolute;
            left: 0;
            right: 0;
            top: 50%;
            height: 2px;
            background: #4ecca3;
        }
        .measurement-arrow::after {
            content: '▼';
            position: absolute;
            left: 50%;
            bottom: 0;
            transform: translateX(-50%);
            color: #4ecca3;
            font-size: 12px;
        }
        .measurement-label {
            text-align: center;
            color: #4ecca3;
            font-weight: bold;
            font-size: 0.9rem;
        }
        .measurement-note {
            font-size: 0.8rem;
            color: #888;
            line-height: 1.5;
            margin-top: 8px;
        }
        .toast {
            position: fixed;
            bottom: 20px;
            left: 50%;
            transform: translateX(-50%);
            background: #16213e;
            color: #eee;
            padding: 12px 24px;
            border-radius: 8px;
            z-index: 2000;
            display: none;
            box-shadow: 0 4px 20px rgba(0,0,0,0.4);
            border: 1px solid #0f3460;
        }
        .toast.show { display: block; animation: toastIn 0.3s; }
        @keyframes toastIn {
            from { opacity: 0; transform: translateX(-50%) translateY(20px); }
            to { opacity: 1; transform: translateX(-50%) translateY(0); }
        }
        .tag-size-presets {
            display: flex;
            gap: 8px;
            margin-bottom: 12px;
            flex-wrap: wrap;
        }
        .preset-btn {
            padding: 6px 12px;
            border: 1px solid #0f3460;
            background: #1a1a2e;
            color: #888;
            border-radius: 4px;
            font-size: 0.8rem;
            cursor: pointer;
            transition: all 0.2s;
        }
        .preset-btn:hover {
            border-color: #4ecca3;
            color: #4ecca3;
        }
        .preset-btn.selected {
            background: rgba(78, 204, 163, 0.2);
            border-color: #4ecca3;
            color: #4ecca3;
        }
    </style>
</head>
<body>
    <header>
        <h1>Robot Tracker</h1>
        <div class="header-right">
            <div class="status">
                <span>FPS: <span class="value" id="fps">0</span></span>
                <span>Tracks: <span class="value" id="trackCount">0</span></span>
                <span id="calibrationBadge" class="calibration-badge not-calibrated" onclick="openCalibration()">Not Calibrated</span>
                <button class="obstacle-toggle" onclick="toggleObstaclePanel()" title="Toggle Obstacle Panel">Obstacles</button>
                <span id="arduinoStatus" class="disconnected">Arduino: <span class="value" id="arduino">Disconnected</span></span>
            </div>
        </div>
    </header>

    <div class="calibration-wizard" id="calibrationWizard">
        <div class="calibration-content" id="calibrationContent">
            <div class="calibration-header" id="calibrationHeader">
                <h2>Camera Calibration</h2>
                <div class="calibration-actions">
                    <button class="calibration-btn-icon" id="pinCalibrationBtn" onclick="toggleCalibrationPin()" title="Pin/Unpin dialogue">📌</button>
                    <button class="calibration-btn-icon" onclick="closeCalibration()" title="Close">✕</button>
                </div>
            </div>
            <div class="calibration-body">
            <div class="calibration-step active" id="calibrationStep1">
                <p>Calibration enables <strong>click-to-navigate</strong> and <strong>path planning</strong> by mapping camera pixels to world coordinates.</p>

                <div class="measurement-guide">
                    <h4>How to Measure Your AprilTag</h4>
                    <div class="tag-diagram">
                        <div class="tag-preview"></div>
                        <div class="measurement-arrow"></div>
                    </div>
                    <div class="measurement-label">TAG SIZE (black-white border edge)</div>
                    <p class="measurement-note">
                        Tag size is measured across the outside edge of the inner border which comprises the black pixels for 36h11.
                        The image shows a complete AprilTag with outer white border. From the 36h11 family, its ID code is 42.
                    </p>
                </div>

                <label style="color: #888; font-size: 0.9rem;">Tag Size (meters):</label>

                <div class="tag-size-presets">
                    <button class="preset-btn" onclick="setTagSize(0.10)">10cm</button>
                    <button class="preset-btn selected" onclick="setTagSize(0.15)">15cm</button>
                    <button class="preset-btn" onclick="setTagSize(0.20)">20cm</button>
                </div>

                <input type="number" class="tag-size-input" id="tagSizeInput" value="0.15" step="0.01" min="0.01" max="1.0">

                <div class="calibration-buttons">
                    <button class="calibration-btn secondary" onclick="cancelCalibration()">Cancel</button>
                    <button class="calibration-btn primary" onclick="startCalibrationDetection()">Detect Tags</button>
                </div>
            </div>

            <div class="calibration-step" id="calibrationStep2">
                <div class="calibration-status info" id="calibrationStatus">
                    Detecting AprilTags...
                </div>

                <p id="selectionPrompt" class="hidden">Click a tag in the list or on the video to select:</p>

                <div class="detected-tags-list" id="detectedTagsList" style="display: none;">
                    <!-- Tags will be populated here -->
                </div>

                <div id="noTagsMessage" style="text-align: center; color: #888; padding: 20px;">
                    No tags detected. Make sure the tag is visible in the camera.
                </div>

                <div class="calibration-buttons">
                    <button class="calibration-btn secondary" onclick="cancelCalibration()">Cancel</button>
                    <button class="calibration-btn primary" id="useSelectedBtn" onclick="useSelectedTag()" disabled>Use Selected Tag</button>
                </div>
            </div>

            <div class="calibration-step" id="calibrationStep3">
                <div class="calibration-status success" id="calibrationSuccess">
                    Calibration Complete!
                </div>

                <div class="dimension-results">
                    <div class="dimension-box">
                        <div class="dimension-value" id="resultWidth">--</div>
                        <div class="dimension-label">Width (meters)</div>
                    </div>
                    <div class="dimension-box">
                        <div class="dimension-value" id="resultHeight">--</div>
                        <div class="dimension-label">Height (meters)</div>
                    </div>
                </div>

                <p id="calibrationSavedTo" style="text-align: center; color: #888; font-size: 0.9rem;"></p>

                <div class="calibration-buttons">
                    <button class="calibration-btn primary" onclick="closeCalibration()">Done</button>
                </div>
            </div>
            </div>
        </div>
    </div>

    <div class="toast" id="toast"></div>
    <div class="main">
        <div class="video-container">
            <div class="video-wrapper" id="videoWrapper">
                <img id="video" src="/stream" alt="Video Stream">
                <canvas id="overlay"></canvas>
                <div class="loading" id="loading">Connecting to stream...</div>
            </div>
        </div>
        <div class="sidebar">
            <div class="panel">
                <h3>Manual Control</h3>
                <div class="controls">
                    <button class="btn forward" data-cmd="W" title="Forward">&#9650;</button>
                    <button class="btn" data-cmd="E" title="Weapon Mode">&#9876;</button>
                    <button class="btn" data-cmd="A" title="Rotate Left">&#9664;</button>
                    <button class="btn" data-cmd="S" title="Backward">&#9660;</button>
                    <button class="btn" data-cmd="D" title="Rotate Right">&#9654;</button>
                    <button class="btn wide stop" data-cmd="X" title="Emergency Stop">&#9632; STOP ALL</button>
                </div>
                <div class="keyboard-hint">
                    <kbd>W</kbd><kbd>A</kbd><kbd>S</kbd><kbd>D</kbd> Move &nbsp;|&nbsp; <kbd>E</kbd> Weapon &nbsp;|&nbsp; <kbd>X</kbd> Stop
                </div>
            </div>
            <div class="panel">
                <h3>Statistics</h3>
                <div class="stats-grid">
                    <div class="stat-item">
                        <div class="stat-value" id="statFps">0</div>
                        <div class="stat-label">FPS</div>
                    </div>
                    <div class="stat-item">
                        <div class="stat-value" id="statTracks">0</div>
                        <div class="stat-label">Active Tracks</div>
                    </div>
                </div>
            </div>
            <div class="panel">
                <h3>Detected Targets</h3>
                <div class="track-list" id="trackList">
                    <div class="no-tracks">Waiting for detections...</div>
                </div>
            </div>
            <div class="panel" id="obstaclePanel" style="display: none;">
                <h3>Static Obstacles</h3>
                <div class="obstacle-list" id="obstacleList">
                    <div class="no-tracks">No obstacles defined</div>
                </div>
                <div class="obstacle-controls">
                    <button id="drawObstacleBtn" class="btn btn-secondary" onclick="toggleDrawMode()">Draw Obstacle</button>
                    <button id="clearObstaclesBtn" class="btn btn-danger" onclick="clearObstacles()">Clear All</button>
                    <button id="saveObstaclesBtn" class="btn btn-primary" onclick="saveObstacles()">Save</button>
                </div>
            </div>
            <div class="panel" id="instructionsPanel">
                <h3>Instructions</h3>
                <div class="instructions calibration-needed" id="instructionsContent">
                    <p class="warning">Camera not calibrated - navigation disabled</p>
                    <p>Click the <strong>"Not Calibrated"</strong> badge above to run calibration.</p>
                    <ul>
                        <li>Click-to-navigate on video</li>
                        <li>Path planning around obstacles</li>
                        <li>World coordinate tracking</li>
                    </ul>
                </div>
            </div>
        </div>
    </div>

    <script>
        var COLORS = ['#4ecca3', '#e94560', '#ffc107', '#00bcd4', '#9c27b0', '#ff5722'];
        var trackColors = {};

        function getTrackColor(trackId) {
            if (!trackColors.hasOwnProperty(trackId)) {
                var keys = Object.keys(trackColors);
                trackColors[trackId] = COLORS[keys.length % COLORS.length];
            }
            return trackColors[trackId];
        }

        var video = document.getElementById('video');
        var overlay = document.getElementById('overlay');
        var ctx = overlay.getContext('2d');
        var trackList = document.getElementById('trackList');
        var loading = document.getElementById('loading');
        var ws = new WebSocket('ws://' + window.location.host + '/ws');

        var tracks = [];
        var destinations = [];
        var lastFrameTime = Date.now();
        var frameCount = 0;
        var fps = 0;
        var calibrationMode = false;
        var detectedTags = [];
        var selectedTagId = null;
        var obstacles = [];
        var drawMode = false;
        var drawStart = null;
        var currentDraw = null;
        var lastDrawnRect = null;
        var isMouseDown = false;
        var isDrawing = false;

        ws.onopen = function() {
            loading.style.display = 'none';
            console.log('WebSocket connected');
        };

        ws.onclose = function() {
            loading.style.display = 'block';
            loading.textContent = 'Connection lost. Reconnecting...';
            setTimeout(function() { window.location.reload(); }, 3000);
        };

        ws.onmessage = function(event) {
            try {
                var data = JSON.parse(event.data);
                handleMessage(data);
            } catch (e) {
                console.error('Failed to parse WebSocket message:', e);
            }
        };

        function handleMessage(data) {
            switch(data.type) {
                case 'track':
                    if (data.track) {
                        updateTrack(data.track);
                    }
                    break;
                case 'tracks':
                    tracks = data.tracks || [];
                    updateTrackList();
                    break;
                case 'bbox':
                    drawBoundingBox(data.bbox);
                    break;
                case 'status':
                    updateStatus(data.status);
                    break;
                case 'detection':
                    if (data.detections) {
                        tracks = data.detections;
                        updateTrackList();
                    }
                    break;
                case 'calibration_tags':
                    handleCalibrationTags(data.tags);
                    break;
                case 'calibration':
                    if (data.calibration) {
                        var calib = data.calibration;
                        if (calib.state === 'complete' || calib.state === 'calibrated') {
                            updateCalibrationBadge(true);
                        } else if (calib.state === 'not_calibrated') {
                            updateCalibrationBadge(false);
                        }
                    }
                    break;
                case 'obstacles':
                    if (data.obstacles) {
                        obstacles = data.obstacles.obstacles || [];
                        redrawOverlay();
                    }
                    break;
            }
        }

        function updateTrack(trackData) {
            var existing = null;
            for (var i = 0; i < tracks.length; i++) {
                if (tracks[i].id === trackData.id) {
                    existing = tracks[i];
                    break;
                }
            }
            if (existing) {
                for (var key in trackData) {
                    existing[key] = trackData[key];
                }
            } else {
                tracks.push(trackData);
            }
            updateTrackList();
        }

        function updateStatus(status) {
            if (status.fps !== undefined) {
                fps = status.fps;
                document.getElementById('fps').textContent = fps.toFixed(1);
                document.getElementById('statFps').textContent = fps.toFixed(0);
            }
            if (status.robotCount !== undefined) {
                document.getElementById('trackCount').textContent = status.robotCount;
                document.getElementById('statTracks').textContent = status.robotCount;
            }
            if (status.arduinoState !== undefined) {
                var el = document.getElementById('arduino');
                el.textContent = status.arduinoState;
                var statusEl = document.getElementById('arduinoStatus');
                if (status.arduinoState === 'Connected') {
                    statusEl.classList.remove('disconnected');
                    statusEl.classList.add('connected');
                } else {
                    statusEl.classList.remove('connected');
                    statusEl.classList.add('disconnected');
                }
            }
        }

        function updateTrackList() {
            if (tracks.length === 0) {
                trackList.innerHTML = '<div class="no-tracks">Waiting for detections...</div>';
                return;
            }

            var html = '';
            for (var i = 0; i < tracks.length; i++) {
                var track = tracks[i];
                var color = getTrackColor(track.id);
                var tagInfo = track.tag_id !== undefined
                    ? '<span class="track-tag">Tag #' + track.tag_id + '</span>'
                    : '';
                html += '<div class="track-item">';
                html += '<div class="track-color" style="background: ' + color + '; color: #1a1a2e;">' + track.id + '</div>';
                html += '<div class="track-info">';
                html += '<div class="track-label">Track #' + track.id + tagInfo + '</div>';
                html += '<div class="track-conf">Confidence: ' + (track.confidence * 100).toFixed(1) + '%</div>';
                html += '</div></div>';
            }
            trackList.innerHTML = html;
        }

        function drawBoundingBox(bbox) {
            if (!bbox) return;

            var color = bbox.color || getTrackColor(bbox.id || 0);

            ctx.strokeStyle = color;
            ctx.lineWidth = 2;
            ctx.strokeRect(bbox.x1, bbox.y1, bbox.x2 - bbox.x1, bbox.y2 - bbox.y1);

            if (bbox.id && selectedTagId === bbox.id && calibrationMode) {
                ctx.strokeStyle = '#00bcd4';
                ctx.lineWidth = 4;
                ctx.strokeRect(bbox.x1 - 4, bbox.y1 - 4, bbox.x2 - bbox.x1 + 8, bbox.y2 - bbox.y1 + 8);
            }

            ctx.fillStyle = color;
            ctx.fillRect(bbox.x1 - 1, bbox.y1 - 22, 60, 18);

            ctx.fillStyle = '#000';
            ctx.font = 'bold 11px sans-serif';
            var label = bbox.label || ('ID:' + (bbox.id || '?'));
            ctx.fillText(label, bbox.x1 + 4, bbox.y1 - 8);
        }

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

        function loadObstacles() {
            fetch('/api/obstacles')
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    obstacles = data.obstacles || [];
                    redrawOverlay();
                })
                .catch(function(err) {
                    console.error('Failed to load obstacles:', err);
                });
        }

        function addObstacle(topLeft, bottomRight) {
            var name = 'obstacle_' + (obstacles.length + 1);
            fetch('/api/obstacles', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    pixel_top_left: topLeft,
                    pixel_bottom_right: bottomRight,
                    name: name,
                    clearance: 0.02
                })
            })
            .then(function(r) { return r.json(); })
            .then(function(data) {
                loadObstacles();
            })
            .catch(function(err) {
                console.error('Failed to add obstacle:', err);
            });
        }

        function deleteObstacle(id) {
            fetch('/api/obstacles/' + encodeURIComponent(id), { method: 'DELETE' })
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    loadObstacles();
                })
                .catch(function(err) {
                    console.error('Failed to delete obstacle:', err);
                });
        }

        function clearObstacles() {
            fetch('/api/obstacles/clear', { method: 'POST' })
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    loadObstacles();
                })
                .catch(function(err) {
                    console.error('Failed to clear obstacles:', err);
                });
        }

        function saveObstacles() {
            fetch('/api/obstacles/save', { method: 'POST' })
                .then(function(r) { return r.json(); })
                .then(function(data) {
                    showToast(data.message || 'Obstacles saved');
                    loadObstacles();
                })
                .catch(function(err) {
                    console.error('Failed to save obstacles:', err);
                });
        }

        function toggleObstaclePanel() {
            var panel = document.getElementById('obstaclePanel');
            if (panel.style.display === 'none') {
                panel.style.display = 'block';
                loadObstacles();
            } else {
                panel.style.display = 'none';
            }
        }

        function toggleDrawMode() {
            drawMode = !drawMode;
            drawStart = null;
            currentDraw = null;
            lastDrawnRect = null;
            redrawOverlay();
            var btn = document.getElementById('drawObstacleBtn');
            if (btn) {
                btn.textContent = drawMode ? 'Cancel Drawing' : 'Draw Obstacle';
                btn.classList.toggle('btn-danger', drawMode);
            }
            showToast(drawMode ? 'Click and drag on video to draw obstacle' : 'Drawing cancelled');
        }

        overlay.addEventListener('mousedown', function(e) {
            if (!drawMode) return;
            e.preventDefault();

            var rect = overlay.getBoundingClientRect();
            var scaleX = overlay.width / rect.width;
            var scaleY = overlay.height / rect.height;
            drawStart = [(e.clientX - rect.left) * scaleX, (e.clientY - rect.top) * scaleY];
            currentDraw = {
                x1: drawStart[0], y1: drawStart[1],
                x2: drawStart[0], y2: drawStart[1]
            };
            isDrawing = true;
            redrawOverlay();
        });

        overlay.addEventListener('mousemove', function(e) {
            if (!drawMode || !drawStart) return;
            var rect = overlay.getBoundingClientRect();
            var scaleX = overlay.width / rect.width;
            var scaleY = overlay.height / rect.height;
            currentDraw.x2 = (e.clientX - rect.left) * scaleX;
            currentDraw.y2 = (e.clientY - rect.top) * scaleY;
            redrawOverlay();
        });

        overlay.addEventListener('mouseup', function(e) {
            if (!drawMode || !drawStart) return;
            var rect = overlay.getBoundingClientRect();
            var scaleX = overlay.width / rect.width;
            var scaleY = overlay.height / rect.height;
            var x2 = (e.clientX - rect.left) * scaleX;
            var y2 = (e.clientY - rect.top) * scaleY;

            var topLeft = [
                Math.round(Math.min(drawStart[0], x2)),
                Math.round(Math.min(drawStart[1], y2))
            ];
            var bottomRight = [
                Math.round(Math.max(drawStart[0], x2)),
                Math.round(Math.max(drawStart[1], y2))
            ];

            if (bottomRight[0] - topLeft[0] > 10 && bottomRight[1] - topLeft[1] > 10) {
                addObstacle(topLeft, bottomRight);
            }

            drawStart = null;
            currentDraw = null;
            lastDrawnRect = null;
            isMouseDown = false;
            isDrawing = false;
            redrawOverlay();
        });

        video.onload = syncCanvasSize;
        video.onloadeddata = syncCanvasSize;
        window.addEventListener('resize', syncCanvasSize);
        setInterval(syncCanvasSize, 1000);

        window.addEventListener('mousedown', function(e) {
            isMouseDown = true;
        });
        window.addEventListener('mouseup', function(e) {
            isMouseDown = false;
        });

        var buttons = document.querySelectorAll('.btn');
        for (var i = 0; i < buttons.length; i++) {
            var btn = buttons[i];
            btn.addEventListener('click', (function(cmd) {
                return function() { sendCommand(cmd); };
            })(btn.dataset.cmd));
            btn.addEventListener('mousedown', function() { this.style.transform = 'scale(0.95)'; });
            btn.addEventListener('mouseup', function() { this.style.transform = ''; });
            btn.addEventListener('mouseleave', function() { this.style.transform = ''; });
        }

        var keyMap = {
            'w': 'W', 'W': 'W',
            'a': 'A', 'A': 'A',
            's': 'S', 'S': 'S',
            'd': 'D', 'D': 'D',
            'e': 'E', 'E': 'E',
            'x': 'X', 'X': 'X'
        };

        document.addEventListener('keydown', function(e) {
            if (e.target.tagName === 'INPUT' || e.target.tagName === 'TEXTAREA') return;

            if (e.key.toLowerCase() === 'z' && drawMode) {
                e.preventDefault();
                toggleDrawMode();
                return;
            }

            if (e.key.toLowerCase() === 'c' && obstacles.length > 0) {
                e.preventDefault();
                if (confirm('Clear all obstacles?')) {
                    clearObstacles();
                }
                return;
            }

            if (keyMap[e.key]) {
                e.preventDefault();
                sendCommand(keyMap[e.key]);
            }
        });

        function sendCommand(cmd) {
            fetch('/api/command', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ command: cmd })
            }).catch(function(err) { console.error('Failed to send command:', err); });
        }

        overlay.addEventListener('click', function(e) {
            if (drawMode) return;

            var rect = overlay.getBoundingClientRect();
            var x = Math.round((e.clientX - rect.left) * (overlay.width / rect.width));
            var y = Math.round((e.clientY - rect.top) * (overlay.height / rect.height));

            if (calibrationMode && detectedTags.length > 0) {
                var closestTag = findClosestTag(x, y);
                if (closestTag !== null) {
                    selectTag(closestTag);
                    return;
                }
            }

            destinations.push({ x: x, y: y, time: Date.now() });

            ctx.strokeStyle = '#4ecca3';
            ctx.lineWidth = 2;
            ctx.beginPath();
            ctx.arc(x, y, 12, 0, Math.PI * 2);
            ctx.stroke();
            ctx.fillStyle = '#4ecca3';
            ctx.beginPath();
            ctx.arc(x, y, 5, 0, Math.PI * 2);
            ctx.fill();

            fetch('/api/destination', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ x: x, y: y })
            }).catch(function(err) { console.error('Failed to send destination:', err); });
        });

        function updateFPS() {
            frameCount++;
            var now = Date.now();
            if (now - lastFrameTime >= 1000) {
                var currentFps = frameCount;
                frameCount = 0;
                lastFrameTime = now;
                if (currentFps > 0 && fps === 0) {
                    document.getElementById('fps').textContent = currentFps.toFixed(1);
                }
            }
            requestAnimationFrame(updateFPS);
        }
        updateFPS();

        video.addEventListener('load', function() {
            loading.style.display = 'none';
        });

        console.log('Robot Tracker UI initialized');

        function openCalibration() {
            calibrationMode = true;
            selectedTagId = null;
            detectedTags = [];
            document.getElementById('calibrationWizard').classList.add('active');
            showCalibrationStep(1);
        }

        function closeCalibration() {
            calibrationMode = false;
            selectedTagId = null;
            document.getElementById('calibrationWizard').classList.remove('active');
        }

        function toggleCalibrationPin() {
            isCalibrationPinned = !isCalibrationPinned;
            var btn = document.getElementById('pinCalibrationBtn');
            var content = document.getElementById('calibrationContent');
            if (isCalibrationPinned) {
                btn.classList.add('pinned');
                content.classList.add('pinned');
            } else {
                btn.classList.remove('pinned');
                content.classList.remove('pinned');
            }
        }

        var dragOffset = { x: 0, y: 0 };
        var isDraggingCalibration = false;

        document.getElementById('calibrationHeader').addEventListener('mousedown', function(e) {
            if (e.target.tagName === 'BUTTON') return;
            isDraggingCalibration = true;
            var content = document.getElementById('calibrationContent');
            var rect = content.getBoundingClientRect();
            dragOffset.x = e.clientX - rect.left;
            dragOffset.y = e.clientY - rect.top;
        });

        document.addEventListener('mousemove', function(e) {
            if (isDraggingCalibration) {
                var content = document.getElementById('calibrationContent');
                content.style.transform = 'none';
                content.style.left = (e.clientX - dragOffset.x) + 'px';
                content.style.top = (e.clientY - dragOffset.y) + 'px';
            }
        });

        document.addEventListener('mouseup', function() {
            isDraggingCalibration = false;
        });

        function cancelCalibration() {
            calibrationMode = false;
            selectedTagId = null;
            fetch('/api/calibration/cancel', { method: 'POST' })
                .then(function() { closeCalibration(); });
        }

        function showCalibrationStep(step) {
            document.getElementById('calibrationStep1').classList.toggle('active', step === 1);
            document.getElementById('calibrationStep2').classList.toggle('active', step === 2);
            document.getElementById('calibrationStep3').classList.toggle('active', step === 3);
        }

        function setTagSize(size) {
            document.getElementById('tagSizeInput').value = size;
            var presetBtns = document.querySelectorAll('.preset-btn');
            presetBtns.forEach(function(btn) {
                btn.classList.toggle('selected', btn.textContent.includes(size.toString().replace('0.', '')));
            });
        }

        function startCalibrationDetection() {
            var tagSize = parseFloat(document.getElementById('tagSizeInput').value) || 0.15;

            fetch('/api/calibration/start', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ tagSize: tagSize })
            })
            .then(function(response) { return response.json(); })
            .then(function(data) {
                if (data.status === 'ok') {
                    showCalibrationStep(2);
                    updateCalibrationStatus('info', 'Detecting AprilTags...');
                    document.getElementById('detectedTagsList').style.display = 'none';
                    document.getElementById('noTagsMessage').style.display = 'none';
                    document.getElementById('selectionPrompt').classList.add('hidden');
                    document.getElementById('useSelectedBtn').disabled = true;

                    setTimeout(pollForDetectedTags, 500);
                }
            });
        }

        function pollForDetectedTags() {
            fetch('/api/calibration/detected-tags')
                .then(function(response) { return response.json(); })
                .then(function(data) {
                    if (data.tags && data.tags.length > 0) {
                        handleCalibrationTags(data.tags);
                    } else {
                        setTimeout(pollForDetectedTags, 500);
                    }
                })
                .catch(function() {
                    setTimeout(pollForDetectedTags, 500);
                });
        }

        function handleCalibrationTags(tags) {
            if (!calibrationMode) return;

            detectedTags = tags;

            if (tags.length === 0) {
                document.getElementById('detectedTagsList').style.display = 'none';
                document.getElementById('noTagsMessage').style.display = 'block';
                document.getElementById('selectionPrompt').classList.add('hidden');
                updateCalibrationStatus('info', 'Looking for AprilTags...');
            } else {
                document.getElementById('detectedTagsList').style.display = 'block';
                document.getElementById('noTagsMessage').style.display = 'none';
                document.getElementById('selectionPrompt').classList.remove('hidden');

                var html = '';
                tags.forEach(function(tag, idx) {
                    var isSelected = tag.id === selectedTagId;
                    html += '<div class="detected-tag-item' + (isSelected ? ' selected' : '') + '" onclick="selectTag(' + tag.id + ')" data-id="' + tag.id + '">';
                    html += '<span class="tag-id">Tag #' + tag.id + '</span>';
                    html += '<span class="tag-status">' + (isSelected ? '✓ Selected' : 'Click to select') + '</span>';
                    html += '</div>';
                });
                document.getElementById('detectedTagsList').innerHTML = html;

                if (selectedTagId !== null) {
                    document.getElementById('useSelectedBtn').disabled = false;
                    updateCalibrationStatus('success', 'Tag #' + selectedTagId + ' selected');
                } else {
                    updateCalibrationStatus('info', tags.length + ' tag(s) detected - select one to use');
                }
            }
        }

        function selectTag(tagId) {
            selectedTagId = tagId;

            var items = document.querySelectorAll('.detected-tag-item');
            items.forEach(function(item) {
                var id = parseInt(item.getAttribute('data-id'));
                item.classList.toggle('selected', id === tagId);
                item.querySelector('.tag-status').textContent = id === tagId ? '✓ Selected' : 'Click to select';
            });

            document.getElementById('useSelectedBtn').disabled = false;
            updateCalibrationStatus('success', 'Tag #' + tagId + ' selected - click "Use Selected Tag"');

            var tag = detectedTags.find(function(t) { return t.id === tagId; });
            if (tag && tag.corners) {
                redrawOverlay();
            }
        }

        function findClosestTag(x, y) {
            if (detectedTags.length === 0) return null;

            var closest = null;
            var minDist = Infinity;

            detectedTags.forEach(function(tag) {
                if (tag.center) {
                    var dx = tag.center[0] - x;
                    var dy = tag.center[1] - y;
                    var dist = Math.sqrt(dx * dx + dy * dy);
                    if (dist < minDist) {
                        minDist = dist;
                        closest = tag.id;
                    }
                }
            });

            return closest;
        }

        function drawObstacleRect(rect) {
            var w = rect.x2 - rect.x1;
            var h = rect.y2 - rect.y1;
            ctx.strokeStyle = '#ffa500';
            ctx.lineWidth = 2;
            ctx.setLineDash([5, 5]);
            ctx.strokeRect(rect.x1, rect.y1, w, h);
            ctx.setLineDash([]);

            ctx.fillStyle = 'rgba(255, 165, 0, 0.2)';
            ctx.fillRect(rect.x1, rect.y1, w, h);
        }

        function redrawOverlay() {
            ctx.clearRect(0, 0, overlay.width, overlay.height);

            obstacles.forEach(function(obs) {
                if (obs.pixel_top_left && obs.pixel_bottom_right) {
                    var x1 = obs.pixel_top_left[0];
                    var y1 = obs.pixel_top_left[1];
                    var x2 = obs.pixel_bottom_right[0];
                    var y2 = obs.pixel_bottom_right[1];
                    var width = x2 - x1;
                    var height = y2 - y1;

                    ctx.strokeStyle = '#ff6b6b';
                    ctx.lineWidth = 2;
                    ctx.setLineDash([5, 5]);
                    ctx.strokeRect(x1, y1, width, height);
                    ctx.setLineDash([]);

                    ctx.fillStyle = '#ff6b6b';
                    ctx.font = 'bold 11px sans-serif';
                    ctx.fillText('OBSTACLE', x1, y1 - 5);

                    ctx.fillStyle = 'rgba(255, 107, 107, 0.1)';
                    ctx.fillRect(x1, y1, width, height);
                }
            });

            if (currentDraw) {
                drawObstacleRect(currentDraw);
            }

            detectedTags.forEach(function(tag) {
                if (tag.corners) {
                    var isSelected = tag.id === selectedTagId;
                    var corners = tag.corners;

                    ctx.strokeStyle = isSelected ? '#00bcd4' : '#4ecca3';
                    ctx.lineWidth = isSelected ? 4 : 2;
                    ctx.beginPath();
                    ctx.moveTo(corners[0][0], corners[0][1]);
                    for (var i = 1; i < corners.length; i++) {
                        ctx.lineTo(corners[i][0], corners[i][1]);
                    }
                    ctx.closePath();
                    ctx.stroke();

                    if (isSelected) {
                        ctx.strokeStyle = 'rgba(0, 188, 212, 0.3)';
                        ctx.lineWidth = 8;
                        ctx.stroke();
                    }

                    ctx.fillStyle = isSelected ? '#00bcd4' : '#4ecca3';
                    ctx.font = 'bold 14px sans-serif';
                    ctx.fillText('Tag #' + tag.id, corners[0][0], corners[0][1] - 10);
                }
            });
        }

        function useSelectedTag() {
            if (selectedTagId === null) {
                showToast('Please select a tag first');
                return;
            }

            var tag = detectedTags.find(function(t) { return t.id === selectedTagId; });
            if (!tag) {
                showToast('Tag not found');
                return;
            }

            var tagSize = parseFloat(document.getElementById('tagSizeInput').value) || 0.15;

            fetch('/api/calibration/compute', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    tagId: selectedTagId,
                    tagSize: tagSize,
                    corners: tag.corners || [[100,100],[200,100],[200,200],[100,200]]
                })
            })
            .then(function(response) { return response.json(); })
            .then(function(data) {
                if (data.state === 'complete') {
                    document.getElementById('resultWidth').textContent = data.computedWidth.toFixed(2);
                    document.getElementById('resultHeight').textContent = data.computedHeight.toFixed(2);

                    var filename = data.filename || 'config/calibration_*.yaml';
                    document.getElementById('calibrationSavedTo').textContent = 'Saved to: ' + filename;

                    updateCalibrationBadge(true);
                    showCalibrationStep(3);
                    calibrationMode = false;
                } else {
                    updateCalibrationStatus('error', data.error || 'Calibration failed');
                }
            })
            .catch(function(err) {
                console.error('Calibration compute error:', err);
                showToast('Calibration failed');
            });
        }

        function updateCalibrationStatus(type, message) {
            var statusEl = document.getElementById('calibrationStatus');
            statusEl.className = 'calibration-status ' + type;
            statusEl.textContent = message;
        }

        function updateCalibrationBadge(calibrated) {
            var badge = document.getElementById('calibrationBadge');
            var instructionsContent = document.getElementById('instructionsContent');
            var instructionsPanel = document.getElementById('instructionsPanel');

            if (calibrated) {
                badge.textContent = 'Calibrated';
                badge.classList.remove('not-calibrated');
                badge.classList.add('calibrated');
                instructionsContent.innerHTML = '<ul><li>Click on video to set navigation target</li><li>Use WASD keys for movement control</li><li>Press <kbd>E</kbd> for weapon mode</li><li>Press <kbd>X</kbd> for emergency stop</li><li>Green boxes = tracked robots</li><li>Red boxes = detected obstacles</li></ul>';
            } else {
                badge.textContent = 'Not Calibrated';
                badge.classList.remove('calibrated');
                badge.classList.add('not-calibrated');
            }
        }

        function showToast(message) {
            var toast = document.getElementById('toast');
            toast.textContent = message;
            toast.classList.add('show');
            setTimeout(function() {
                toast.classList.remove('show');
            }, 3000);
        }

        checkCalibrationStatus();  // Check immediately on load
        setInterval(checkCalibrationStatus, 5000);
        function checkCalibrationStatus() {
            fetch('/api/calibration/status')
                .then(function(response) { return response.json(); })
                .then(function(data) {
                    if (data.state === 'complete' || data.state === 'calibrated') {
                        updateCalibrationBadge(true);
                    } else if (data.state === 'not_calibrated') {
                        updateCalibrationBadge(false);
                    }
                })
                .catch(function() {});
        }
    </script>
</body>
</html>`
