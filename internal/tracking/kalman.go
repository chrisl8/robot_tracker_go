package tracking

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

	kf.F[0][0] = 1
	kf.F[0][2] = 1
	kf.F[1][1] = 1
	kf.F[1][3] = 1
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

func (kf *KalmanFilter) Predict() [4]float64 {
	if !kf.initialized {
		return kf.x
	}

	xNew := [4]float64{}
	for i := 0; i < 4; i++ {
		xNew[i] = 0
		for j := 0; j < 4; j++ {
			xNew[i] += kf.F[i][j] * kf.x[j]
		}
	}

	PNew := [4][4]float64{}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			PNew[i][j] = 0
			for k := 0; k < 4; k++ {
				PNew[i][j] += kf.F[i][k] * kf.P[k][j]
			}
		}
	}
	for i := 0; i < 4; i++ {
		for j := 0; j < 4; j++ {
			PNew[i][j] += kf.Q[i][j]
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

func bboxToCenter(bbox [4]int) (float64, float64) {
	cx := float64((bbox[0] + bbox[2]) / 2)
	cy := float64((bbox[1] + bbox[3]) / 2)
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
