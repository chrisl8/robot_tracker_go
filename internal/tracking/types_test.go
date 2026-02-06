package tracking

import (
	"testing"
)

func TestTrackState(t *testing.T) {
	tests := []struct {
		state    TrackState
		expected string
	}{
		{TrackStateTentative, "tentative"},
		{TrackStateConfirmed, "confirmed"},
		{TrackStateLost, "lost"},
		{TrackState(99), "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			track := &Track{State: tt.state}
			if result := track.StateString(); result != tt.expected {
				t.Errorf("StateString() = %s, want %s", result, tt.expected)
			}
		})
	}
}

func TestNewTrack(t *testing.T) {
	bbox := [4]int{10, 20, 100, 200}
	track := NewTrack(1, bbox, 1234.5, 0.9)

	if track.TrackID != 1 {
		t.Errorf("TrackID = %d, want 1", track.TrackID)
	}
	if track.Bbox != bbox {
		t.Errorf("Bbox = %v, want %v", track.Bbox, bbox)
	}
	if track.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", track.Timestamp)
	}
	if track.Confidence != 0.9 {
		t.Errorf("Confidence = %f, want 0.9", track.Confidence)
	}
	if track.Age != 1 {
		t.Errorf("Age = %d, want 1", track.Age)
	}
	if track.Hits != 1 {
		t.Errorf("Hits = %d, want 1", track.Hits)
	}
	if track.State != TrackStateTentative {
		t.Errorf("State = %v, want TrackStateTentative", track.State)
	}
	if len(track.History) != 0 {
		t.Errorf("History length = %d, want 0", len(track.History))
	}
}

func TestTrack_Update(t *testing.T) {
	track := NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.5)

	track.Update([4]int{15, 25, 105, 205}, 1001.0, 0.6)

	if track.Bbox != [4]int{15, 25, 105, 205} {
		t.Errorf("Bbox = %v, want [15, 25, 105, 205]", track.Bbox)
	}
	if track.Timestamp != 1001.0 {
		t.Errorf("Timestamp = %f, want 1001.0", track.Timestamp)
	}
	if track.Confidence != 0.6 {
		t.Errorf("Confidence = %f, want 0.6", track.Confidence)
	}
	if track.Age != 2 {
		t.Errorf("Age = %d, want 2", track.Age)
	}
	if track.Hits != 2 {
		t.Errorf("Hits = %d, want 2", track.Hits)
	}
	if len(track.History) != 1 {
		t.Errorf("History length = %d, want 1", len(track.History))
	}
}

func TestTrack_Update_BecomesConfirmed(t *testing.T) {
	track := NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.5)

	for i := 0; i < 2; i++ {
		track.Update([4]int{10 + i, 20 + i, 100 + i, 200 + i}, float64(1000+i), 0.6)
	}

	if track.Hits != 3 {
		t.Errorf("Hits = %d, want 3", track.Hits)
	}
	if track.State != TrackStateConfirmed {
		t.Errorf("After 3 hits, State = %v, want TrackStateConfirmed", track.State)
	}
}

func TestTrack_AgeTrack(t *testing.T) {
	track := NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.5)

	track.AgeTrack()

	if track.Age != 2 {
		t.Errorf("Age = %d, want 2", track.Age)
	}
}

func TestTrack_ToDict(t *testing.T) {
	tagID := 5
	track := NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.9)
	track.TagID = &tagID
	track.State = TrackStateConfirmed

	dict := track.ToDict()

	if dict["track_id"].(int) != 1 {
		t.Errorf("track_id = %v, want 1", dict["track_id"])
	}
	if dict["state"].(string) != "confirmed" {
		t.Errorf("state = %v, want confirmed", dict["state"])
	}
	if dict["tag_id"].(*int) != &tagID {
		t.Error("tag_id mismatch")
	}
}

func TestTrack_ToDict_HistoryLimited(t *testing.T) {
	track := NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.5)

	for i := 0; i < 50; i++ {
		track.Update([4]int{10 + i, 20 + i, 100 + i, 200 + i}, float64(1000+i), 0.6)
	}

	dict := track.ToDict()
	history := dict["history"].([]map[string]interface{})

	if len(history) > 30 {
		t.Errorf("History should be limited to 30, got %d", len(history))
	}
}

func TestTrackHistoryPoint(t *testing.T) {
	hp := TrackHistoryPoint{
		Timestamp: 1234.5,
		Bbox:      [4]int{10, 20, 100, 200},
	}

	if hp.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", hp.Timestamp)
	}
	if hp.Bbox != [4]int{10, 20, 100, 200} {
		t.Errorf("Bbox = %v, want [10, 20, 100, 200]", hp.Bbox)
	}
}

func TestTrackingResult(t *testing.T) {
	result := TrackingResult{
		Tracks:        []Track{},
		Timestamp:     1234.5,
		FrameIdx:      10,
		NumDetections: 5,
		NumTrackers:   3,
	}

	if result.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", result.Timestamp)
	}
	if result.FrameIdx != 10 {
		t.Errorf("FrameIdx = %d, want 10", result.FrameIdx)
	}
	if result.NumDetections != 5 {
		t.Errorf("NumDetections = %d, want 5", result.NumDetections)
	}
	if result.NumTrackers != 3 {
		t.Errorf("NumTrackers = %d, want 3", result.NumTrackers)
	}
}

func TestDetection(t *testing.T) {
	tagID := 7
	classID := 2
	det := Detection{
		Bbox:       [4]int{10, 20, 100, 200},
		Confidence: 0.85,
		ClassID:    classID,
		TagID:      &tagID,
	}

	if det.Bbox != [4]int{10, 20, 100, 200} {
		t.Errorf("Bbox = %v, want [10, 20, 100, 200]", det.Bbox)
	}
	if det.Confidence != 0.85 {
		t.Errorf("Confidence = %f, want 0.85", det.Confidence)
	}
	if det.ClassID != classID {
		t.Errorf("ClassID = %d, want %d", det.ClassID, classID)
	}
	if det.TagID != &tagID {
		t.Error("TagID mismatch")
	}
}

func TestTrackerInterface(t *testing.T) {
	var tracker Tracker
	tracker = NewByteTrack(nil)

	result := tracker.Update([]Detection{}, 1000.0, 1)

	if result == nil {
		t.Error("Update should return non-nil result")
	}
	if result.FrameIdx != 1 {
		t.Errorf("FrameIdx = %d, want 1", result.FrameIdx)
	}

	tracker.Reset()
}
