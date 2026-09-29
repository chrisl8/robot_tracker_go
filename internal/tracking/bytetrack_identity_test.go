package tracking

import "testing"

func tagged(bbox [4]int, tag int, conf float64) Detection {
	id := tag
	return Detection{Bbox: bbox, Confidence: conf, TagID: &id}
}

func trackByTag(res *TrackingResult, tag int) *Track {
	for i := range res.Tracks {
		if res.Tracks[i].TagID != nil && *res.Tracks[i].TagID == tag {
			return &res.Tracks[i]
		}
	}
	return nil
}

func mustTrackByTag(t *testing.T, res *TrackingResult, tag int) *Track {
	t.Helper()
	tr := trackByTag(res, tag)
	if tr == nil {
		t.Fatalf("no track with tag %d in %+v", tag, res.Tracks)
	}
	return tr
}

// Two robots swap places between frames (their boxes now overlap the OTHER
// robot's previous box most). The tag ID is authoritative: each track must
// stay with its own tag rather than follow whichever box overlaps.
func TestByteTrack_TagIdentityWinsOverIoU(t *testing.T) {
	bt := NewByteTrack(nil)
	a := [4]int{100, 100, 160, 160}
	b := [4]int{170, 100, 230, 160}

	bt.Update([]Detection{tagged(a, 1, 1), tagged(b, 2, 1)}, 0.0, 1)
	first := bt.Update([]Detection{tagged(a, 1, 1), tagged(b, 2, 1)}, 0.033, 2)
	id1 := mustTrackByTag(t, first, 1).TrackID
	id2 := mustTrackByTag(t, first, 2).TrackID

	// Tag 1 jumps to where tag 2 was, and tag 2 to where tag 1 was.
	res := bt.Update([]Detection{tagged(b, 1, 1), tagged(a, 2, 1)}, 0.066, 3)

	if got := trackByTag(res, 1); got == nil || got.TrackID != id1 {
		t.Errorf("tag 1 changed track: %+v, want track %d", got, id1)
	}
	if got := trackByTag(res, 2); got == nil || got.TrackID != id2 {
		t.Errorf("tag 2 changed track: %+v, want track %d", got, id2)
	}
	if len(res.Tracks) != 2 {
		t.Errorf("tracks = %d, want 2 (no duplicates)", len(res.Tracks))
	}
	if got := trackByTag(res, 1); got != nil && got.Bbox != b {
		t.Errorf("tag 1 bbox = %v, want %v", got.Bbox, b)
	}
}

// A detection carrying tag 5 must never be merged into a track that already
// belongs to tag 1, even when the boxes overlap heavily.
func TestByteTrack_DifferentTagsNeverShareATrack(t *testing.T) {
	bt := NewByteTrack(nil)
	box := [4]int{100, 100, 160, 160}
	bt.Update([]Detection{tagged(box, 1, 1)}, 0.0, 1)

	res := bt.Update([]Detection{tagged(box, 5, 1)}, 0.033, 2)

	if trackByTag(res, 5) == nil {
		t.Fatal("tag 5 should have its own track")
	}
	if t1 := trackByTag(res, 1); t1 == nil || t1.Hits != 1 {
		t.Errorf("tag 1's track should be untouched, got %+v", t1)
	}
}

// The low-confidence pass must only revive tracks that were not already
// updated this frame; it used to re-update tracks matched (or just created) in
// the high-confidence pass, double-counting hits and overwriting their bbox.
func TestByteTrack_LowConfPassSkipsTracksAlreadyUpdatedThisFrame(t *testing.T) {
	bt := NewByteTrack(nil)
	box := [4]int{100, 100, 160, 160}
	bt.Update([]Detection{tagged(box, 1, 1)}, 0.0, 1)

	high := tagged(box, 1, 1)
	low := Detection{Bbox: [4]int{102, 102, 162, 162}, Confidence: 0.1}
	res := bt.Update([]Detection{high, low}, 0.033, 2)

	got := trackByTag(res, 1)
	if got == nil {
		t.Fatal("track missing")
	}
	if got.Hits != 2 {
		t.Errorf("Hits = %d, want 2 (one per frame, not double-counted)", got.Hits)
	}
	if got.Bbox != box {
		t.Errorf("Bbox = %v, want %v (low-conf detection must not overwrite it)", got.Bbox, box)
	}
}

func TestByteTrack_LowConfPassStillRevivesUnmatchedTrack(t *testing.T) {
	bt := NewByteTrack(nil)
	box := [4]int{100, 100, 160, 160}
	bt.Update([]Detection{tagged(box, 1, 1)}, 0.0, 1)

	// Only a low-confidence detection this frame (e.g. partial occlusion).
	res := bt.Update([]Detection{{Bbox: box, Confidence: 0.2}}, 0.033, 2)

	if len(res.Tracks) != 1 || res.Tracks[0].Hits != 2 {
		t.Errorf("tracks = %+v, want the one track revived (Hits 2)", res.Tracks)
	}
}

// The low-confidence pass must honor the configured MatchThresh like the
// high-confidence pass does, instead of a hardcoded 0.3.
func TestByteTrack_LowConfPassHonorsMatchThresh(t *testing.T) {
	bt := NewByteTrack(&ByteTrackConfig{TrackThresh: 0.5, TrackBuffer: 30, MatchThresh: 0.9, MinBoxArea: 1})
	box := [4]int{100, 100, 160, 160}
	bt.Update([]Detection{{Bbox: box, Confidence: 1}}, 0.0, 1)

	// IoU ~0.56: above 0.3 but below the configured 0.9.
	low := Detection{Bbox: [4]int{120, 100, 180, 160}, Confidence: 0.2}
	res := bt.Update([]Detection{low}, 0.033, 2)

	if len(res.Tracks) != 1 || res.Tracks[0].Hits != 1 {
		t.Errorf("tracks = %+v, want the track NOT matched (Hits 1)", res.Tracks)
	}
}

// Track IDs handed to new detections in one frame, and the order of the
// result, must not depend on Go's random map iteration.
func TestByteTrack_DeterministicIDsAndOrder(t *testing.T) {
	boxes := [][4]int{{0, 0, 50, 50}, {100, 0, 150, 50}, {200, 0, 250, 50}, {300, 0, 350, 50}, {400, 0, 450, 50}}
	for run := 0; run < 50; run++ {
		bt := NewByteTrack(nil)
		dets := make([]Detection, len(boxes))
		for i, b := range boxes {
			dets[i] = Detection{Bbox: b, Confidence: 1}
		}
		res := bt.Update(dets, 0.0, 1)
		for i, tr := range res.Tracks {
			if tr.TrackID != i {
				t.Fatalf("run %d: result[%d].TrackID = %d, want %d (ids/order must be deterministic)", run, i, tr.TrackID, i)
			}
			if tr.Bbox != boxes[i] {
				t.Fatalf("run %d: track %d got bbox %v, want %v (ids follow detection order)", run, i, tr.Bbox, boxes[i])
			}
		}
	}
}

func TestByteTrack_ResetClearsTimestamp(t *testing.T) {
	bt := NewByteTrack(nil)
	bt.Update([]Detection{{Bbox: [4]int{0, 0, 50, 50}, Confidence: 1}}, 100.0, 1)

	bt.Reset()

	if bt.hasLastTimestamp {
		t.Error("Reset must clear hasLastTimestamp, or the first predict after it uses a stale dt")
	}
	if got := bt.predictDt(500.0); got != 1.0 {
		t.Errorf("predictDt after Reset = %v, want the 1.0 first-frame default", got)
	}
}
