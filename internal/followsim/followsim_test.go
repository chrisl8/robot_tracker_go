package followsim

import (
	"math"
	"math/rand"
	"os"
	"strconv"
	"testing"

	"github.com/chrisl8/robot_tracker_go/internal/config"
	"github.com/chrisl8/robot_tracker_go/internal/controller"
	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

func TestMain(m *testing.M) {
	utils.SetQuietMode(true) // the planner logs every replan and waypoint skip
	os.Exit(m.Run())
}

func idealPlant(theta float64) *Plant {
	return NewPlant(IdealParams(), rand.New(rand.NewSource(1)), 0, 0, theta) // #nosec G404 -- simulation, not security
}

// The simulator's own physics: each command does what it says. Without this a
// follower could look good against a plant that is simply wrong.
func TestPlant_CommandsDoWhatTheySay(t *testing.T) {
	tests := []struct {
		name   string
		cmd    controller.Command
		theta  float64
		wantDX float64 // sign of x movement per frame (0 = none)
		wantDY float64
		wantDT float64 // sign of heading change
	}{
		{"forward along heading 0", controller.CommandForward, 0, 1, 0, 0},
		{"forward along heading 90deg (down the image)", controller.CommandForward, math.Pi / 2, 0, 1, 0},
		{"backward goes the other way", controller.CommandBackward, 0, -1, 0, 0},
		{"right increases heading", controller.CommandRight, 0, 0, 0, 1},
		{"left decreases heading", controller.CommandLeft, 0, 0, 0, -1},
		{"stop does nothing", controller.CommandStop, 0, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pl := idealPlant(tt.theta)
			pl.Step(tt.cmd) // latency of one frame: this is the frame it is still in flight
			pl.Step(tt.cmd)
			sign := func(v float64) float64 {
				if math.Abs(v) < 1e-9 {
					return 0
				}
				return math.Copysign(1, v)
			}
			if sign(pl.X) != tt.wantDX || sign(pl.Y) != tt.wantDY || sign(normalize(pl.Theta-tt.theta)) != tt.wantDT {
				t.Errorf("after %v: moved (%.4f, %.4f), turned %.4f; want signs (%v, %v, %v)",
					tt.cmd, pl.X, pl.Y, normalize(pl.Theta-tt.theta), tt.wantDX, tt.wantDY, tt.wantDT)
			}
		})
	}
}

func TestPlant_CommandLatency(t *testing.T) {
	p := IdealParams()
	p.CmdLatency = 3
	pl := NewPlant(p, rand.New(rand.NewSource(1)), 0, 0, 0) // #nosec G404 -- simulation, not security
	for i := 1; i <= 5; i++ {
		pl.Step(controller.CommandForward)
		moved := pl.X > 0
		if want := i > 3; moved != want {
			t.Errorf("after %d frames moved=%v, want %v (latency 3)", i, moved, want)
		}
	}
}

// A plant that stalls forever must not be reported as having arrived: the
// harness has to be able to say "failed".
func TestRun_ReportsFailureForARobotThatCannotWalk(t *testing.T) {
	p := IdealParams()
	p.Walk = Regime{PStall: 1, GripMin: 1, GripMax: 1}
	scene := Scenes()[2]
	res := RunWith(scene, NewBaseline, DefaultExecutorSettings(), p, rand.New(rand.NewSource(3)), 3) // #nosec G404 -- simulation, not security
	if res.Reached || res.Seconds < MaxSeconds-1 {
		t.Errorf("a robot that cannot walk: reached=%v after %.0fs, want a timeout failure", res.Reached, res.Seconds)
	}
}

