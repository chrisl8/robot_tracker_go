package ui

const indexHTML = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Robot Tracker</title>
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
        h1 { font-size: 1.4rem; font-weight: 500; }
        .status { display: flex; gap: 20px; font-size: 0.9rem; }
        .status span { color: #888; }
        .status .value { color: #4ecca3; font-weight: 500; }
        .status .connected .value { color: #4ecca3; }
        .main { display: flex; flex: 1; }
        .video-container {
            flex: 1;
            display: flex;
            flex-direction: column;
            padding: 16px;
        }
        .video-wrapper {
            flex: 1;
            position: relative;
            background: #0a0a0f;
            border-radius: 8px;
            overflow: hidden;
            min-height: 400px;
        }
        #video { width: 100%; height: 100%; object-fit: contain; }
        #overlay {
            position: absolute;
            top: 0; left: 0;
            width: 100%; height: 100%;
            pointer-events: none;
        }
        .sidebar {
            width: 280px;
            background: #16213e;
            padding: 16px;
            border-left: 1px solid #0f3460;
            display: flex;
            flex-direction: column;
            gap: 16px;
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
        }
        .controls {
            display: grid;
            grid-template-columns: repeat(3, 1fr);
            gap: 8px;
            max-width: 160px;
            margin: 0 auto;
        }
        .btn {
            padding: 12px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 1.2rem;
            transition: all 0.15s ease;
            background: #0f3460;
            color: #eee;
        }
        .btn:hover { background: #1a4a7a; }
        .btn:active { transform: scale(0.95); }
        .btn.wide { grid-column: span 2; }
        .btn.stop { background: #e94560; }
        .btn.stop:hover { background: #ff5a75; }
        .keyboard-hint {
            font-size: 0.75rem;
            color: #666;
            text-align: center;
            margin-top: 8px;
        }
        .keyboard-hint kbd {
            background: #0f3460;
            padding: 2px 6px;
            border-radius: 4px;
            margin: 0 2px;
        }
        .track-list {
            max-height: 200px;
            overflow-y: auto;
        }
        .track-item {
            display: flex;
            align-items: center;
            gap: 10px;
            padding: 8px;
            background: #16213e;
            border-radius: 6px;
            margin-bottom: 6px;
        }
        .track-id {
            width: 32px; height: 32px;
            border-radius: 50%;
            display: flex;
            align-items: center;
            justify-content: center;
            font-weight: 600;
            font-size: 0.85rem;
        }
        .track-info { flex: 1; }
        .track-label { font-size: 0.85rem; }
        .track-conf { font-size: 0.75rem; color: #888; }
        .destination {
            position: absolute;
            width: 16px; height: 16px;
            border: 2px solid #4ecca3;
            border-radius: 50%;
            transform: translate(-50%, -50%);
            pointer-events: none;
        }
        .destination::after {
            content: '';
            position: absolute;
            top: 50%; left: 50%;
            width: 6px; height: 6px;
            background: #4ecca3;
            border-radius: 50%;
            transform: translate(-50%, -50%);
        }
        .path-line {
            position: absolute;
            pointer-events: none;
        }
    </style>
</head>
<body>
    <header>
        <h1>Robot Tracker</h1>
        <div class="status">
            <span>FPS: <span class="value" id="fps">0</span></span>
            <span>Robots: <span class="value" id="robotCount">0</span></span>
            <span class="connected">Arduino: <span class="value" id="arduino">Disconnected</span></span>
        </div>
    </header>
    <div class="main">
        <div class="video-container">
            <div class="video-wrapper">
                <img id="video" src="/stream" alt="Video Stream">
                <canvas id="overlay"></canvas>
            </div>
        </div>
        <div class="sidebar">
            <div class="panel">
                <h3>Manual Control</h3>
                <div class="controls">
                    <button class="btn" data-cmd="W" title="Forward">▲</button>
                    <button class="btn" data-cmd="E" title="Weapon">⚔</button>
                    <button class="btn" data-cmd="A" title="Left">◀</button>
                    <button class="btn" data-cmd="S" title="Stop">●</button>
                    <button class="btn" data-cmd="D" title="Right">▶</button>
                    <button class="btn wide stop" data-cmd="X" title="Emergency Stop">STOP ALL</button>
                </div>
                <div class="keyboard-hint">
                    <kbd>W</kbd><kbd>A</kbd><kbd>S</kbd><kbd>D</kbd> Move | <kbd>E</kbd> Weapon | <kbd>X</kbd> Stop
                </div>
            </div>
            <div class="panel">
                <h3>Detected Tracks</h3>
                <div class="track-list" id="trackList">
                    <div style="color: #666; text-align: center; padding: 20px;">No tracks detected</div>
                </div>
            </div>
            <div class="panel">
                <h3>Instructions</h3>
                <p style="font-size: 0.85rem; color: #aaa; line-height: 1.5;">
                    • Click on video to set destination<br>
                    • Use controls or keyboard to move<br>
                    • Green box = tracked robot<br>
                    • Red box = detected obstacle
                </p>
            </div>
        </div>
    </div>
    <script>
        const video = document.getElementById('video');
        const overlay = document.getElementById('overlay');
        const ctx = overlay.getContext('2d');
        const trackList = document.getElementById('trackList');
        const ws = new WebSocket('ws://' + window.location.host + '/ws');

        let tracks = [];
        let destinations = [];
        let paths = [];

        ws.onmessage = function(event) {
            const data = JSON.parse(event.data);
            switch(data.type) {
                case 'track':
                    tracks = data.tracks || [];
                    break;
                case 'detection':
                    break;
                case 'status':
                    document.getElementById('fps').textContent = data.fps.toFixed(1);
                    document.getElementById('robotCount').textContent = data.robotCount;
                    document.getElementById('arduino').textContent = data.connected ? 'Connected' : 'Disconnected';
                    break;
            }
        };

        video.onload = function() {
            overlay.width = video.clientWidth;
            overlay.height = video.clientHeight;
        };

        function drawOverlay() {
            ctx.clearRect(0, 0, overlay.width, overlay.height);

            tracks.forEach(track => {
                if (track.bbox) {
                    ctx.strokeStyle = track.color || '#4ecca3';
                    ctx.lineWidth = 2;
                    ctx.strokeRect(track.bbox[0], track.bbox[1], track.bbox[2] - track.bbox[0], track.bbox[3] - track.bbox[1]);

                    ctx.fillStyle = track.color || '#4ecca3';
                    ctx.fillRect(track.bbox[0] - 1, track.bbox[1] - 20, 50, 18);
                    ctx.fillStyle = '#000';
                    ctx.font = '12px sans-serif';
                    ctx.fillText('ID:' + track.id, track.bbox[0] + 4, track.bbox[1] - 7);

                    if (track.history && track.history.length > 1) {
                        ctx.strokeStyle = track.color || '#4ecca3';
                        ctx.lineWidth = 1;
                        ctx.globalAlpha = 0.5;
                        ctx.beginPath();
                        track.history.forEach((p, i) => {
                            if (i === 0) ctx.moveTo(p[0], p[1]);
                            else ctx.lineTo(p[0], p[1]);
                        });
                        ctx.stroke();
                        ctx.globalAlpha = 1;
                    }
                }
            });

            destinations.forEach(dest => {
                ctx.strokeStyle = '#4ecca3';
                ctx.lineWidth = 2;
                ctx.beginPath();
                ctx.arc(dest.x, dest.y, 10, 0, Math.PI * 2);
                ctx.stroke();
                ctx.fillStyle = '#4ecca3';
                ctx.beginPath();
                ctx.arc(dest.x, dest.y, 4, 0, Math.PI * 2);
                ctx.fill();
            });

            requestAnimationFrame(drawOverlay);
        }
        drawOverlay();

        document.querySelectorAll('.btn').forEach(btn => {
            btn.addEventListener('click', () => sendCommand(btn.dataset.cmd));
        });

        document.addEventListener('keydown', (e) => {
            const keyMap = { 'w': 'W', 'a': 'A', 's': 'S', 'd': 'D', 'e': 'E', 'x': 'X', ' ': ' ' };
            if (keyMap[e.key.toLowerCase()]) {
                e.preventDefault();
                sendCommand(keyMap[e.key.toLowerCase()]);
            }
        });

        function sendCommand(cmd) {
            fetch('/api/command', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ command: cmd })
            });
        }

        overlay.addEventListener('click', (e) => {
            const rect = overlay.getBoundingClientRect();
            const x = e.clientX - rect.left;
            const y = e.clientY - rect.top;
            destinations.push({ x, y });
            fetch('/api/destination', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ x, y })
            });
        });

        window.addEventListener('resize', () => {
            overlay.width = video.clientWidth;
            overlay.height = video.clientHeight;
        });
    </script>
</body>
</html>`
