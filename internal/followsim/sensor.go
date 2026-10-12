package followsim

import (
	"math"
	"math/rand"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
)

// Sensor is the camera: what the follower is told about the robot. Position
// has Gaussian error. Heading trails the true heading by HeadLag frames and
// has the noise recorded in controller/executor.go: a few degrees standing
// still, 5-15 degrees walking, and, while rotating, occasional 20-70 degree
// spikes (AprilTag corner confusion).
type Sensor struct {
	p       PlantParams
	rng     *rand.Rand
	history []float64 // true headings, newest last
	prev    float64
	hasPrev bool
}

func NewSensor(p PlantParams, rng *rand.Rand) *Sensor { return &Sensor{p: p, rng: rng} }

const deg = math.Pi / 180

// Measure returns the camera's view of pl this frame: position, heading, and
// the change in measured heading since the last frame (what the real loop
// calls headingDelta).
func (s *Sensor) Measure(pl *Plant) (pos [2]float64, heading, delta float64) {
	s.history = append(s.history, pl.Theta)
	lagged := s.history[max(0, len(s.history)-1-s.p.HeadLag)]

	var sigma float64
	spike := false
	switch pl.Applied {
	case controller.CommandStop:
		sigma = 1.5 * deg
	case controller.CommandForward, controller.CommandBackward:
		sigma = 7 * deg
	case controller.CommandLeft, controller.CommandRight:
		sigma = 8 * deg
		spike = s.rng.Float64() < 0.12 // #nosec G404 -- simulation, not security
	}
	noise := s.rng.NormFloat64() * sigma * s.p.HeadNoiseK
	if spike {
		mag := (20 + 50*s.rng.Float64()) * deg // #nosec G404 -- simulation, not security
		if s.rng.Intn(2) == 0 {                // #nosec G404 -- simulation, not security
			mag = -mag
		}
		noise += mag * s.p.HeadNoiseK
	}
	heading = normalize(lagged + noise)

	if s.hasPrev {
		delta = normalize(heading - s.prev)
	}
	s.prev, s.hasPrev = heading, true

	pos = [2]float64{
		pl.X + s.rng.NormFloat64()*s.p.PosNoise,
		pl.Y + s.rng.NormFloat64()*s.p.PosNoise,
	}
	return pos, heading, delta
}
