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
        .status { display: flex; gap: 20px; font-size: 0.9rem; }
        .status span { color: #888; }
        .status .value { color: #4ecca3; font-weight: 500; }
        .status .connected .value { color: #4ecca3; }
        .status .disconnected .value { color: #e94560; }
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
            pointer-events: none;
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
        .instructions li {
            margin-bottom: 6px;
        }
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
    </style>
</head>
<body>
    <header>
        <h1>Robot Tracker</h1>
        <div class="status">
            <span>FPS: <span class="value" id="fps">0</span></span>
            <span>Tracks: <span class="value" id="trackCount">0</span></span>
            <span id="arduinoStatus" class="disconnected">Arduino: <span class="value" id="arduino">Disconnected</span></span>
        </div>
    </header>
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
            <div class="panel">
                <h3>Instructions</h3>
                <ul class="instructions">
                    <li>Click on video to set navigation target</li>
                    <li>Use WASD keys for movement control</li>
                    <li>Press <kbd>E</kbd> for weapon mode</li>
                    <li>Press <kbd>X</kbd> for emergency stop</li>
                    <li>Green boxes = tracked robots</li>
                    <li>Red boxes = detected obstacles</li>
                </ul>
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
                trackList.innerHTML = '<div class=\"no-tracks\">Waiting for detections...</div>';
                return;
            }

            var html = '';
            for (var i = 0; i < tracks.length; i++) {
                var track = tracks[i];
                var color = getTrackColor(track.id);
                var tagInfo = track.tag_id !== undefined
                    ? '<span class=\"track-tag\">Tag #' + track.tag_id + '</span>'
                    : '';
                html += '<div class=\"track-item\">';
                html += '<div class=\"track-color\" style=\"background: ' + color + '; color: #1a1a2e;\">' + track.id + '</div>';
                html += '<div class=\"track-info\">';
                html += '<div class=\"track-label\">Track #' + track.id + tagInfo + '</div>';
                html += '<div class=\"track-conf\">Confidence: ' + (track.confidence * 100).toFixed(1) + '%</div>';
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
                overlay.width = rect.width;
                overlay.height = rect.height;
            }
        }

        video.onload = syncCanvasSize;
        video.onloadeddata = syncCanvasSize;

        window.addEventListener('resize', syncCanvasSize);

        setInterval(syncCanvasSize, 1000);

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
            var rect = overlay.getBoundingClientRect();
            var x = Math.round((e.clientX - rect.left) * (overlay.width / rect.width));
            var y = Math.round((e.clientY - rect.top) * (overlay.height / rect.height));

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
    </script>
</body>
</html>`
