package followsim

import (
	"math"
	"math/rand"

	"github.com/chrisl8/robot_tracker_go/internal/controller"
)

// FrameRate is the simulated camera/control rate, frames per second (the real
// loop runs at about 14.7).
const FrameRate = 15.0

const dt = 1.0 / FrameRate

// Regime describes stick-and-lurch for one kind of motion: the robot sits in
// a regime (stalled, slipping, or gripping and racing) for a while, then draws
// a new one. A regime is a multiplier on the nominal rate.
type Regime struct {
	PStall, PSlip    float64 // probability a new regime is stalled / slipping
	GripMin, GripMax float64 // multiplier range when gripping
	LeavePerFrame    float64 // probability per frame of drawing a new regime
}

func (r Regime) draw(rng *rand.Rand) float64 { // #nosec G404 -- simulation, not security
	u := rng.Float64()
	switch {
	case u < r.PStall:
		return 0.05 * rng.Float64()
	case u < r.PStall+r.PSlip:
		return 0.15 + 0.25*rng.Float64()
	default:
		return r.GripMin + (r.GripMax-r.GripMin)*rng.Float64()
	}
}

// PlantParams is one run's robot-and-floor: everything the follower must not
// be tuned to.
type PlantParams struct {
	Speed      float64 // nominal walking speed, m/s
	TurnRate   float64 // nominal turn rate, rad/s
	TurnAsym   float64 // right-turn rate divided by left-turn rate
	BackFactor float64 // reverse speed relative to forward
	Veer       float64 // rad: travel direction minus heading, a bias
	VeerWalk   float64 // rad/frame: std of the veer's random walk
	CmdLatency int     // frames between a command and the robot acting on it

	Walk, Turn Regime

	PosNoise   float64 // m: std of the camera's position error
	HeadLag    int     // frames the camera's heading trails the true one
	HeadNoiseK float64 // multiplier on the measured heading-noise levels
}

// IdealParams is a well-behaved robot, for checking that the simulator itself
// does what commands say. It is not a plant any follower should be judged on.
func IdealParams() PlantParams {
	steady := Regime{GripMin: 1, GripMax: 1}
	return PlantParams{
		Speed: 0.10, TurnRate: 1.5, TurnAsym: 1, BackFactor: 1,
		CmdLatency: 1, Walk: steady, Turn: steady,
		PosNoise: 0, HeadLag: 0, HeadNoiseK: 0,
	}
}

func logUniform(rng *rand.Rand, lo, hi float64) float64 { // #nosec G404 -- simulation, not security
	return math.Exp(math.Log(lo) + rng.Float64()*(math.Log(hi)-math.Log(lo)))
}

func uniform(rng *rand.Rand, lo, hi float64) float64 { // #nosec G404 -- simulation, not security
	return lo + rng.Float64()*(hi-lo)
}

// SampleParams draws a run's plant. The ranges are guesses made deliberately
// wide: nominal speed and turn rate span a factor of six or more, and a weak
// battery hurts turning more than walking. The heading noise levels the
// follower sees come from the measurements recorded in controller/executor.go.
func SampleParams(rng *rand.Rand) PlantParams { // #nosec G404 -- simulation, not security
	battery := uniform(rng, 0.5, 1.0)
	friction := func() Regime {
		return Regime{
			PStall: uniform(rng, 0, 0.30), PSlip: uniform(rng, 0.05, 0.35),
			GripMin: 1.0, GripMax: 2.0,
			LeavePerFrame: uniform(rng, 0.08, 0.30),
		}
	}
	return PlantParams{
		Speed:      logUniform(rng, 0.03, 0.18) * battery,
		TurnRate:   logUniform(rng, 0.3, 2.2) * math.Pow(battery, 1.5),
		TurnAsym:   uniform(rng, 0.7, 1.4),
		BackFactor: uniform(rng, 0.5, 1.2),
		Veer:       rng.NormFloat64() * 6 * math.Pi / 180,
		VeerWalk:   uniform(rng, 0, 1.5) * math.Pi / 180,
		CmdLatency: 1 + rng.Intn(4),
		Walk:       friction(),
		Turn:       friction(),
		PosNoise:   uniform(rng, 0.005, 0.03),
		HeadLag:    1 + rng.Intn(3),
		HeadNoiseK: uniform(rng, 0.6, 1.5),
	}
}

// Plant is the true robot: where it really is. Followers never see it
// directly, only through a Sensor.
type Plant struct {
	X, Y, Theta float64 // theta is the way it faces; Right turns increase it

	p         PlantParams
	rng       *rand.Rand
	walkMult  float64
	turnMult  float64
	veer      float64
	pending   []controller.Command // commands in flight, oldest first
	Applied   controller.Command   // the command acting this frame
	Travelled float64
}

// NewPlant puts a robot at (x, y) facing theta.
func NewPlant(p PlantParams, rng *rand.Rand, x, y, theta float64) *Plant {
	pl := &Plant{X: x, Y: y, Theta: theta, p: p, rng: rng, veer: p.Veer, Applied: controller.CommandStop}
	pl.walkMult = p.Walk.draw(rng)
	pl.turnMult = p.Turn.draw(rng)
	pl.pending = make([]controller.Command, p.CmdLatency)
	for i := range pl.pending {
		pl.pending[i] = controller.CommandStop
	}
	return pl
}

// Step issues cmd and advances the world one frame. The command acts
// CmdLatency frames later.
func (pl *Plant) Step(cmd controller.Command) {
	pl.pending = append(pl.pending, cmd)
	pl.Applied = pl.pending[0]
	pl.pending = pl.pending[1:]

	if pl.rng.Float64() < pl.p.Walk.LeavePerFrame {
		pl.walkMult = pl.p.Walk.draw(pl.rng)
	}
	if pl.rng.Float64() < pl.p.Turn.LeavePerFrame {
		pl.turnMult = pl.p.Turn.draw(pl.rng)
	}

	switch pl.Applied {
	case controller.CommandForward, controller.CommandBackward:
		pl.veer += pl.rng.NormFloat64() * pl.p.VeerWalk
		pl.veer = math.Max(-15*math.Pi/180, math.Min(15*math.Pi/180, pl.veer))
		dist := pl.p.Speed * pl.walkMult * dt
		dir := pl.Theta + pl.veer
		if pl.Applied == controller.CommandBackward {
			dist *= pl.p.BackFactor
			dir += math.Pi
		}
		pl.X += dist * math.Cos(dir)
		pl.Y += dist * math.Sin(dir)
		pl.Travelled += dist
	case controller.CommandRight:
		pl.Theta += pl.p.TurnRate * math.Sqrt(pl.p.TurnAsym) * pl.turnMult * dt
	case controller.CommandLeft:
		pl.Theta -= pl.p.TurnRate / math.Sqrt(pl.p.TurnAsym) * pl.turnMult * dt
	case controller.CommandStop:
	}
	pl.Theta = normalize(pl.Theta)
}

func normalize(a float64) float64 {
	for a > math.Pi {
		a -= 2 * math.Pi
	}
	for a < -math.Pi {
		a += 2 * math.Pi
	}
	return a
}
