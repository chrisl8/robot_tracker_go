package tracking

import "sort"

// LostAfterMissedFrames is how many consecutive frames a track may go without
// a matching detection and still be reported as confirmed. Beyond that it is
// reported as TrackStateLost (until it is matched again), even though the
// tracker keeps it for TrackBuffer frames so it can be re-acquired. Consumers
// that act on a track's position (autonomous driving) must not treat a robot
// as located just because the tracker is still holding its slot.
const LostAfterMissedFrames = 3

type ByteTrackConfig struct {
	TrackThresh float64
	TrackBuffer int
	MatchThresh float64
	MinBoxArea  int
}

type ByteTrack struct {
	config      *ByteTrackConfig
	tracks      map[int]*TrackedTrack
	nextTrackID int
	frameCount  int

	// lastTimestamp/hasLastTimestamp track when predictAllTracks last ran,
	// so the next Update can compute the real elapsed dt for the Kalman
	// filters instead of assuming a fixed frame period. Only updated on the
	// Update path that actually predicts (see predictDt).
	lastTimestamp    float64
	hasLastTimestamp bool
}

type TrackedTrack struct {
	track           *Track
	kf              *KalmanFilter
	timeSinceUpdate int
}

func NewByteTrack(config *ByteTrackConfig) *ByteTrack {
	if config == nil {
		config = &ByteTrackConfig{
			TrackThresh: 0.5,
			TrackBuffer: 30,
			MatchThresh: 0.3,
			MinBoxArea:  100,
		}
	}

	return &ByteTrack{
		config:      config,
		tracks:      make(map[int]*TrackedTrack),
		nextTrackID: 0,
		frameCount:  0,
	}
}

func (t *ByteTrack) Update(detections []Detection, timestamp float64, frameIdx int) *TrackingResult {
	t.frameCount++

	if len(detections) == 0 {
		return t.updateWithoutDetections(timestamp, frameIdx)
	}

	highConfDetections := make([]Detection, 0, len(detections))
	lowConfDetections := make([]Detection, 0, len(detections))

	for _, det := range detections {
		area := (det.Bbox[2] - det.Bbox[0]) * (det.Bbox[3] - det.Bbox[1])
		if area < t.config.MinBoxArea {
			continue
		}
		if det.Confidence >= t.config.TrackThresh {
			highConfDetections = append(highConfDetections, det)
		} else {
			lowConfDetections = append(lowConfDetections, det)
		}
	}

	dt := t.predictDt(timestamp)
	t.predictAllTracks(dt)
	t.lastTimestamp = timestamp
	t.hasLastTimestamp = true

	// Age all tracks — matched tracks get reset to 0 below
	for _, tt := range t.tracks {
		tt.timeSinceUpdate++
		tt.track.AgeTrack()
	}

	matchedDetections, matchedTrackIDs, unmatchedDetections := t.matchTracks(highConfDetections, false)

	// Update matched tracks
	for i, detIdx := range matchedDetections {
		trackID := matchedTrackIDs[i]
		tt := t.tracks[trackID]
		if tt == nil {
			continue
		}
		tt.track.Update(highConfDetections[detIdx].Bbox, timestamp, highConfDetections[detIdx].Confidence)
		cx, cy := bboxToCenter(highConfDetections[detIdx].Bbox)
		tt.kf.Update([2]float64{cx, cy})
		tt.timeSinceUpdate = 0
		if tt.track.TagID == nil && highConfDetections[detIdx].TagID != nil {
			tagID := *highConfDetections[detIdx].TagID
			tt.track.TagID = &tagID
		}
	}

	// Create new tracks for unmatched detections
	for _, detIdx := range unmatchedDetections {
		if detIdx < len(highConfDetections) {
			t.createNewTrack(highConfDetections[detIdx], timestamp)
		}
	}

	// Match low-confidence detections, but only against tracks that were not
	// already updated (matched or created) this frame.
	lowMatched, lowMatchedTrackIDs, _ := t.matchTracks(lowConfDetections, true)

	for i, detIdx := range lowMatched {
		if detIdx < len(lowConfDetections) {
			trackID := lowMatchedTrackIDs[i]
			tt := t.tracks[trackID]
			if tt == nil {
				continue
			}
			tt.track.Update(lowConfDetections[detIdx].Bbox, timestamp, lowConfDetections[detIdx].Confidence)
			cx, cy := bboxToCenter(lowConfDetections[detIdx].Bbox)
			tt.kf.Update([2]float64{cx, cy})
			tt.timeSinceUpdate = 0
		}
	}

	t.removeLostTracks()

	return t.buildTrackingResult(timestamp, frameIdx, len(detections))
}

