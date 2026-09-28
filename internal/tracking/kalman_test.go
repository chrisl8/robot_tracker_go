package tracking

import (
	"testing"
)

func TestNewKalmanFilter(t *testing.T) {
	kf := NewKalmanFilter()

	if kf == nil {
		t.Fatal("NewKalmanFilter returned nil")
	}
	if kf.initialized {
		t.Error("New KalmanFilter should not be initialized")
	}
	if kf.x[0] != 0 || kf.x[1] != 0 || kf.x[2] != 0 || kf.x[3] != 0 {
		t.Error("Initial state should be zero")
	}
}

func TestKalmanFilter_Initialize(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 150, 250)

	if !kf.initialized {
		t.Error("After Initialize, KalmanFilter should be initialized")
	}
	if kf.x[0] != 100 {
		t.Errorf("x[0] = %f, want 100", kf.x[0])
	}
	if kf.x[1] != 200 {
		t.Errorf("x[1] = %f, want 200", kf.x[1])
	}
	if kf.x[2] != 150 {
		t.Errorf("x[2] = %f, want 150", kf.x[2])
	}
	if kf.x[3] != 250 {
		t.Errorf("x[3] = %f, want 250", kf.x[3])
	}
}

func TestKalmanFilter_Initialize_SetsCovariance(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 0, 0)

	for i := 0; i < 4; i++ {
		if kf.P[i][i] != 1000.0 {
			t.Errorf("P[%d][%d] = %f, want 1000.0", i, i, kf.P[i][i])
		}
	}
}

func TestKalmanFilter_Predict_NotInitialized(t *testing.T) {
	kf := NewKalmanFilter()
	state := kf.Predict(1.0)

	for i := 0; i < 4; i++ {
		if state[i] != 0 {
			t.Errorf("Uninitialized Predict should return zero state, got %v", state)
		}
	}
}

func TestKalmanFilter_Predict_Initialized(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 5, 10)

	state := kf.Predict(1.0)

	if state[0] != 105 {
		t.Errorf("After one predict, x[0] = %f, want 105", state[0])
	}
	if state[1] != 210 {
		t.Errorf("After one predict, x[1] = %f, want 210", state[1])
	}
	if state[2] != 5 {
		t.Errorf("After one predict, x[2] = %f, want 5 (constant velocity)", state[2])
	}
	if state[3] != 10 {
		t.Errorf("After one predict, x[3] = %f, want 10 (constant velocity)", state[3])
	}
}

func TestKalmanFilter_Update_NotInitialized(t *testing.T) {
	kf := NewKalmanFilter()
	state := kf.Update([2]float64{100, 200})

	if !kf.initialized {
		t.Error("After Update, KalmanFilter should be initialized")
	}
	if state[0] != 100 {
		t.Errorf("After Update, x[0] = %f, want 100", state[0])
	}
	if state[1] != 200 {
		t.Errorf("After Update, x[1] = %f, want 200", state[1])
	}
}

func TestKalmanFilter_Update_Initialized(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 0, 0)

	state := kf.Update([2]float64{100, 200})

	if state[0] != 100 {
		t.Errorf("After Update with same measurement, x[0] = %f, want 100", state[0])
	}
	if state[1] != 200 {
		t.Errorf("After Update with same measurement, x[1] = %f, want 200", state[1])
	}
}

func TestKalmanFilter_GetState(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 5, 10)

	state := kf.GetState()

	if state[0] != 100 || state[1] != 200 || state[2] != 5 || state[3] != 10 {
		t.Errorf("GetState = %v, want [100, 200, 5, 10]", state)
	}
}

func TestKalmanFilter_GetCovariance(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 0, 0)

	cov := kf.GetCovariance()

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			if i == j && cov[i][j] == 0 {
				t.Errorf("Diagonal covariance should not be zero")
			}
		}
	}
}

func TestKalmanFilter_PredictUpdate(t *testing.T) {
	kf := NewKalmanFilter()

	kf.Initialize(0, 0, 1, 1)

	kf.Predict(1.0)
	state1 := kf.Update([2]float64{100, 100})

	if state1[0] < 0 || state1[0] > 100 {
		t.Errorf("After predict and update, x[0] should be between 0 and 100, got %f", state1[0])
	}
	if state1[1] < 0 || state1[1] > 100 {
		t.Errorf("After predict and update, x[1] should be between 0 and 100, got %f", state1[1])
	}
}

