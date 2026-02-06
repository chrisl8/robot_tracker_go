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
			MatchThresh: 0.8,
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

	matchedDetections, matchedTrackIDs, unmatchedDetections := t.matchTracks(highConfDetections)

	for i, detIdx := range matchedDetections {
		trackID := matchedTrackIDs[i]
		tt := t.tracks[trackID]
		tt.track.Update(highConfDetections[detIdx].Bbox, timestamp, highConfDetections[detIdx].Confidence)
		tt.timeSinceUpdate = 0
		if tt.track.TagID == nil && highConfDetections[detIdx].TagID != nil {
			tagID := *highConfDetections[detIdx].TagID
			tt.track.TagID = &tagID
		}
	}

	for _, detIdx := range unmatchedDetections {
		t.createNewTrack(highConfDetections[detIdx], timestamp)
	}

	lowMatched, lowUnmatchedDetections, lowMatchedTrackIDs := t.matchTracksLowConf(lowConfDetections)

	for i, detIdx := range lowMatched {
		trackID := lowMatchedTrackIDs[i]
		tt := t.tracks[trackID]
		tt.track.Update(lowConfDetections[detIdx].Bbox, timestamp, lowConfDetections[detIdx].Confidence)
		tt.timeSinceUpdate = 0
	}

	for _, detIdx := range lowUnmatchedDetections {
		t.createNewTrack(lowConfDetections[detIdx], timestamp)
	}

	for trackID := range t.tracks {
		t.tracks[trackID].timeSinceUpdate++
	}

	t.removeLostTracks()

	return t.buildTrackingResult(timestamp, frameIdx, len(detections))
}

func (t *ByteTrack) predictAllTracks() {
	for _, tt := range t.tracks {
		tt.kf.Predict()
	}
}

func (t *ByteTrack) matchTracks(detections []Detection) (matchedDetectionsResult []int, unmatchedDetections []int, unmatchedTracksResult []int) {
	if len(detections) == 0 {
		for trackID := range t.tracks {
			unmatchedTracksResult = append(unmatchedTracksResult, trackID)
		}
		return
	}

	activeTracks := make([]Track, 0, len(t.tracks))
	trackIDs := make([]int, 0, len(t.tracks))
	for trackID, tt := range t.tracks {
		if tt.timeSinceUpdate == 0 {
			activeTracks = append(activeTracks, *tt.track)
			trackIDs = append(trackIDs, trackID)
		}
	}

	costMatrix := ComputeIoUCost(detections, activeTracks, t.config.MatchThresh)
	assignment := Hungarian(costMatrix)

	matchedDetections := make([]int, 0)
	matchedTrackIDs := make([]int, 0)
	unmatchedDetectionsMap := make(map[int]bool)
	unmatchedTracksMap := make(map[int]bool)

	for i := 0; i < len(detections); i++ {
		unmatchedDetectionsMap[i] = true
	}
	for i := 0; i < len(activeTracks); i++ {
		unmatchedTracksMap[i] = true
	}

	for i, j := range assignment.RowToCol {
		if i < len(detections) && j < len(activeTracks) && costMatrix[i][j] < 0.5 {
			matchedDetections = append(matchedDetections, i)
			matchedTrackIDs = append(matchedTrackIDs, trackIDs[j])
			delete(unmatchedDetectionsMap, i)
			delete(unmatchedTracksMap, j)
		}
	}

	for idx := range unmatchedDetectionsMap {
		unmatchedDetections = append(unmatchedDetections, idx)
	}
	for idx := range unmatchedTracksMap {
		unmatchedTracksResult = append(unmatchedTracksResult, trackIDs[idx])
	}

	return matchedDetections, unmatchedDetections, unmatchedTracksResult
}

func (t *ByteTrack) matchTracksLowConf(detections []Detection) (matched []int, unmatchedDetections []int, matchedTrackIDs []int) {
	if len(detections) == 0 {
		return
	}

	activeTracks := make([]Track, 0, len(t.tracks))
	trackIDs := make([]int, 0, len(t.tracks))
	for trackID, tt := range t.tracks {
		if tt.timeSinceUpdate == 0 {
			activeTracks = append(activeTracks, *tt.track)
			trackIDs = append(trackIDs, trackID)
		}
	}

	if len(activeTracks) == 0 {
		for i := range detections {
			unmatchedDetections = append(unmatchedDetections, i)
		}
		return
	}

	costMatrix := ComputeIoUCost(detections, activeTracks, 0.3)
	assignment := Hungarian(costMatrix)

	matchedDetections := make([]int, 0)
	unmatchedDetectionsMap := make(map[int]bool)

	for i := 0; i < len(detections); i++ {
		unmatchedDetectionsMap[i] = true
	}

	for i, j := range assignment.RowToCol {
		if i < len(detections) && j < len(activeTracks) && costMatrix[i][j] < 0.5 {
			matchedDetections = append(matchedDetections, i)
			matchedTrackIDs = append(matchedTrackIDs, trackIDs[j])
			delete(unmatchedDetectionsMap, i)
		}
	}

	for idx := range unmatchedDetectionsMap {
		unmatchedDetections = append(unmatchedDetections, idx)
	}

	return matchedDetections, unmatchedDetections, matchedTrackIDs
}

func (t *ByteTrack) getMatchedTrackID(matchIdx int) int {
	return matchIdx
}

func (t *ByteTrack) getMatchedTrackIDFromLowConf(matchIdx int) int {
	return matchIdx
}

func (t *ByteTrack) createNewTrack(detection Detection, timestamp float64) {
	track := NewTrack(t.nextTrackID, detection.Bbox, timestamp, detection.Confidence)
	t.nextTrackID++

	kf := NewKalmanFilter()
	cx, cy := bboxToCenter(detection.Bbox)
	kf.Initialize(float64(detection.Bbox[0]), float64(detection.Bbox[1]), cx, cy)

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
