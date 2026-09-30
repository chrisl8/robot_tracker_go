//go:build gocv

package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestHandleStatus_ReportsRobotInfo(t *testing.T) {
	gin.SetMode(gin.TestMode)
	s := NewWebServer(":0")
	s.SetRobotInfo("alive", "asleep", "W", 2)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/status", nil)
	s.handleStatus(c)

	var got map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("bad JSON %q: %v", w.Body.String(), err)
	}
	for key, want := range map[string]any{
		"robotLink":    "alive",
		"robotServos":  "asleep",
		"robotMode":    "W",
		"robotReboots": float64(2),
	} {
		if got[key] != want {
			t.Errorf("%s = %v, want %v", key, got[key], want)
		}
	}
}

func TestStatusMessage_OmitsUnknownRobotDetails(t *testing.T) {
	b, err := json.Marshal(StatusMessage{ArduinoState: "Connected", RobotLink: "silent"})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	_ = json.Unmarshal(b, &got)
	if _, present := got["robotServos"]; present {
		t.Error("robotServos should be omitted when unknown")
	}
	if _, present := got["robotMode"]; present {
		t.Error("robotMode should be omitted when unknown")
	}
	if v, present := got["robotReboots"]; !present || v != float64(0) {
		t.Errorf("robotReboots = %v (present=%v), want an explicit 0", v, present)
	}
}
