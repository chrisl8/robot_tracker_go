//go:build gocv

package ui

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/chrisl8/robot_tracker_go/internal/position"
)

func postJSON(t *testing.T, s *WebServer, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.router.engine.ServeHTTP(w, req)
	return w
}

func serverWithFrame(t *testing.T, w, h int) *WebServer {
	t.Helper()
	s := NewWebServer(":0")
	est, err := position.NewPositionEstimator("")
	if err != nil {
		t.Fatal(err)
	}
	est.SetFrameSize(w, h)
	s.SetPositionEstimator(est)
	return s
}

func TestHandleCommand_RejectsUnknownCommand(t *testing.T) {
	s := NewWebServer(":0")
	called := false
	s.Callbacks.OnCommand = func(string) error { called = true; return nil }

	if w := postJSON(t, s, "POST", "/api/command", `{"command":"X"}`); w.Code != http.StatusBadRequest {
		t.Errorf("unknown command: status %d, want 400", w.Code)
	}
	if called {
		t.Error("an invalid command must never reach the controller callback")
	}
	if w := postJSON(t, s, "POST", "/api/command", `{"command":"F"}`); w.Code != http.StatusOK || !called {
		t.Errorf("valid command: status %d called=%v, want 200 and called", w.Code, called)
	}
}

func TestHandleCommand_CallbackErrorIsConflict(t *testing.T) {
	s := NewWebServer(":0")
	s.Callbacks.OnCommand = func(string) error { return errors.New("emergency stop is active") }

	if w := postJSON(t, s, "POST", "/api/command", `{"command":"F"}`); w.Code != http.StatusConflict {
		t.Errorf("status %d, want 409", w.Code)
	}
}

func TestHandleDestination_Validation(t *testing.T) {
	s := serverWithFrame(t, 640, 480)
	var sets int
	s.Callbacks.OnDestinationSet = func(int, [2]float64) error { sets++; return nil }

	bad := []string{
		`{"robot_id":-1,"x":10,"y":10}`,
		`{"robot_id":1,"x":-5,"y":10}`,
		`{"robot_id":1,"x":10,"y":-5}`,
		`{"robot_id":1,"x":640,"y":10}`,
		`{"robot_id":1,"x":10,"y":480}`,
		`{"robot_id":1,"x":99999,"y":99999}`,
	}
	for _, body := range bad {
		if w := postJSON(t, s, "POST", "/api/destination", body); w.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", body, w.Code)
		}
	}
	if sets != 0 {
		t.Errorf("OnDestinationSet called %d times for invalid requests, want 0", sets)
	}
	if s.destination.current.Valid {
		t.Error("an invalid destination must not be stored")
	}

	if w := postJSON(t, s, "POST", "/api/destination", `{"robot_id":1,"x":639,"y":479}`); w.Code != http.StatusOK || sets != 1 {
		t.Errorf("valid destination: status %d sets=%d, want 200 and 1", w.Code, sets)
	}
}

// If the app refuses the destination (e.g. not calibrated) the UI must not be
// told there is one: it used to be stored and broadcast anyway, so the operator
// saw a goal the planner never received.
func TestHandleDestination_RefusedByAppIsNotStoredOrBroadcast(t *testing.T) {
	s := serverWithFrame(t, 640, 480)
	s.Callbacks.OnDestinationSet = func(int, [2]float64) error { return errors.New("not calibrated") }

	httpServer := httptest.NewServer(s.router.engine)
	defer httpServer.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	waitForClients(t, s, 1)

	w := postJSON(t, s, "POST", "/api/destination", `{"robot_id":1,"x":100,"y":100}`)

	if w.Code != http.StatusConflict {
		t.Errorf("status %d, want 409", w.Code)
	}
	if s.destination.current.Valid {
		t.Error("a refused destination must not be stored")
	}
	_ = conn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break // timeout: nothing more was broadcast
		}
		if strings.Contains(string(data), `"destination"`) {
			t.Fatalf("a refused destination was broadcast to clients: %s", data)
		}
	}
}