func TestBboxToCenter(t *testing.T) {
	tests := []struct {
		bbox      [4]int
		expectedX float64
		expectedY float64
	}{
		{[4]int{0, 0, 100, 200}, 50, 100},
		{[4]int{10, 20, 110, 220}, 60, 120},
		{[4]int{0, 0, 0, 0}, 0, 0},
		// Odd coordinate sums: regression test for integer-division
		// truncation (dividing before casting to float loses the .5px).
		{[4]int{0, 0, 101, 201}, 50.5, 100.5},
		{[4]int{1, 1, 100, 200}, 50.5, 100.5},
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			cx, cy := bboxToCenter(tt.bbox)
			if cx != tt.expectedX || cy != tt.expectedY {
				t.Errorf("bboxToCenter(%v) = (%f, %f), want (%f, %f)",
					tt.bbox, cx, cy, tt.expectedX, tt.expectedY)
			}
		})
	}
}

func TestCenterToBbox(t *testing.T) {
	tests := []struct {
		name          string
		cx, cy        float64
		width, height int
		expected      [4]int
	}{
		{
			name:     "centered at 50,50 with size 20x30",
			cx:       50,
			cy:       50,
			width:    20,
			height:   30,
			expected: [4]int{40, 35, 60, 65},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := centerToBbox(tt.cx, tt.cy, tt.width, tt.height)
			if result != tt.expected {
				t.Errorf("centerToBbox(%f, %f, %d, %d) = %v, want %v",
					tt.cx, tt.cy, tt.width, tt.height, result, tt.expected)
			}
		})
	}
}

func TestKalmanFilter_MultipleUpdates(t *testing.T) {
	kf := NewKalmanFilter()

	kf.Update([2]float64{100, 100})
	state1 := kf.GetState()

	kf.Update([2]float64{100, 100})
	state2 := kf.GetState()

	if state1[0] != state2[0] || state1[1] != state2[1] {
		t.Error("Multiple identical updates should converge to measurement")
	}
}

func TestKalmanFilter_MultiplePredicts(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(0, 0, 1, 1)

	state0 := kf.GetState()
	state1 := kf.Predict(1.0)
	state2 := kf.Predict(1.0)
	state3 := kf.Predict(1.0)

	if state1[0] != state0[0]+1 {
		t.Error("First predict should add velocity")
	}
	if state2[0] != state1[0]+1 {
		t.Error("Second predict should add velocity again")
	}
	if state3[0] != state2[0]+1 {
		t.Error("Third predict should add velocity again")
	}
}

func TestKalmanFilter_Predict_ScalesPositionByDt(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(0, 0, 5, 10)

	stateHalf := kf.Predict(0.5)
	if stateHalf[0] != 2.5 {
		t.Errorf("With dt=0.5 and vx=5, x[0] = %f, want 2.5", stateHalf[0])
	}
	if stateHalf[1] != 5 {
		t.Errorf("With dt=0.5 and vy=10, x[1] = %f, want 5", stateHalf[1])
	}

	kf2 := NewKalmanFilter()
	kf2.Initialize(0, 0, 5, 10)
	stateDouble := kf2.Predict(2.0)
	if stateDouble[0] != 10 {
		t.Errorf("With dt=2 and vx=5, x[0] = %f, want 10", stateDouble[0])
	}
	if stateDouble[1] != 20 {
		t.Errorf("With dt=2 and vy=10, x[1] = %f, want 20", stateDouble[1])
	}
}

func TestKalmanFilter_Predict_ScalesProcessNoiseByDt(t *testing.T) {
	kfSmall := NewKalmanFilter()
	kfSmall.Initialize(0, 0, 1, 1)
	kfSmall.Predict(1.0)
	covSmall := kfSmall.GetCovariance()

	kfLarge := NewKalmanFilter()
	kfLarge.Initialize(0, 0, 1, 1)
	kfLarge.Predict(2.0)
	covLarge := kfLarge.GetCovariance()

	for i := 0; i < 4; i++ {
		if covLarge[i][i] <= covSmall[i][i] {
			t.Errorf("Predict with larger dt should add more process noise: covLarge[%d][%d]=%f, covSmall[%d][%d]=%f",
				i, i, covLarge[i][i], i, i, covSmall[i][i])
		}
	}
}

func TestKalmanFilter_ZeroVelocity(t *testing.T) {
	kf := NewKalmanFilter()
	kf.Initialize(100, 200, 0, 0)

	state := kf.Predict(1.0)

	if state[0] != 100 {
		t.Errorf("With zero velocity, x[0] should remain 100, got %f", state[0])
	}
	if state[1] != 200 {
		t.Errorf("With zero velocity, x[1] should remain 200, got %f", state[1])
	}
}
