package tracking

import (
	"testing"
)

func TestNewByteTrack(t *testing.T) {
	bt := NewByteTrack(nil)

	if bt == nil {
		t.Fatal("NewByteTrack returned nil")
	}
	if bt.tracks == nil {
		t.Error("tracks should not be nil")
	}
	if bt.nextTrackID != 0 {
		t.Errorf("nextTrackID = %d, want 0", bt.nextTrackID)
	}
	if bt.frameCount != 0 {
		t.Errorf("frameCount = %d, want 0", bt.frameCount)
	}
}

func TestNewByteTrack_WithConfig(t *testing.T) {
	config := &ByteTrackConfig{
		TrackThresh: 0.7,
		TrackBuffer: 60,
		MatchThresh: 0.6,
		MinBoxArea:  200,
	}
	bt := NewByteTrack(config)

	if bt.config.TrackThresh != 0.7 {
		t.Errorf("TrackThresh = %f, want 0.7", bt.config.TrackThresh)
	}
	if bt.config.TrackBuffer != 60 {
		t.Errorf("TrackBuffer = %d, want 60", bt.config.TrackBuffer)
	}
}

func TestByteTrack_Update_EmptyDetections(t *testing.T) {
	bt := NewByteTrack(nil)

	result := bt.Update([]Detection{}, 1000.0, 1)

	if result == nil {
		t.Error("Update should return non-nil result")
		return
	}
	if result.FrameIdx != 1 {
		t.Errorf("FrameIdx = %d, want 1", result.FrameIdx)
	}
	if result.NumDetections != 0 {
		t.Errorf("NumDetections = %d, want 0", result.NumDetections)
	}
}

func TestByteTrack_Update_SingleDetection(t *testing.T) {
	bt := NewByteTrack(nil)

	detections := []Detection{
		{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6},
	}
	result := bt.Update(detections, 1000.0, 1)

	if result.NumTrackers != 1 {
		t.Errorf("After one detection, NumTrackers = %d, want 1", result.NumTrackers)
	}
	if len(result.Tracks) != 1 {
		t.Errorf("After one detection, Tracks length = %d, want 1", len(result.Tracks))
	}
}

func TestByteTrack_Update_MultipleDetections(t *testing.T) {
	bt := NewByteTrack(nil)

	detections := []Detection{
		{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6},
		{Bbox: [4]int{300, 400, 400, 500}, Confidence: 0.6},
	}
	result := bt.Update(detections, 1000.0, 1)

	if result.NumTrackers != 2 {
		t.Errorf("After two detections, NumTrackers = %d, want 2", result.NumTrackers)
	}
}

func TestByteTrack_Update_TrackConfirmed(t *testing.T) {
	bt := NewByteTrack(nil)

	bbox := [4]int{10, 20, 100, 200}
	for i := 0; i < 3; i++ {
		detections := []Detection{
			{Bbox: bbox, Confidence: 0.6},
		}
		bt.Update(detections, float64(1000+i), i+1)
	}

	result := bt.Update([]Detection{}, 1003.0, 4)
	if len(result.Tracks) != 1 {
		t.Fatalf("After 3 frames, should have 1 track, got %d", len(result.Tracks))
	}
	if result.Tracks[0].State != TrackStateConfirmed {
		t.Errorf("After 3 hits, State = %v, want TrackStateConfirmed", result.Tracks[0].State)
	}
}

func TestByteTrack_Update_SameDetection(t *testing.T) {
	bt := NewByteTrack(nil)

	bbox := [4]int{10, 20, 100, 200}
	for i := 0; i < 5; i++ {
		detections := []Detection{
			{Bbox: bbox, Confidence: 0.6},
		}
		bt.Update(detections, float64(1000+i), i+1)
	}

	result := bt.Update([]Detection{}, 1005.0, 6)

	if result.NumTrackers != 1 {
		t.Errorf("Same detection multiple times should still be 1 track, got %d", result.NumTrackers)
	}
}

func TestByteTrack_Update_MatchThreshBelowHalfIsHonored(t *testing.T) {
	// Regression test: bytetrack.go used to re-gate every match with a
	// hardcoded `< 0.5` after ComputeIoUCost had already applied the
	// configured MatchThresh, making any MatchThresh below 0.5 a no-op.
	// A MatchThresh of 0.3 must accept a match whose IoU falls in
	// [0.3, 0.5) instead of silently spawning a new track for it.
	config := &ByteTrackConfig{
		TrackThresh: 0.5,
		TrackBuffer: 30,
		MatchThresh: 0.3,
		MinBoxArea:  100,
	}
	bt := NewByteTrack(config)

	bt.Update([]Detection{{Bbox: [4]int{0, 0, 100, 100}, Confidence: 0.6}}, 1000.0, 1)

	// Shifted just enough that IoU ≈ 0.40 — inside the configured
	// MatchThresh (0.3) but below the old hardcoded 0.5 re-gate.
	result := bt.Update([]Detection{{Bbox: [4]int{43, 0, 143, 100}, Confidence: 0.6}}, 1001.0, 2)

	if result.NumTrackers != 1 {
		t.Errorf("IoU in [MatchThresh, 0.5) should still match the existing track, got %d trackers", result.NumTrackers)
	}
}