func waitForClients(t *testing.T, s *WebServer, n int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		s.hub.clientMutex.RLock()
		got := len(s.hub.clients)
		s.hub.clientMutex.RUnlock()
		if got == n {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d websocket clients", n)
}

// A mode or e-stop change made from one browser must reach every other open
// browser, or they keep showing (and sending) stale controls.
func TestControlStateChangesAreBroadcast(t *testing.T) {
	s := NewWebServer(":0")
	mode, estopped := "hold", false
	s.Callbacks.OnGetControlState = func() (string, bool) { return mode, estopped }
	s.Callbacks.OnModeChange = func(m string) error { mode = m; return nil }
	s.Callbacks.OnEmergencyStop = func() { mode, estopped = "hold", true }
	s.Callbacks.OnClearEmergencyStop = func() error { estopped = false; return nil }

	httpServer := httptest.NewServer(s.router.engine)
	defer httpServer.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	waitForClients(t, s, 1)

	expectControl := func(wantMode string, wantStopped bool) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				t.Fatalf("no control_state broadcast for mode=%s stopped=%v: %v", wantMode, wantStopped, err)
			}
			var msg struct {
				Type    string `json:"type"`
				Control *struct {
					Mode             string `json:"mode"`
					EmergencyStopped bool   `json:"emergency_stopped"`
				} `json:"control"`
			}
			if json.Unmarshal(data, &msg) != nil || msg.Type != "control_state" || msg.Control == nil {
				continue
			}
			if msg.Control.Mode != wantMode || msg.Control.EmergencyStopped != wantStopped {
				t.Fatalf("control_state = %+v, want mode=%s stopped=%v", *msg.Control, wantMode, wantStopped)
			}
			return
		}
	}

	postJSON(t, s, "POST", "/api/mode", `{"mode":"manual"}`)
	expectControl("manual", false)

	postJSON(t, s, "POST", "/api/emergency-stop", ``)
	expectControl("hold", true)

	postJSON(t, s, "POST", "/api/clear-emergency-stop", ``)
	expectControl("hold", false)
}

func TestObstacleAPI_UnknownIDIs404AndDoesNotDirtyState(t *testing.T) {
	s := NewWebServer(":0")
	postJSON(t, s, "POST", "/api/obstacles", `{"pixel_top_left":[10,10],"pixel_bottom_right":[20,20],"name":"a"}`)
	s.obstacles.saved = true // pretend it was just saved

	if w := postJSON(t, s, "DELETE", "/api/obstacles/nope", ``); w.Code != http.StatusNotFound {
		t.Errorf("DELETE unknown: status %d, want 404", w.Code)
	}
	if w := postJSON(t, s, "PUT", "/api/obstacles/nope", `{"pixel_top_left":[1,1],"pixel_bottom_right":[2,2]}`); w.Code != http.StatusNotFound {
		t.Errorf("PUT unknown: status %d, want 404", w.Code)
	}
	if !s.obstacles.saved {
		t.Error("a failed delete/update must not mark the obstacle list as unsaved")
	}
	if len(s.obstacles.list) != 1 {
		t.Errorf("obstacle list changed: %d entries, want 1", len(s.obstacles.list))
	}
}

// Update must not mutate a snapshot already handed out (to the planner
// callback or the file saver).
func TestObstacleAPI_UpdateDoesNotMutateEarlierSnapshot(t *testing.T) {
	s := NewWebServer(":0")
	postJSON(t, s, "POST", "/api/obstacles", `{"pixel_top_left":[10,10],"pixel_bottom_right":[20,20],"name":"a"}`)
	s.obstacles.mutex.RLock()
	snapshot := s.obstacles.list
	s.obstacles.mutex.RUnlock()
	before := snapshot[0].PixelsBottomRight

	w := postJSON(t, s, "PUT", "/api/obstacles/a", `{"pixel_top_left":[10,10],"pixel_bottom_right":[99,99]}`)

	if w.Code != http.StatusOK {
		t.Fatalf("PUT status %d, want 200", w.Code)
	}
	if snapshot[0].PixelsBottomRight != before {
		t.Errorf("update mutated an earlier snapshot in place: %v -> %v", before, snapshot[0].PixelsBottomRight)
	}
	if got := s.obstacles.list[0].PixelsBottomRight; got != [2]int{99, 99} {
		t.Errorf("update not applied: %v", got)
	}
}

func TestObstacleAPI_NamesStayUnique(t *testing.T) {
	s := NewWebServer(":0")
	for i := 0; i < 3; i++ {
		postJSON(t, s, "POST", "/api/obstacles", `{"pixel_top_left":[10,10],"pixel_bottom_right":[20,20],"name":"obstacle_1"}`)
	}
	seen := map[string]bool{}
	for _, o := range s.obstacles.list {
		if seen[o.Name] {
			t.Fatalf("duplicate obstacle name %q", o.Name)
		}
		seen[o.Name] = true
	}
	if len(seen) != 3 {
		t.Errorf("got %d obstacles, want 3", len(seen))
	}
}

func TestCalibrationCancel_KeepsAWorkingCalibration(t *testing.T) {
	s := serverWithFrame(t, 640, 480)
	s.calibration.positionEstimator.GetHomography().SetFromValues(0.01, 0, 0, 0, 0.01, 0, 0, 0, 1)
	if !s.calibration.positionEstimator.IsCalibrated() {
		t.Fatal("test setup: estimator should be calibrated")
	}
	s.SetCalibrationState("detecting", "Looking for tags", "", 0.15)

	postJSON(t, s, "POST", "/api/calibration/cancel", ``)

	s.calibration.mutex.RLock()
	state := s.calibration.state
	s.calibration.mutex.RUnlock()
	if state != "calibrated" {
		t.Errorf("state after cancelling on a calibrated system = %q, want calibrated", state)
	}
}