// predictDt returns the elapsed seconds to use for this frame's Kalman
// predict step, computed from the real timestamp the caller passed to
// Update (see cmd/main.go's frameStart-based timestamps), rather than
// assuming a fixed frame period. Falls back to 1.0 (the filter's original
// hardcoded assumption) on the very first frame, or if the computed delta
// is non-positive (clock went backward or a duplicate timestamp), so a
// track is never predicted with an invalid dt. Note this is only called
// from the real-detections path in Update — updateWithoutDetections
// intentionally skips prediction and never advances lastTimestamp, so a run
// of no-detection frames correctly makes the next real predict use the full
// elapsed gap rather than understating it.
func (t *ByteTrack) predictDt(timestamp float64) float64 {
	if t.hasLastTimestamp {
		if d := timestamp - t.lastTimestamp; d > 0 {
			return d
		}
	}
	return 1.0
}

func (t *ByteTrack) predictAllTracks(dt float64) {
	for _, tt := range t.tracks {
		state := tt.kf.Predict(dt)
		// Update track bbox with predicted position for matching
		w := tt.track.Bbox[2] - tt.track.Bbox[0]
		h := tt.track.Bbox[3] - tt.track.Bbox[1]
		tt.track.Bbox = centerToBbox(state[0], state[1], w, h)
	}
}

// matchTracks associates detections with existing tracks. It returns the
// indices of matched detections, the parallel track IDs they matched, and the
// indices of unmatched detections (ascending). With onlyStale, tracks already
// updated this frame (timeSinceUpdate == 0) are not candidates.
//
// Tag identity is authoritative: a detection carrying a tag ID goes to the
// track holding that same tag, whatever the IoU, and IoU never pairs two
// different tags. Only the remainder is matched by IoU (Hungarian). Tracks are
// visited in ascending ID order so results don't depend on map iteration.
func (t *ByteTrack) matchTracks(detections []Detection, onlyStale bool) (matched []int, matchedTrackIDs []int, unmatched []int) {
	matched, matchedTrackIDs, unmatched = []int{}, []int{}, []int{}
	if len(detections) == 0 {
		return matched, matchedTrackIDs, unmatched
	}

	type candidate struct {
		id    int
		track *Track
	}
	var cands []candidate
	for id, tt := range t.tracks {
		if tt == nil || tt.track == nil || (onlyStale && tt.timeSinceUpdate == 0) {
			continue
		}
		cands = append(cands, candidate{id, tt.track})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].id < cands[j].id })
	trackIDs := make([]int, len(cands))
	candidates := make([]*Track, len(cands)) // parallel to trackIDs
	for j, c := range cands {
		trackIDs[j], candidates[j] = c.id, c.track
	}

	detDone := make([]bool, len(detections))
	trackDone := make([]bool, len(trackIDs))

	// 1. Tag identity.
	for i, det := range detections {
		if det.TagID == nil {
			continue
		}
		for j := range trackIDs {
			tag := candidates[j].TagID
			if !trackDone[j] && tag != nil && *tag == *det.TagID {
				matched = append(matched, i)
				matchedTrackIDs = append(matchedTrackIDs, trackIDs[j])
				detDone[i], trackDone[j] = true, true
				break
			}
		}
	}

	// 2. IoU over whatever is left.
	var restDets []int
	for i := range detections {
		if !detDone[i] {
			restDets = append(restDets, i)
		}
	}
	var restTracks []int
	for j := range trackIDs {
		if !trackDone[j] {
			restTracks = append(restTracks, j)
		}
	}

	if len(restDets) > 0 && len(restTracks) > 0 {
		dets := make([]Detection, len(restDets))
		for k, i := range restDets {
			dets[k] = detections[i]
		}
		tracks := make([]Track, len(restTracks))
		for k, j := range restTracks {
			tracks[k] = *candidates[j]
		}

		cost := ComputeIoUCost(dets, tracks, t.config.MatchThresh)
		for k := range dets {
			for l := range tracks {
				if dets[k].TagID != nil && tracks[l].TagID != nil && *dets[k].TagID != *tracks[l].TagID {
					cost[k][l] = NoMatchCost
				}
			}
		}

		assignment := Hungarian(cost)
		for k, l := range assignment.RowToCol {
			if l >= 0 && k < len(dets) && l < len(tracks) && cost[k][l] < NoMatchCost {
				i, j := restDets[k], restTracks[l]
				matched = append(matched, i)
				matchedTrackIDs = append(matchedTrackIDs, trackIDs[j])
				detDone[i] = true
			}
		}
	}

	for i := range detections {
		if !detDone[i] {
			unmatched = append(unmatched, i)
		}
	}
	return matched, matchedTrackIDs, unmatched
}

