package main

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// perfWindow accumulates per-frame timings so a summary can be logged every
// few seconds. Unlike the old one-frame FRAME TIMING sample, it sees every
// frame, so a slowdown (and its start and end) is visible afterwards.
type perfWindow struct {
	mu          sync.Mutex
	start       time.Time
	frames      int
	total       durStat
	detect      durStat
	plan        durStat
	cameraFails int
}

type durStat struct {
	sum, max time.Duration
}

func (d *durStat) add(v time.Duration) {
	d.sum += v
	if v > d.max {
		d.max = v
	}
}

func (d durStat) avg(n int) time.Duration {
	if n == 0 {
		return 0
	}
	return d.sum / time.Duration(n)
}

// Record adds one processed frame: total time, the detection share, and the
// tracking+planning share.
func (p *perfWindow) Record(total, detect, plan time.Duration) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.frames++
	p.total.add(total)
	p.detect.add(detect)
	p.plan.add(plan)
}

// RecordCameraFailure counts a failed camera read.
func (p *perfWindow) RecordCameraFailure() {
	p.mu.Lock()
	p.cameraFails++
	p.mu.Unlock()
}

type perfSummary struct {
	Window      time.Duration
	Frames      int
	FPS         float64
	TotalAvg    time.Duration
	TotalMax    time.Duration
	DetectAvg   time.Duration
	DetectMax   time.Duration
	PlanAvg     time.Duration
	PlanMax     time.Duration
	CameraFails int
}

// TakeSummary returns the stats since the last call and starts a new window.
func (p *perfWindow) TakeSummary(now time.Time) perfSummary {
	p.mu.Lock()
	defer p.mu.Unlock()

	window := now.Sub(p.start)
	s := perfSummary{
		Window:      window,
		Frames:      p.frames,
		TotalAvg:    p.total.avg(p.frames),
		TotalMax:    p.total.max,
		DetectAvg:   p.detect.avg(p.frames),
		DetectMax:   p.detect.max,
		PlanAvg:     p.plan.avg(p.frames),
		PlanMax:     p.plan.max,
		CameraFails: p.cameraFails,
	}
	if window > 0 {
		s.FPS = float64(p.frames) / window.Seconds()
	}

	p.start = now
	p.frames, p.cameraFails = 0, 0
	p.total, p.detect, p.plan = durStat{}, durStat{}, durStat{}
	return s
}

func ms(d time.Duration) int64 { return d.Milliseconds() }

// Line formats the summary as one log line (without extras).
func (s perfSummary) Line() string {
	return fmt.Sprintf("PERF fps=%.1f frames=%d frame=%d/%dms detect=%d/%dms plan=%d/%dms camfail=%d",
		s.FPS, s.Frames, ms(s.TotalAvg), ms(s.TotalMax), ms(s.DetectAvg), ms(s.DetectMax),
		ms(s.PlanAvg), ms(s.PlanMax), s.CameraFails)
}

// parseTopCPU parses `ps -Ao pid,%cpu,comm` output (any order) and returns the
// n busiest processes other than selfPID as "name(cpu%)", busiest first.
func parseTopCPU(output string, selfPID, n int) []string {
	type proc struct {
		name string
		cpu  float64
	}
	var procs []proc
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		pid, err := strconv.Atoi(fields[0])
		if err != nil || pid == selfPID {
			continue
		}
		cpu, err := strconv.ParseFloat(fields[1], 64)
		if err != nil {
			continue
		}
		name := strings.Join(fields[2:], " ")
		if i := strings.LastIndex(name, "/"); i >= 0 {
			name = name[i+1:]
		}
		procs = append(procs, proc{name: name, cpu: cpu})
	}
	sort.SliceStable(procs, func(a, b int) bool { return procs[a].cpu > procs[b].cpu })

	out := make([]string, 0, n)
	for _, p := range procs {
		if len(out) == n {
			break
		}
		out = append(out, fmt.Sprintf("%s(%.0f%%)", p.name, p.cpu))
	}
	return out
}