func TestCalibrationCancel_UncalibratedStaysUncalibrated(t *testing.T) {
	s := NewWebServer(":0")
	s.SetCalibrationState("detecting", "Looking for tags", "", 0.15)

	postJSON(t, s, "POST", "/api/calibration/cancel", ``)

	s.calibration.mutex.RLock()
	state := s.calibration.state
	s.calibration.mutex.RUnlock()
	if state != "not_calibrated" {
		t.Errorf("state = %q, want not_calibrated", state)
	}
}

func TestCalibrationCompute_RejectsTooManyTags(t *testing.T) {
	s := NewWebServer(":0")
	tags := make([]string, maxCalibrationTags+1)
	for i := range tags {
		tags[i] = `{"id":100,"corners":[[0,0],[1,0],[1,1],[0,1]]}`
	}
	body := `{"tags":[` + strings.Join(tags, ",") + `]}`

	w := postJSON(t, s, "POST", "/api/calibration/compute", body)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status %d, want 400", w.Code)
	}
	// The fitter would also reject these bogus tags with a 400, so check that
	// the request was refused by the size cap itself.
	var resp CalibrationComputeResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(resp.Error, "too many tags") {
		t.Errorf("error = %q, want the too-many-tags cap to reject it", resp.Error)
	}
}

func TestCalibrationMessage_CarriesResolutionMismatch(t *testing.T) {
	s := serverWithFrame(t, 640, 480)
	// Calibrated at a different resolution than the camera now delivers.
	dir := t.TempDir()
	path := dir + "/cal.yaml"
	yaml := "version: 2\ncamera:\n  name: x\n  resolution: [1280, 720]\nhomography:\n  - [0.01, 0, 0]\n  - [0, 0.01, 0]\n  - [0, 0, 1]\nworld_scale: 100\n"
	if err := os.WriteFile(path, []byte(yaml), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.calibration.positionEstimator.LoadCalibration(path); err != nil {
		t.Fatal(err)
	}
	if !s.calibration.positionEstimator.ResolutionMismatch() {
		t.Fatal("test setup: expected a resolution mismatch")
	}

	httpServer := httptest.NewServer(s.router.engine)
	defer httpServer.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpServer.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	waitForClients(t, s, 1)

	s.SetCalibrationState("calibrated", "loaded", "x.yaml", 0.15)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("no calibration message: %v", err)
		}
		var msg struct {
			Type        string `json:"type"`
			Calibration *struct {
				ResolutionMismatch bool `json:"resolutionMismatch"`
			} `json:"calibration"`
		}
		if json.Unmarshal(data, &msg) == nil && msg.Type == "calibration" && msg.Calibration != nil {
			if !msg.Calibration.ResolutionMismatch {
				t.Errorf("calibration broadcast dropped resolutionMismatch: %s", data)
			}
			return
		}
	}
}

func TestObstacleAPI_RejectsOutOfFrameAndEmptyBoxes(t *testing.T) {
	s := serverWithFrame(t, 640, 480)

	bad := []string{
		`{"pixel_top_left":[600,10],"pixel_bottom_right":[700,50],"name":"a"}`,
		`{"pixel_top_left":[-5,10],"pixel_bottom_right":[50,50],"name":"a"}`,
		`{"pixel_top_left":[10,10],"pixel_bottom_right":[10,10],"name":"a"}`,
		`{"pixel_top_left":[50,50],"pixel_bottom_right":[10,10],"name":"a"}`,
	}
	for _, body := range bad {
		if w := postJSON(t, s, "POST", "/api/obstacles", body); w.Code != http.StatusBadRequest {
			t.Errorf("add %s: status %d, want 400", body, w.Code)
		}
	}
	if len(s.obstacles.list) != 0 {
		t.Errorf("invalid obstacles were stored: %d", len(s.obstacles.list))
	}

	postJSON(t, s, "POST", "/api/obstacles", `{"pixel_top_left":[10,10],"pixel_bottom_right":[50,50],"name":"ok"}`)
	if w := postJSON(t, s, "PUT", "/api/obstacles/ok", `{"pixel_top_left":[10,10],"pixel_bottom_right":[700,50]}`); w.Code != http.StatusBadRequest {
		t.Errorf("update out of frame: status %d, want 400", w.Code)
	}
	if got := s.obstacles.list[0].PixelsBottomRight; got != [2]int{50, 50} {
		t.Errorf("rejected update still changed the obstacle: %v", got)
	}
}
