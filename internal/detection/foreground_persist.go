package detection

import (
	"bytes"
	"encoding/gob"
	"os"
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