func TestByteTrack_Update_LowConfidenceFiltered(t *testing.T) {
	bt := NewByteTrack(nil)

	detections := []Detection{
		{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.3},
	}
	result := bt.Update(detections, 1000.0, 1)

	if result.NumTrackers != 0 {
		t.Errorf("Low confidence detection should be filtered, got %d trackers", result.NumTrackers)
	}
}

func TestByteTrack_Update_SmallBoxFiltered(t *testing.T) {
	bt := NewByteTrack(nil)

	detections := []Detection{
		{Bbox: [4]int{10, 20, 15, 25}, Confidence: 0.9},
	}
	result := bt.Update(detections, 1000.0, 1)

	if result.NumTrackers != 0 {
		t.Errorf("Small box (area=50 < 100) should be filtered, got %d trackers", result.NumTrackers)
	}
}

func TestByteTrack_Reset(t *testing.T) {
	bt := NewByteTrack(nil)
	bt.Update([]Detection{{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6}}, 1000.0, 1)

	if bt.GetTrackCount() != 1 {
		t.Errorf("Before reset, track count = %d, want 1", bt.GetTrackCount())
	}

	bt.Reset()

	if bt.GetTrackCount() != 0 {
		t.Errorf("After reset, track count = %d, want 0", bt.GetTrackCount())
	}
	if bt.nextTrackID != 0 {
		t.Errorf("After reset, nextTrackID = %d, want 0", bt.nextTrackID)
	}
	if bt.frameCount != 0 {
		t.Errorf("After reset, frameCount = %d, want 0", bt.frameCount)
	}
}

func TestByteTrack_GetTrackCount(t *testing.T) {
	bt := NewByteTrack(nil)

	if bt.GetTrackCount() != 0 {
		t.Errorf("Empty tracker should have 0 tracks, got %d", bt.GetTrackCount())
	}

	bt.Update([]Detection{{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6}}, 1000.0, 1)

	if bt.GetTrackCount() != 1 {
		t.Errorf("After one detection, track count = %d, want 1", bt.GetTrackCount())
	}
}

func TestByteTrack_Update_DifferentLocations(t *testing.T) {
	bt := NewByteTrack(nil)

	detections := []Detection{
		{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6},
	}
	bt.Update(detections, 1000.0, 1)

	detections = []Detection{
		{Bbox: [4]int{400, 500, 500, 600}, Confidence: 0.6},
	}
	result := bt.Update(detections, 1001.0, 2)

	if result.NumTrackers != 2 {
		t.Errorf("Different locations should create different tracks, got %d", result.NumTrackers)
	}
}

func TestByteTrack_Update_FrameCount(t *testing.T) {
	bt := NewByteTrack(nil)

	bt.Update([]Detection{}, 1000.0, 1)
	bt.Update([]Detection{}, 1001.0, 2)
	bt.Update([]Detection{}, 1002.0, 3)

	if bt.frameCount != 3 {
		t.Errorf("frameCount = %d, want 3", bt.frameCount)
	}
}

func TestByteTrack_Update_Timestamp(t *testing.T) {
	bt := NewByteTrack(nil)

	result := bt.Update([]Detection{{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6}}, 1234.5, 1)

	if result.Timestamp != 1234.5 {
		t.Errorf("Timestamp = %f, want 1234.5", result.Timestamp)
	}
}

func TestByteTrack_PredictDt_FirstCallDefaultsToOne(t *testing.T) {
	bt := NewByteTrack(nil)

	if dt := bt.predictDt(12345.0); dt != 1.0 {
		t.Errorf("predictDt before any Update = %f, want 1.0", dt)
	}
}

func TestByteTrack_PredictDt_UsesElapsedTimeSinceLastUpdate(t *testing.T) {
	bt := NewByteTrack(nil)
	bt.Update([]Detection{{Bbox: [4]int{0, 0, 100, 100}, Confidence: 0.9}}, 1000.0, 1)

	// Simulate a frame drop / latency spike: the next real frame arrives 5
	// seconds later, not the ~1 frame period a hardcoded dt=1 would assume.
	if dt := bt.predictDt(1005.0); dt != 5.0 {
		t.Errorf("predictDt after a 5s gap = %f, want 5.0", dt)
	}
}

func TestByteTrack_PredictDt_NonPositiveDeltaFallsBackToOne(t *testing.T) {
	bt := NewByteTrack(nil)
	bt.Update([]Detection{{Bbox: [4]int{0, 0, 100, 100}, Confidence: 0.9}}, 1000.0, 1)

	if dt := bt.predictDt(1000.0); dt != 1.0 {
		t.Errorf("predictDt with a duplicate timestamp = %f, want 1.0 fallback", dt)
	}
	if dt := bt.predictDt(999.0); dt != 1.0 {
		t.Errorf("predictDt with a backward-moving clock = %f, want 1.0 fallback", dt)
	}
}

