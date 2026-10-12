//go:build gocv

package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

const (
	// perfLogEveryTicks is how many watchdog ticks (seconds) between PERF lines.
	perfLogEveryTicks = 5
	// lowFPSForOffenders is the frame rate under which a PERF line also names
	// the busiest other processes.
	lowFPSForOffenders = 8.0
	offenderLogEvery   = 30 * time.Second
)

// logPerf writes one PERF line: frame rate and timings over the last window,
// this process's CPU use, machine load, memory, and control state. When the
// frame rate is low it also names the busiest other processes (rate-limited),
// so a slowdown caused by something else on the machine identifies itself.
func (rs *RobotSystem) logPerf() {
	s := rs.stats.perf.TakeSummary(time.Now())

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	line := s.Line() + fmt.Sprintf(" mem=%.0fMB mode=%s", float64(mem.Alloc)/1024/1024, rs.GetControlMode())

	now := time.Now()
	cpu := processCPUSeconds()
	if wall := now.Sub(rs.stats.lastCPUAt).Seconds(); wall > 0 && rs.stats.lastCPUSeconds > 0 {
		line += fmt.Sprintf(" proc_cpu=%.1fcores", (cpu-rs.stats.lastCPUSeconds)/wall)
	}
	rs.stats.lastCPUSeconds, rs.stats.lastCPUAt = cpu, now

	if rs.detection.fg != nil && rs.detection.fg.enabled.Load() {
		line += rs.detection.fg.perfSummary()
	}
	if load, ok := systemLoadAverage(); ok {
		line += fmt.Sprintf(" load1=%.2f", load)
	}
	if s.FPS < lowFPSForOffenders && time.Since(rs.stats.lastOffenderLog) > offenderLogEvery {
		if top := topCPUProcesses(4); len(top) > 0 {
			line += " LOW_FPS busiest_processes=" + strings.Join(top, ",")
			rs.stats.lastOffenderLog = now
		}
	}
	utils.Log(line)
}

// frameStallThreshold is how long without a processed frame counts as a
// stalled camera worth telling the UI about.
const frameStallThreshold = 2 * time.Second

const (
	// cameraDeadRestartAfter is how long a camera that has been delivering
	// frames may then go without one before the service restarts itself. It
	// outlasts the camera's own reopen attempts (each can block ~45 s on the
	// dead device), because on macOS a camera that was unplugged never delivers
	// frames to the same process again, even once it is plugged back in.
	cameraDeadRestartAfter = 60 * time.Second
	launchdLabel           = "com.chrisl8.robot-tracker"
)

// restartService asks launchd to restart this service (SIGTERM to us, which stops
// the robot cleanly, then a fresh start). A var so tests can replace it.
var restartService = func() error {
	// #nosec G204 -- fixed arguments: a constant label and our own uid
	return exec.Command("launchctl", "kickstart", "-k",
		fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)).Run()
}

// restartIfCameraDead restarts the service when frames, which did flow before,
// have been missing for cameraDeadRestartAfter. It reports whether it asked for
// a restart. A camera that never delivered a frame is left alone, so a missing
// camera permission cannot put the service into a restart loop. Outside launchd
// (a hand-run process) the restart fails and the log says so.
func (rs *RobotSystem) restartIfCameraDead(age time.Duration) bool {
	if age <= cameraDeadRestartAfter || rs.stats.lastFrameNanos.Load() == 0 {
		return false
	}
	utils.Logf("Camera has delivered no frames for %.0fs and reopening did not help: restarting the service", age.Seconds())
	if err := restartService(); err != nil {
		utils.Logf("Could not restart the service (not running under launchd?): %v; restart it by hand", err)
	}
	return true
}

// startFrameWatchdog broadcasts status once a second while no frames are being
// processed. The normal status update rides on the frame loop, so without this
// a dead camera would leave the UI silently showing the last good FPS.
func (rs *RobotSystem) startFrameWatchdog() {
	rs.stats.watchdogOnce.Do(func() {
		rs.stats.watchdogStop = make(chan struct{})
		stop := rs.stats.watchdogStop
		rs.stats.perf.TakeSummary(time.Now()) // start the first window now
		rs.stats.lastCPUAt = time.Now()
		rs.stats.lastCPUSeconds = processCPUSeconds()
		go func() {
			ticker := time.NewTicker(time.Second)
			defer ticker.Stop()
			ticks := 0
			stallHalted := false
			var lastRestartAsk time.Time
			for {
				select {
				case <-stop:
					return
				case <-ticker.C:
					ticks++
					if ticks%perfLogEveryTicks == 0 {
						rs.logPerf()
					}
					last := rs.stats.startTime
					if n := rs.stats.lastFrameNanos.Load(); n != 0 {
						last = time.Unix(0, n)
					}
					age := time.Since(last)
					if age > frameStallThreshold && rs.web.webServer != nil {
						rs.web.webServer.BroadcastCameraStalled(time.Since(rs.stats.startTime).Seconds(), age.Seconds())
						// The frame loop publishes the motion status, and it is
						// the thing that has stopped.
						rs.web.webServer.SetMotionStatus(computeMotion(motionInputs{cameraStalled: true, arduinoUp: true}))
					}
					// Autonomous control only runs from the frame loop, so with no
					// frames nothing would ever stop the robot. Halt it once per
					// stall (not every tick, so it can be driven again by hand).
					if age > frameStallThreshold && !stallHalted {
						stallHalted = true
						utils.Logf("Camera stalled for %.1fs: halting robot", age.Seconds())
						if rs.io.commandQueue != nil {
							rs.io.commandQueue.HaltMotion()
						}
					} else if age <= frameStallThreshold {
						stallHalted = false
					}
					// Give launchd time to stop us; do not ask again every tick.
					if time.Since(lastRestartAsk) > 30*time.Second && rs.restartIfCameraDead(age) {
						lastRestartAsk = time.Now()
					}
				}
			}
		}()
	})
}
