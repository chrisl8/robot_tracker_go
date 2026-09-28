package detection

import (
	"bytes"
	"encoding/gob"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/chrisl8/robot_tracker_go/internal/utils"
)

// modelStateVersion guards against a future incompatible change to
// modelState misinterpreting an old file; bump it whenever modelState's
// meaning changes in a way an old decode wouldn't catch on its own.
const modelStateVersion = 1

// persistedModelState is the on-disk envelope around a modelState.
type persistedModelState struct {
	Version int
	SavedAt time.Time
	State   modelState
}

// saveModelState writes s to path. It writes to a temporary file in the same
// directory first and renames it into place, so a crash mid-write can never
// leave a torn file at path — the previous save (if any) survives untouched.
func saveModelState(path string, s modelState) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(persistedModelState{
		Version: modelStateVersion,
		SavedAt: time.Now(),
		State:   s,
	}); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// loadModelState reads a previously-saved state. A missing file, a corrupt
// file, or a version this build doesn't understand all return ok == false —
// never an error — so a caller can always fall back to warming up normally
// rather than failing to start.
func loadModelState(path string) (modelState, bool) {
	// #nosec G304 -- path is the background-save file name built by the app, not user input
	data, err := os.ReadFile(path)
	if err != nil {
		return modelState{}, false
	}
	var p persistedModelState
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&p); err != nil {
		utils.Debugf("foreground: discarding unreadable background save at %s: %v", path, err)
		return modelState{}, false
	}
	if p.Version != modelStateVersion {
		utils.Debugf("foreground: discarding background save at %s (version %d, want %d)", path, p.Version, modelStateVersion)
		return modelState{}, false
	}
	return p.State, true
}

// deleteModelState removes a previously-saved state, if any. It is
// best-effort: a missing file is not an error.
func deleteModelState(path string) {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		utils.Debugf("foreground: could not remove background save at %s: %v", path, err)
	}
}

// modelPersister owns saving the learned background to disk and restoring it
// across restarts, for ForegroundDetector. The zero value is disabled: every
// method is then a no-op (restore reports nothing to restore).
//
// Concurrency contract: enable must be called before any other method and
// before Process starts. maybeSave and restore are called from the Process
// goroutine only (lastSave is unsynchronized). saveNow and wait may be called
// from any goroutine — at shutdown saveNow can run while a periodic save is
// still in flight, so writeMu serializes every actual write to the file;
// inFlight only avoids scheduling a redundant goroutine while one is
// pending. wait lets Close outlast a background save.
type modelPersister struct {
	path     string
	interval time.Duration
	lastSave time.Time
	inFlight atomic.Bool
	writeMu  sync.Mutex     // serializes actual writes: see saveNow
	wg       sync.WaitGroup // lets wait block on an in-flight background save
}

// enable turns on saving to path every interval while the model is warm.
// path == "" leaves persistence off.
func (p *modelPersister) enable(path string, interval time.Duration) {
	p.path = path
	p.interval = interval
}

// saveNow immediately saves m if persistence is enabled and m is warm,
// blocking until the write completes. Intended for clean shutdown, where a
// fire-and-forget save (as maybeSave does) could be killed by process exit
// before it finishes.
func (p *modelPersister) saveNow(m *foregroundModel) {
	if p.path == "" {
		return
	}
	snap, ok := m.snapshot()
	if !ok {
		return
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	if err := saveModelState(p.path, snap); err != nil {
		utils.Debugf("foreground: could not save background to %s: %v", p.path, err)
	}
}

// maybeSave saves m in the background at most once per interval, while m is
// warm. snapshot() already copies the slices it returns, so handing them to a
// goroutine is safe.
func (p *modelPersister) maybeSave(now time.Time, m *foregroundModel) {
	if p.path == "" || p.interval <= 0 {
		return
	}
	if !p.lastSave.IsZero() && now.Sub(p.lastSave) < p.interval {
		return
	}
	snap, ok := m.snapshot()
	if !ok {
		return
	}
	p.lastSave = now
	if !p.inFlight.CompareAndSwap(false, true) {
		return
	}
	path := p.path
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.inFlight.Store(false)
		p.writeMu.Lock()
		defer p.writeMu.Unlock()
		if err := saveModelState(path, snap); err != nil {
			utils.Debugf("foreground: could not save background to %s: %v", path, err)
		}
	}()
}

// restore loads a previously-saved background into m (which the caller just
// created at working resolution sw x sh), marking it warm. A missing or
// unreadable save, or one whose size doesn't match sw x sh (a different
// camera, a different scale config), is left alone so m warms up normally,
// exactly as if nothing had been saved.
func (p *modelPersister) restore(m *foregroundModel, sw, sh int) {
	if p.path == "" {
		return
	}
	s, ok := loadModelState(p.path)
	if !ok {
		return
	}
	if s.W != sw || s.H != sh {
		utils.Debugf("foreground: ignoring background save at %s (%dx%d, working resolution is %dx%d)",
			p.path, s.W, s.H, sw, sh)
		return
	}
	if m.restore(s) {
		utils.Logf("foreground: restored background from %s, skipping warm-up", p.path)
	}
}

// clear deletes the saved background. Used when the operator resets the
// model: a stale save from before the reset must not silently undo that on
// the next restart.
func (p *modelPersister) clear() {
	if p.path != "" {
		deleteModelState(p.path)
	}
}

// wait blocks until any in-flight background save has finished.
func (p *modelPersister) wait() { p.wg.Wait() }