func TestSampleParams_CoverAWideRange(t *testing.T) {
	rng := rand.New(rand.NewSource(7)) // #nosec G404 -- simulation, not security
	minS, maxS := math.Inf(1), 0.0
	minT, maxT := math.Inf(1), 0.0
	latencies := map[int]bool{}
	for i := 0; i < 500; i++ {
		p := SampleParams(rng)
		minS, maxS = math.Min(minS, p.Speed), math.Max(maxS, p.Speed)
		minT, maxT = math.Min(minT, p.TurnRate), math.Max(maxT, p.TurnRate)
		latencies[p.CmdLatency] = true
	}
	if maxS/minS < 6 || maxT/minT < 8 || len(latencies) < 4 {
		t.Errorf("plants are too alike: speed %.3f-%.3f, turn %.2f-%.2f rad/s, %d latencies; the point is a wide spread",
			minS, maxS, minT, maxT, len(latencies))
	}
}

func TestRun_IsDeterministicPerSeed(t *testing.T) {
	scene := Scenes()[2]
	a := Run(scene, NewBaseline, DefaultExecutorSettings(), 11)
	b := Run(scene, NewBaseline, DefaultExecutorSettings(), 11)
	c := Run(scene, NewBaseline, DefaultExecutorSettings(), 12)
	if a != b {
		t.Errorf("the same seed gave different runs:\n%+v\n%+v", a, b)
	}
	if a.Params == c.Params {
		t.Error("different seeds gave the same plant")
	}
}

// The simulator's executor settings must be the real ones.
func TestDefaultExecutorSettingsMatchConfig(t *testing.T) {
	cfg, err := config.Load("../../config/tracking_config.yaml")
	if err != nil {
		t.Skipf("config not loadable here: %v", err)
	}
	pe, want := cfg.PathExecution, DefaultExecutorSettings()
	got := ExecutorSettings{pe.WaypointThreshold, pe.SpinThresholdDeg, pe.BurstFrames, pe.MaxWaitFrames, pe.ForwardThresholdDeg}
	if got != want {
		t.Errorf("config path_execution = %+v, simulator defaults = %+v: update DefaultExecutorSettings", got, want)
	}
}

// With a well-behaved robot the baseline follower gets round every scene, so a
// failure in the randomized runs is the plant's doing and not the harness's.
func TestBaseline_ReachesTheGoalWithAnIdealRobot(t *testing.T) {
	for _, scene := range Scenes() {
		t.Run(scene.Name, func(t *testing.T) {
			for seed := int64(1); seed <= 5; seed++ {
				res := RunWith(scene, NewBaseline, DefaultExecutorSettings(), IdealParams(), rand.New(rand.NewSource(seed)), seed) // #nosec G404 -- simulation, not security
				if !res.Reached || res.Collided || res.LeftFrame {
					t.Errorf("seed %d: reached=%v collided=%v leftFrame=%v finalDist=%.2f after %.0fs", seed, res.Reached, res.Collided, res.LeftFrame, res.FinalDist, res.Seconds)
				}
			}
		})
	}
}

func TestSummarize(t *testing.T) {
	rs := []Result{
		{Reached: true, Seconds: 10, MinClearance: 0.1, MaxCrossTrack: 0.02, Travelled: 1, CmdSwitches: 4},
		{Reached: true, Seconds: 20, MinClearance: 0.2, MaxCrossTrack: 0.04, Travelled: 1, CmdSwitches: 6, Replans: 2},
		{Reached: false, Collided: true, MinClearance: -0.1, MaxCrossTrack: 0.2, Travelled: 2, CmdSwitches: 2},
		{PlanFailed: true},
	}
	s := Summarize(rs)
	if s.Runs != 4 || s.ReachedPct != 50 || s.CollidedPct != 25 || s.PlanFailedPct != 25 {
		t.Errorf("percentages wrong: %+v", s)
	}
	if s.SecondsP50 != 10 || s.SecondsP90 != 20 || s.ClearanceP05 != -0.1 {
		t.Errorf("percentiles wrong: %+v", s)
	}
	if math.Abs(s.SwitchesPerMetre-3) > 1e-9 || math.Abs(s.ReplansMean-2.0/3) > 1e-9 {
		t.Errorf("rates wrong: switches/m=%v replans=%v", s.SwitchesPerMetre, s.ReplansMean)
	}
	if got := Summarize(nil); got.Runs != 0 {
		t.Errorf("empty summary: %+v", got)
	}
}