func (t *ByteTrack) createNewTrack(detection Detection, timestamp float64) {
	// If this detection has a TagID matching an existing track, reuse that track
	if detection.TagID != nil {
		for _, tt := range t.tracks {
			if tt.track.TagID != nil && *tt.track.TagID == *detection.TagID {
				tt.track.Update(detection.Bbox, timestamp, detection.Confidence)
				cx, cy := bboxToCenter(detection.Bbox)
				tt.kf.Update([2]float64{cx, cy})
				tt.timeSinceUpdate = 0
				return
			}
		}
	}

	// No existing track with this TagID — create a new one
	track := NewTrack(t.nextTrackID, detection.Bbox, timestamp, detection.Confidence)
	t.nextTrackID++

	kf := NewKalmanFilter()
	cx, cy := bboxToCenter(detection.Bbox)
	kf.Initialize(cx, cy, 0, 0)

	tt := &TrackedTrack{
		track:           track,
		kf:              kf,
		timeSinceUpdate: 0,
	}

	if detection.TagID != nil {
		tagID := *detection.TagID
		tt.track.TagID = &tagID
	}

	t.tracks[track.TrackID] = tt
}

func (t *ByteTrack) updateWithoutDetections(timestamp float64, frameIdx int) *TrackingResult {
	for _, tt := range t.tracks {
		tt.timeSinceUpdate++
		tt.track.AgeTrack()
	}

	t.removeLostTracks()

	return t.buildTrackingResult(timestamp, frameIdx, 0)
}

func (t *ByteTrack) removeLostTracks() {
	for trackID, tt := range t.tracks {
		if tt.timeSinceUpdate > t.config.TrackBuffer {
			delete(t.tracks, trackID)
		}
	}
}

func (t *ByteTrack) buildTrackingResult(timestamp float64, frameIdx int, numDetections int) *TrackingResult {
	tracks := make([]Track, 0, len(t.tracks))
	for _, tt := range t.tracks {
		snapshot := *tt.track
		if snapshot.State == TrackStateConfirmed && tt.timeSinceUpdate > LostAfterMissedFrames {
			snapshot.State = TrackStateLost // the tracker's own copy stays confirmed for re-acquisition
		}
		tracks = append(tracks, snapshot)
	}
	sort.Slice(tracks, func(i, j int) bool { return tracks[i].TrackID < tracks[j].TrackID })

	return &TrackingResult{
		Tracks:        tracks,
		Timestamp:     timestamp,
		FrameIdx:      frameIdx,
		NumDetections: numDetections,
		NumTrackers:   len(tracks),
	}
}

func (t *ByteTrack) Reset() {
	t.tracks = make(map[int]*TrackedTrack)
	t.nextTrackID = 0
	t.frameCount = 0
	t.lastTimestamp = 0
	t.hasLastTimestamp = false
}

func (t *ByteTrack) GetTrackCount() int {
	return len(t.tracks)
}