// TestByteTrack_PredictAllTracks_UsesGivenDt is a regression test for the
// bug where predictAllTracks called kf.Predict() with no dt (implicit
// dt=1), so a track's predicted position never reflected real elapsed time
// between frames. It bypasses the Kalman filter's own (separately tracked,
// and not the subject of this test) velocity estimation by setting a known
// velocity directly via Initialize, isolating dt handling in
// predictAllTracks/Predict.
func TestByteTrack_PredictAllTracks_UsesGivenDt(t *testing.T) {
	bt := NewByteTrack(nil)
	bt.Update([]Detection{{Bbox: [4]int{0, 0, 100, 100}, Confidence: 0.9}}, 1000.0, 1)

	var tt *TrackedTrack
	for _, v := range bt.tracks {
		tt = v
	}
	if tt == nil {
		t.Fatal("expected a track to exist after Update")
	}
	tt.kf.Initialize(50, 50, 10, 0) // position (50,50), vx=10/s

	bt.predictAllTracks(2.0)

	state := tt.kf.GetState()
	if state[0] != 70 {
		t.Errorf("predictAllTracks(2.0) with vx=10 => x[0]=%f, want 70 (50 + 10*2)", state[0])
	}
}

func TestTrackedTrack_Struct(t *testing.T) {
	tt := &TrackedTrack{
		track:           NewTrack(1, [4]int{10, 20, 100, 200}, 1000.0, 0.6),
		kf:              NewKalmanFilter(),
		timeSinceUpdate: 0,
	}

	if tt.track.TrackID != 1 {
		t.Errorf("track.TrackID = %d, want 1", tt.track.TrackID)
	}
	if tt.kf == nil {
		t.Error("kf should not be nil")
	}
	if tt.timeSinceUpdate != 0 {
		t.Errorf("timeSinceUpdate = %d, want 0", tt.timeSinceUpdate)
	}
}

func TestByteTrack_Update_TagIDPreserved(t *testing.T) {
	bt := NewByteTrack(nil)
	tagID := 5

	detections := []Detection{
		{Bbox: [4]int{10, 20, 100, 200}, Confidence: 0.6, TagID: &tagID},
	}
	bt.Update(detections, 1000.0, 1)

	result := bt.Update([]Detection{}, 1001.0, 2)

	if len(result.Tracks) != 1 {
		t.Fatalf("Expected 1 track, got %d", len(result.Tracks))
	}
	if result.Tracks[0].TagID == nil || *result.Tracks[0].TagID != 5 {
		t.Error("TagID should be preserved across updates")
	}
}

func TestByteTrack_Update_MultipleFrames(t *testing.T) {
	bt := NewByteTrack(nil)

	for i := 0; i < 10; i++ {
		result := bt.Update([]Detection{}, float64(1000+i), i+1)
		if result.FrameIdx != i+1 {
			t.Errorf("FrameIdx = %d, want %d", result.FrameIdx, i+1)
		}
	}
}

// A track whose detections stop must not keep reporting "confirmed" for the
// whole track buffer: autonomous driving acts only on confirmed tracks, so a
// robot whose tag vanished would otherwise be driven from a frozen position.
func TestByteTrack_ReportsTrackLostAfterMissedFrames(t *testing.T) {
	bt := NewByteTrack(nil) // TrackBuffer 30
	tag := 5
	det := []Detection{{Bbox: [4]int{100, 100, 140, 140}, Confidence: 1, TagID: &tag}}

	ts := 0.066
	res := bt.Update(det, ts, 0)
	for i := 1; i < 4; i++ {
		ts += 0.066
		res = bt.Update(det, ts, i)
	}
	if len(res.Tracks) != 1 || res.Tracks[0].State != TrackStateConfirmed {
		t.Fatalf("setup: want one confirmed track, got %+v", res.Tracks)
	}

	for miss := 1; miss <= LostAfterMissedFrames+2; miss++ {
		ts += 0.066
		res = bt.Update(nil, ts, 10+miss)
		if len(res.Tracks) != 1 {
			t.Fatalf("miss %d: tracker dropped the track early", miss)
		}
		want := TrackStateConfirmed
		if miss > LostAfterMissedFrames {
			want = TrackStateLost
		}
		if got := res.Tracks[0].State; got != want {
			t.Errorf("after %d missed frames: state %s, want %s", miss, res.Tracks[0].StateString(), (&Track{State: want}).StateString())
		}
	}

	// Seen again: confirmed straight away, same track.
	ts += 0.066
	res = bt.Update(det, ts, 99)
	if len(res.Tracks) != 1 || res.Tracks[0].State != TrackStateConfirmed {
		t.Errorf("after re-acquisition: %+v, want one confirmed track", res.Tracks)
	}
}