// The simulator is only worth trusting if it reproduces what was seen on the
// real robot with the real follower: the planner keeps a path 0.12 m from
// obstacles (gap between robot body and obstacle) but early arrival at bends
// cuts the corner, so the robot gets visibly closer than planned and trips the
// proximity backstop over and over. If this stops holding, the plant has
// become too kind to be a useful test.
func TestBaseline_ReproducesTheCornerCuttingSeenOnTheRobot(t *testing.T) {
	s := Summarize(RunMany(Scenes()[2], NewBaseline, DefaultExecutorSettings(), 40, 1000))
	const plannedGap = 0.12
	if s.ClearanceP50 > plannedGap-0.04 {
		t.Errorf("median closest approach %.3f m: the robot barely cuts corners in the simulator (planned gap %.2f m)", s.ClearanceP50, plannedGap)
	}
	if s.ReplansMean < 1 {
		t.Errorf("%.2f proximity replans per run: the simulator never makes the robot back away", s.ReplansMean)
	}
}

// The carrot follower has to earn its place: across the same sequence of
// random robots it must be no worse than the waypoint follower on anything we
// measure, and the point of it - corners no longer cut into obstacles - must
// show: no more collisions, a better worst-case gap, fewer proximity replans.
func TestCarrot_IsNoWorseThanBaselineAndKeepsFurtherFromObstacles(t *testing.T) {
	for _, scene := range Scenes() {
		t.Run(scene.Name, func(t *testing.T) {
			base := Summarize(RunMany(scene, NewBaseline, DefaultExecutorSettings(), 50, 1000))
			carrot := Summarize(RunMany(scene, NewCarrot, DefaultExecutorSettings(), 50, 1000))
			t.Logf("baseline %s", base)
			t.Logf("carrot   %s", carrot)
			if carrot.ReachedPct < base.ReachedPct-5 {
				t.Errorf("reaches the goal in %.0f%% of runs, baseline %.0f%%", carrot.ReachedPct, base.ReachedPct)
			}
			if carrot.CollidedPct > base.CollidedPct || carrot.LeftFramePct > base.LeftFramePct {
				t.Errorf("collides %.0f%% / leaves the frame %.0f%%, baseline %.0f%% / %.0f%%", carrot.CollidedPct, carrot.LeftFramePct, base.CollidedPct, base.LeftFramePct)
			}
			if carrot.ClearanceP05 < base.ClearanceP05+0.01 {
				t.Errorf("worst-case gap to obstacles %.3f m is not better than the baseline's %.3f m", carrot.ClearanceP05, base.ClearanceP05)
			}
			if carrot.ClearanceP50 < base.ClearanceP50+0.01 {
				t.Errorf("median closest approach %.3f m is not better than the baseline's %.3f m", carrot.ClearanceP50, base.ClearanceP50)
			}
			if carrot.ReplansMean > base.ReplansMean {
				t.Errorf("%.2f proximity replans per run, baseline %.2f", carrot.ReplansMean, base.ReplansMean)
			}
			if carrot.SecondsP50 > base.SecondsP50*1.15 {
				t.Errorf("median time %.0fs, baseline %.0fs: it is slower", carrot.SecondsP50, base.SecondsP50)
			}
		})
	}
}

// TestReport prints how each follower does over many random robots, for each
// scene, on the same sequence of robots: `go test ./internal/followsim -run
// TestReport -v`. FOLLOWSIM_RUNS sets the number of runs per scene (default 40).
func TestReport(t *testing.T) {
	n := 40
	if v, err := strconv.Atoi(os.Getenv("FOLLOWSIM_RUNS")); err == nil && v > 0 {
		n = v
	}
	for _, scene := range Scenes() {
		for _, f := range []struct {
			name    string
			factory FollowerFactory
		}{{"baseline", NewBaseline}, {"carrot  ", NewCarrot}} {
			s := Summarize(RunMany(scene, f.factory, DefaultExecutorSettings(), n, 1000))
			t.Logf("%-22s %s %s", scene.Name, f.name, s)
		}
	}
}
