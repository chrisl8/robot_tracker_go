package tracking

type ByteTrackConfig struct {
	TrackThresh float64
	TrackBuffer int
	MatchThresh float64
	FrameRate   int
	MinBoxArea  int
	MOT20       bool
}

type ByteTrack struct {
	config      *ByteTrackConfig
	tracks      map[int]*TrackedTrack
	nextTrackID int
	frameCount  int
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
			FrameRate:   30,
			MinBoxArea:  100,
			MOT20:       false,
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

	t.predictAllTracks()

	// Age all tracks — matched tracks get reset to 0 below
	for _, tt := range t.tracks {
		tt.timeSinceUpdate++
		tt.track.AgeTrack()
	}

	matchedDetections, unmatchedDetections, matchedTrackIDs := t.matchTracks(highConfDetections)

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

	// Match low-confidence detections
	lowMatched, _, lowMatchedTrackIDs := t.matchTracksLowConf(lowConfDetections)

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

func (t *ByteTrack) predictAllTracks() {
	for _, tt := range t.tracks {
		state := tt.kf.Predict()
		// Update track bbox with predicted position for matching
		w := tt.track.Bbox[2] - tt.track.Bbox[0]
		h := tt.track.Bbox[3] - tt.track.Bbox[1]
		tt.track.Bbox = centerToBbox(state[0], state[1], w, h)
	}
}

func (t *ByteTrack) matchTracks(detections []Detection) (matchedDetections []int, unmatchedDetections []int, matchedTrackIDs []int) {
	if len(detections) == 0 {
		return []int{}, []int{}, []int{}
	}

	activeTracks := make([]Track, 0, len(t.tracks))
	trackIDs := make([]int, 0, len(t.tracks))
	for trackID, tt := range t.tracks {
		activeTracks = append(activeTracks, *tt.track)
		trackIDs = append(trackIDs, trackID)
	}

	costMatrix := ComputeIoUCost(detections, activeTracks, t.config.MatchThresh)
	assignment := Hungarian(costMatrix)

	matchedDetectionsResult := make([]int, 0)
	matchedTrackIDsResult := make([]int, 0)
	unmatchedDetectionsMap := make(map[int]bool)

	for i := 0; i < len(detections); i++ {
		unmatchedDetectionsMap[i] = true
	}

	for i, j := range assignment.RowToCol {
		if j >= 0 && i < len(detections) && j < len(activeTracks) && costMatrix[i][j] < 0.5 {
			matchedDetectionsResult = append(matchedDetectionsResult, i)
			matchedTrackIDsResult = append(matchedTrackIDsResult, trackIDs[j])
			delete(unmatchedDetectionsMap, i)
		}
	}

	for idx := range unmatchedDetectionsMap {
		unmatchedDetections = append(unmatchedDetections, idx)
	}

	return matchedDetectionsResult, unmatchedDetections, matchedTrackIDsResult
}

func (t *ByteTrack) matchTracksLowConf(detections []Detection) (matched []int, unmatchedDetections []int, matchedTrackIDs []int) {
	if len(detections) == 0 {
		return []int{}, []int{}, []int{}
	}

	activeTracks := make([]Track, 0, len(t.tracks))
	trackIDs := make([]int, 0, len(t.tracks))
	for trackID, tt := range t.tracks {
		activeTracks = append(activeTracks, *tt.track)
		trackIDs = append(trackIDs, trackID)
	}

	if len(activeTracks) == 0 {
		for i := range detections {
			unmatchedDetections = append(unmatchedDetections, i)
		}
		return []int{}, unmatchedDetections, []int{}
	}

	costMatrix := ComputeIoUCost(detections, activeTracks, 0.3)
	assignment := Hungarian(costMatrix)

	matched = []int{}
	matchedTrackIDs = []int{}
	unmatchedDetectionsMap := make(map[int]bool)

	for i := 0; i < len(detections); i++ {
		unmatchedDetectionsMap[i] = true
	}

	for i, j := range assignment.RowToCol {
		if j >= 0 && i < len(detections) && j < len(activeTracks) && costMatrix[i][j] < 0.5 {
			matched = append(matched, i)
			matchedTrackIDs = append(matchedTrackIDs, trackIDs[j])
			delete(unmatchedDetectionsMap, i)
		}
	}

	for idx := range unmatchedDetectionsMap {
		unmatchedDetections = append(unmatchedDetections, idx)
	}

	return matched, unmatchedDetections, matchedTrackIDs
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
		tracks = append(tracks, *tt.track)
	}

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
}

func (t *ByteTrack) GetTrackCount() int {
	return len(t.tracks)
}
