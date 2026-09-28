package tracking

// KalmanFilter is a constant-velocity Kalman filter over state vector
// x = [x, y, vx, vy] (2D position + velocity). It's used by ByteTrack to
// predict a track's bbox center forward in time for IoU-based matching
// against new detections (see predictAllTracks in bytetrack.go); the
// filter's own velocity state (x[2], x[3]) is not read by anything outside
// that matching step — real robot velocity for navigation/control is
// computed independently in cmd/main.go's estimateRobotVelocity, directly
// from track timestamps.
//
// Predict takes dt (elapsed seconds) explicitly rather than assuming a
// fixed frame period, since camera frame timing varies (FPS drops, CPU
// spikes). Callers must pass the real elapsed time since the previous
// Predict/Update, not a constant.
type KalmanFilter struct {
	x           [4]float64
	P           [4][4]float64
	Q           [4][4]float64
	R           [2][2]float64
	F           [4][4]float64
	H           [2][4]float64
	initialized bool
}

func NewKalmanFilter() *KalmanFilter {
	kf := &KalmanFilter{
		initialized: false,
	}

	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			kf.P[i][j] = 0
			kf.Q[i][j] = 0
			kf.F[i][j] = 0
		}
	}

	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			kf.R[i][j] = 0
			kf.H[i][j] = 0
		}
	}

	kf.Q[0][0] = 1.0
	kf.Q[1][1] = 1.0
	kf.Q[2][2] = 1.0
	kf.Q[3][3] = 1.0

	kf.R[0][0] = 1.0
	kf.R[1][1] = 1.0

	// F[0][2] and F[1][3] (the dt-dependent velocity->position terms) are
	// set on each Predict call instead of here, since dt varies per call.
	kf.F[0][0] = 1
	kf.F[1][1] = 1
	kf.F[2][2] = 1
	kf.F[3][3] = 1

	kf.H[0][0] = 1
	kf.H[1][1] = 1

	return kf
}

func (kf *KalmanFilter) Initialize(x, y, cx, cy float64) {
	kf.x[0] = x
	kf.x[1] = y
	kf.x[2] = cx
	kf.x[3] = cy

	for i := 0; i < 4; i++ {
		kf.P[i][i] = 1000.0
	}

	kf.initialized = true
}

// Predict advances the filter's state by dt seconds using the
// constant-velocity model (position += velocity * dt) and grows the
// covariance by process noise scaled linearly by dt. dt must be the actual
// elapsed seconds since the previous Predict/Update call — see the
// KalmanFilter doc comment. The dt-scaled Q here is a simplified
// approximation (not a fully discretized white-noise-acceleration model);
// Q itself was already an uncalibrated identity guess, so this keeps the
// same character while at least making uncertainty growth track elapsed
// time instead of a fixed per-call amount.
func (kf *KalmanFilter) Predict(dt float64) [4]float64 {
	if !kf.initialized {
		return kf.x
	}

	kf.F[0][2] = dt
	kf.F[1][3] = dt

	xNew := [4]float64{}
	for i := 0; i < 4; i++ {
		xNew[i] = 0
		for j := 0; j < 4; j++ {
			xNew[i] += kf.F[i][j] * kf.x[j]
		}
	}

	// P = F*P*F^T + Q*dt. Both F factors are required: with only F*P the
	// covariance goes asymmetric and the velocity rows never couple to the
	// position rows, so the filter can never learn velocity.
	var FP [4][4]float64
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				FP[i][j] += kf.F[i][k] * kf.P[k][j]
			}
		}
	}
	PNew := [4][4]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			for k := 0; k < 4; k++ {
				PNew[i][j] += FP[i][k] * kf.F[j][k]
			}
		}
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			PNew[i][j] += kf.Q[i][j] * dt
		}
	}

	kf.x = xNew
	kf.P = PNew

	return kf.x
}

func (kf *KalmanFilter) Update(measurement [2]float64) [4]float64 {
	if !kf.initialized {
		kf.Initialize(measurement[0], measurement[1], 0, 0)
		return kf.x
	}

	// Innovation: y = z - H*x
	y := [2]float64{
		measurement[0] - kf.x[0],
		measurement[1] - kf.x[1],
	}

	// Innovation covariance: S = H*P*H^T + R
	// For H = [[1,0,0,0],[0,1,0,0]]: S[i][j] = P[i][j] + R[i][j]
	S := [2][2]float64{}
	for i := 0; i < 2; i++ {
		for j := 0; j < 2; j++ {
			S[i][j] = kf.P[i][j] + kf.R[i][j]
		}
	}

	detS := S[0][0]*S[1][1] - S[0][1]*S[1][0]
	if detS == 0 {
		return kf.x
	}

	SInv := [2][2]float64{}
	SInv[0][0] = S[1][1] / detS
	SInv[0][1] = -S[0][1] / detS
	SInv[1][0] = -S[1][0] / detS
	SInv[1][1] = S[0][0] / detS

	// Kalman gain: K = P*H^T*S^(-1)
	// For this H: (P*H^T)[i][j] = P[i][j] for j<2
	K := [4][2]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 2; j++ {
			for k := 0; k < 2; k++ {
				K[i][j] += kf.P[i][k] * SInv[k][j]
			}
		}
	}

	// State update: x = x + K*y
	for i := 0; i < 4; i++ {
		for j := 0; j < 2; j++ {
			kf.x[i] += K[i][j] * y[j]
		}
	}

	// Covariance update: P = (I - K*H)*P
	// For this H: (K*H)[i][k] = K[i][k] for k<2, 0 for k>=2
	PNew := [4][4]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			PNew[i][j] = kf.P[i][j]
			for k := 0; k < 2; k++ {
				PNew[i][j] -= K[i][k] * kf.P[k][j]
			}
		}
	}

	kf.P = PNew

	return kf.x
}

func (kf *KalmanFilter) GetState() [4]float64 {
	return kf.x
}

func (kf *KalmanFilter) GetCovariance() [4][4]float64 {
	return kf.P
}

// bboxToCenter returns the bbox's center point. The sum is cast to float64
// before dividing by 2, not after — integer division would truncate an odd
// pixel sum toward zero and lose up to 0.5px per measurement, compounding
// across frames into real-world position error.
func bboxToCenter(bbox [4]int) (float64, float64) {
	cx := float64(bbox[0]+bbox[2]) / 2.0
	cy := float64(bbox[1]+bbox[3]) / 2.0
	return cx, cy
}

func centerToBbox(cx, cy float64, width, height int) [4]int {
	x1 := int(cx) - width/2
	y1 := int(cy) - height/2
	x2 := int(cx) + width/2
	y2 := int(cy) + height/2
	if x1 < 0 {
		x1 = 0
	}
	if y1 < 0 {
		y1 = 0
	}
	return [4]int{x1, y1, x2, y2}
}
