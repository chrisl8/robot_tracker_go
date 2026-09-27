package detection

import (
	"bytes"
	"encoding/gob"
	"os"
	"path/filepath"
	"testing"
)

func testState() modelState {
	return modelState{
		W: 4, H: 2,
		Bg:    []float32{1, 2, 3, 4, 5, 6, 7, 8},
		Known: []bool{true, false, true, true, false, true, true, false},
		Gain:  1.23,
	}
}

func statesEqual(a, b modelState) bool {
	if a.W != b.W || a.H != b.H || a.Gain != b.Gain {
		return false
	}
	if len(a.Bg) != len(b.Bg) || len(a.Known) != len(b.Known) {
		return false
	}
	for i := range a.Bg {
		if a.Bg[i] != b.Bg[i] {
			return false
		}
	}
	for i := range a.Known {
		if a.Known[i] != b.Known[i] {
			return false
		}
	}
	return true
}

func TestSaveLoadModelState_RoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	want := testState()
	if err := saveModelState(path, want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, ok := loadModelState(path)
	if !ok {
		t.Fatal("expected a successful load")
	}
	if !statesEqual(got, want) {
		t.Errorf("loaded state = %+v, want %+v", got, want)
	}
}

func TestLoadModelState_MissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.bin")
	if _, ok := loadModelState(path); ok {
		t.Error("loading a missing file should report ok == false")
	}
}

func TestLoadModelState_CorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	if err := os.WriteFile(path, []byte("not a gob stream"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, ok := loadModelState(path); ok {
		t.Error("loading corrupt bytes should report ok == false, not decode garbage")
	}
}

func TestLoadModelState_MismatchedVersion(t *testing.T) {
	// Simulate a file written by an incompatible build: same envelope type
	// (persistedModelState is unexported, but this test is in-package), a
	// different Version.
	path := filepath.Join(t.TempDir(), "future.bin")
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(persistedModelState{
		Version: modelStateVersion + 1,
		State:   testState(),
	}); err != nil {
		t.Fatalf("setup encode: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0600); err != nil {
		t.Fatalf("setup write: %v", err)
	}
	if _, ok := loadModelState(path); ok {
		t.Error("a save from an incompatible future version should be rejected")
	}
}

func TestSaveModelState_DoesNotCorruptPreviousSaveOnRewrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	first := testState()
	if err := saveModelState(path, first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Simulate a crash mid-write: the .tmp file is left behind with garbage,
	// but the real path must be untouched since the rename never happened.
	if err := os.WriteFile(path+".tmp", []byte("garbage from an interrupted write"), 0600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	got, ok := loadModelState(path)
	if !ok {
		t.Fatal("expected the previous save to still load")
	}
	if !statesEqual(got, first) {
		t.Error("a torn .tmp file must not affect the previously-saved real file")
	}

	// A real second save must still succeed and replace the first.
	second := testState()
	second.Gain = 9.87
	if err := saveModelState(path, second); err != nil {
		t.Fatalf("second save: %v", err)
	}
	got, ok = loadModelState(path)
	if !ok || !statesEqual(got, second) {
		t.Errorf("second save did not take effect: got %+v, want %+v", got, second)
	}
}

func TestDeleteModelState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bg.bin")
	if err := saveModelState(path, testState()); err != nil {
		t.Fatalf("setup save: %v", err)
	}
	deleteModelState(path)
	if _, ok := loadModelState(path); ok {
		t.Error("expected the file to be gone after delete")
	}
	// Deleting again (already gone) must not panic or error out audibly.
	deleteModelState(path)
}
