package tracking

// TrackState is where a track is in its life: tentative, confirmed or lost.
type TrackState int

const (
	// TrackStateTentative is a new track that has not matched enough detections yet.
	TrackStateTentative TrackState = iota
	// TrackStateConfirmed is a track that has matched at least three detections and is trusted.
	TrackStateConfirmed
	// TrackStateLost is a confirmed track that has gone unmatched for some frames; the tracker keeps it for a while in case it reappears.
	TrackStateLost
)

// Track is one tracked object: its pixel box and tag, its age and hit count, a short history, and, filled in later by the position stage, its world position and heading.
type Track struct {
	TrackID       int
	Bbox          [4]int
	Timestamp     float64
	Confidence    float64
	TagID         *int
	ClassID       *int
	Age           int
	Hits          int
	State         TrackState
	History       []TrackHistoryPoint
	WorldPos      [2]float64
	PixelRadius   float64
	Heading       float64
	HeadingOffset float64
	Corners       [4][2]float64
}

// TrackHistoryPoint is one past position of a track.
type TrackHistoryPoint struct {
	Timestamp float64
	Bbox      [4]int
}

// TrackingResult is the tracker's output for one frame.
type TrackingResult struct {
	Tracks        []Track
	Timestamp     float64
	FrameIdx      int
	NumDetections int
	NumTrackers   int
}

// NewTrack returns a tentative track with its first detection.
func NewTrack(trackID int, bbox [4]int, timestamp, confidence float64) *Track {
	return &Track{
		TrackID:    trackID,
		Bbox:       bbox,
		Timestamp:  timestamp,
		Confidence: confidence,
		Age:        1,
		Hits:       1,
		State:      TrackStateTentative,
		History:    make([]TrackHistoryPoint, 0),
	}
}

// ToDict returns the track as a map for JSON, with at most the last 30 history points.
func (t *Track) ToDict() map[string]interface{} {
	history := t.History
	if len(history) > 30 {
		history = history[len(history)-30:]
	}

	historyList := make([]map[string]interface{}, len(history))
	for i, hp := range history {
		historyList[i] = map[string]interface{}{
			"timestamp": hp.Timestamp,
			"bbox":      hp.Bbox,
		}
	}

	result := map[string]interface{}{
		"track_id":   t.TrackID,
		"bbox":       t.Bbox,
		"timestamp":  t.Timestamp,
		"confidence": t.Confidence,
		"tag_id":     t.TagID,
		"class_id":   t.ClassID,
		"age":        t.Age,
		"hits":       t.Hits,
		"state":      t.StateString(),
		"history":    historyList,
	}

	if t.PixelRadius > 0 {
		result["pixel_radius"] = t.PixelRadius
	}

	return result
}

// StateString returns the track state as text: "tentative", "confirmed" or "lost".
func (t *Track) StateString() string {
	switch t.State {
	case TrackStateTentative:
		return "tentative"
	case TrackStateConfirmed:
		return "confirmed"
	case TrackStateLost:
		return "lost"
	default:
		return "unknown"
	}
}

// Update records a matched detection: it replaces the box, counts a hit, extends the history (capped at 100 points) and confirms the track after three hits.
func (t *Track) Update(bbox [4]int, timestamp, confidence float64) {
	t.Bbox = bbox
	t.Timestamp = timestamp
	t.Confidence = confidence
	t.Hits++

	t.History = append(t.History, TrackHistoryPoint{
		Timestamp: timestamp,
		Bbox:      bbox,
	})

	if len(t.History) > 100 {
		t.History = t.History[len(t.History)-100:]
	}

	if t.Hits >= 3 {
		t.State = TrackStateConfirmed
	}
}

// AgeTrack advances Age by one frame. The tracker calls it once per track per
// frame (matched or not), so Age is frames since creation. Update deliberately
// does not age the track as well, or matched tracks would age twice per frame.
func (t *Track) AgeTrack() {
	t.Age++
}

// Detection is one thing seen in a frame, as the tracker takes it: a pixel box, a confidence and the tag it belongs to.
type Detection struct {
	Bbox       [4]int
	Confidence float64
	ClassID    int
	TagID      *int
	Corners    [4][2]float64
}

// Tracker follows detections over frames. ByteTrack implements it.
type Tracker interface {
	Update(detections []Detection, timestamp float64, frameIdx int) *TrackingResult
	Reset()
}
