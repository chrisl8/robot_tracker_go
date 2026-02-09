package tracking

type TrackState int

const (
	TrackStateTentative TrackState = iota
	TrackStateConfirmed
	TrackStateLost
)

type Track struct {
	TrackID     int
	Bbox        [4]int
	Timestamp   float64
	Confidence  float64
	TagID       *int
	ClassID     *int
	Age         int
	Hits        int
	State       TrackState
	History     []TrackHistoryPoint
	WorldPos    [2]float64
	PixelRadius float64
}

type TrackHistoryPoint struct {
	Timestamp float64
	Bbox      [4]int
}

type TrackingResult struct {
	Tracks        []Track
	Timestamp     float64
	FrameIdx      int
	NumDetections int
	NumTrackers   int
}

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

func (t *Track) Update(bbox [4]int, timestamp, confidence float64) {
	t.Bbox = bbox
	t.Timestamp = timestamp
	t.Confidence = confidence
	t.Hits++
	t.Age++

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

func (t *Track) AgeTrack() {
	t.Age++
}

type Detection struct {
	Bbox       [4]int
	Confidence float64
	ClassID    int
	TagID      *int
}

type Tracker interface {
	Update(detections []Detection, timestamp float64, frameIdx int) *TrackingResult
	Reset()
}
