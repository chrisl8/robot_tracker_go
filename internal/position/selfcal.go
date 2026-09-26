package position

import (
	"errors"
	"fmt"
	"math"

	"gonum.org/v1/gonum/mat"
)

// tagPose is where a target tag lies on the floor: its centre and rotation
// in the world frame defined by the Center tag.
type tagPose struct {
	X, Y, Theta float64
}

// layoutSolution is the outcome of solving for the floor homography and the
// tag poses together.
type layoutSolution struct {
	worldToPixel [3][3]float64
	poses        []tagPose // poses[0] is the Center tag: identity
}

var errLayoutSolve = errors.New("could not solve the calibration layout")

// solveLayout jointly estimates the floor-to-pixel homography and the pose of
// every non-Center tag from the detected pixel corners of tags of a KNOWN
// physical size. The Center tag (obs[0]) fixes the world frame: origin at its
// centre, axes along its edges. Nothing about where the other tags were placed
// is assumed, so they can be laid out by eye.
//
// It is a Levenberg-Marquardt least-squares fit on pixel reprojection error,
// started from the homography implied by the Center tag alone.
func solveLayout(obs [][4]Point2D) (*layoutSolution, error) {
	if len(obs) < 2 {
		return nil, fmt.Errorf("%w: need at least two tags", errLayoutSolve)
	}

	local := localTagCorners()
	init := NewHomography()
	if err := init.ComputeFromPoints(local[:], obs[0][:]); err != nil {
		return nil, fmt.Errorf("%w: center tag: %v", errLayoutSolve, err)
	}
	h0Inv, ok := inverse3(init.H)
	if !ok {
		return nil, fmt.Errorf("%w: center tag homography is singular", errLayoutSolve)
	}

	nTags := len(obs)
	nParams := 8 + 3*(nTags-1)
	p := make([]float64, nParams)
	scale := make([]float64, nParams)
	for i := 0; i < 8; i++ {
		p[i] = init.H[i/3][i%3]
		scale[i] = math.Max(math.Abs(p[i]), 1e-3)
	}
	for k := 1; k < nTags; k++ {
		var w [4]Point2D
		var cx, cy float64
		for c := 0; c < 4; c++ {
			w[c] = applyH(h0Inv, obs[k][c])
			cx += w[c].X / 4
			cy += w[c].Y / 4
		}
		top := Point2D{X: (w[1].X - w[0].X + w[2].X - w[3].X) / 2, Y: (w[1].Y - w[0].Y + w[2].Y - w[3].Y) / 2}
		o := 8 + 3*(k-1)
		p[o], p[o+1], p[o+2] = cx, cy, math.Atan2(top.Y, top.X)
		scale[o], scale[o+1], scale[o+2] = 0.1, 0.1, 0.1
	}

	residuals := func(q []float64) []float64 {
		return layoutResiduals(q, scale, obs, local)
	}

	q := make([]float64, nParams)
	for i := range q {
		q[i] = p[i] / scale[i]
	}
	if err := levenbergMarquardt(q, residuals); err != nil {
		return nil, err
	}

	sol := &layoutSolution{poses: make([]tagPose, nTags)}
	hm := unpackHomography(q, scale)
	sol.worldToPixel = hm
	for k := 1; k < nTags; k++ {
		o := 8 + 3*(k-1)
		sol.poses[k] = tagPose{X: q[o] * scale[o], Y: q[o+1] * scale[o+1], Theta: q[o+2] * scale[o+2]}
	}

	// Every corner must project in front of the camera with a consistent sign.
	sign := 0.0
	for k := 0; k < nTags; k++ {
		for _, w := range poseCorners(sol.poses[k], local) {
			d := hm[2][0]*w.X + hm[2][1]*w.Y + hm[2][2]
			if d == 0 || (sign != 0 && d*sign < 0) {
				return nil, fmt.Errorf("%w: solution projects tags behind the camera", errLayoutSolve)
			}
			sign = d
		}
	}
	return sol, nil
}

// localTagCorners returns a target tag's corners in its own frame, in
// detector order (top-left, top-right, bottom-right, bottom-left), origin at
// the tag centre, +y down.
func localTagCorners() [4]Point2D {
	h := TargetTagSize / 2
	return [4]Point2D{{X: -h, Y: -h}, {X: h, Y: -h}, {X: h, Y: h}, {X: -h, Y: h}}
}

func poseCorners(pose tagPose, local [4]Point2D) [4]Point2D {
	c, s := math.Cos(pose.Theta), math.Sin(pose.Theta)
	var out [4]Point2D
	for i, l := range local {
		out[i] = Point2D{X: c*l.X - s*l.Y + pose.X, Y: s*l.X + c*l.Y + pose.Y}
	}
	return out
}

func unpackHomography(q, scale []float64) [3][3]float64 {
	var h [3][3]float64
	for i := 0; i < 8; i++ {
		h[i/3][i%3] = q[i] * scale[i]
	}
	h[2][2] = 1
	return h
}

func applyH(m [3][3]float64, p Point2D) Point2D {
	w := m[2][0]*p.X + m[2][1]*p.Y + m[2][2]
	if w == 0 {
		return Point2D{X: math.Inf(1), Y: math.Inf(1)}
	}
	return Point2D{
		X: (m[0][0]*p.X + m[0][1]*p.Y + m[0][2]) / w,
		Y: (m[1][0]*p.X + m[1][1]*p.Y + m[1][2]) / w,
	}
}

func layoutResiduals(q, scale []float64, obs [][4]Point2D, local [4]Point2D) []float64 {
	hm := unpackHomography(q, scale)
	res := make([]float64, 0, len(obs)*8)
	for k := range obs {
		pose := tagPose{}
		if k > 0 {
			o := 8 + 3*(k-1)
			pose = tagPose{X: q[o] * scale[o], Y: q[o+1] * scale[o+1], Theta: q[o+2] * scale[o+2]}
		}
		for i, w := range poseCorners(pose, local) {
			pix := applyH(hm, w)
			if math.IsInf(pix.X, 0) || math.IsNaN(pix.X) || math.IsNaN(pix.Y) {
				res = append(res, 1e6, 1e6)
				continue
			}
			res = append(res, pix.X-obs[k][i].X, pix.Y-obs[k][i].Y)
		}
	}
	return res
}

func sumSquares(r []float64) float64 {
	s := 0.0
	for _, v := range r {
		s += v * v
	}
	return s
}

// levenbergMarquardt minimises sum(resid(q)^2) in place using central-difference
// Jacobians. The problem sizes here are tiny (<= ~20 parameters).
func levenbergMarquardt(q []float64, resid func([]float64) []float64) error {
	n := len(q)
	r := resid(q)
	cost := sumSquares(r)
	if math.IsNaN(cost) || math.IsInf(cost, 0) {
		return fmt.Errorf("%w: non-finite starting error", errLayoutSolve)
	}

	lambda := 1e-3
	for iter := 0; iter < 200; iter++ {
		jac := mat.NewDense(len(r), n, nil)
		for j := 0; j < n; j++ {
			step := 1e-6 * math.Max(1, math.Abs(q[j]))
			qp := append([]float64(nil), q...)
			qm := append([]float64(nil), q...)
			qp[j] += step
			qm[j] -= step
			rp, rm := resid(qp), resid(qm)
			for i := range r {
				jac.Set(i, j, (rp[i]-rm[i])/(2*step))
			}
		}

		var jtj mat.Dense
		jtj.Mul(jac.T(), jac)
		grad := mat.NewVecDense(n, nil)
		grad.MulVec(jac.T(), mat.NewVecDense(len(r), r))

		improved := false
		for try := 0; try < 30; try++ {
			damped := mat.DenseCopyOf(&jtj)
			for i := 0; i < n; i++ {
				damped.Set(i, i, jtj.At(i, i)*(1+lambda)+1e-12)
			}
			var dq mat.VecDense
			rhs := mat.NewVecDense(n, nil)
			rhs.ScaleVec(-1, grad)
			if err := dq.SolveVec(damped, rhs); err != nil {
				lambda *= 10
				continue
			}

			trial := make([]float64, n)
			for i := range trial {
				trial[i] = q[i] + dq.AtVec(i)
			}
			rTrial := resid(trial)
			cTrial := sumSquares(rTrial)
			if !math.IsNaN(cTrial) && cTrial < cost {
				decrease := cost - cTrial
				copy(q, trial)
				r, cost = rTrial, cTrial
				lambda = math.Max(lambda/3, 1e-9)
				improved = true
				if decrease <= 1e-12*(1+cost) {
					return nil
				}
				break
			}
			lambda *= 4
		}
		if !improved {
			return nil
		}
	}
	return nil
}

func inverse3(m [3][3]float64) ([3][3]float64, bool) {
	det := m[0][0]*(m[1][1]*m[2][2]-m[1][2]*m[2][1]) -
		m[0][1]*(m[1][0]*m[2][2]-m[1][2]*m[2][0]) +
		m[0][2]*(m[1][0]*m[2][1]-m[1][1]*m[2][0])
	if math.Abs(det) < 1e-300 || math.IsNaN(det) {
		return [3][3]float64{}, false
	}
	inv := 1 / det
	return [3][3]float64{
		{(m[1][1]*m[2][2] - m[1][2]*m[2][1]) * inv, (m[0][2]*m[2][1] - m[0][1]*m[2][2]) * inv, (m[0][1]*m[1][2] - m[0][2]*m[1][1]) * inv},
		{(m[1][2]*m[2][0] - m[1][0]*m[2][2]) * inv, (m[0][0]*m[2][2] - m[0][2]*m[2][0]) * inv, (m[0][2]*m[1][0] - m[0][0]*m[1][2]) * inv},
		{(m[1][0]*m[2][1] - m[1][1]*m[2][0]) * inv, (m[0][1]*m[2][0] - m[0][0]*m[2][1]) * inv, (m[0][0]*m[1][1] - m[0][1]*m[1][0]) * inv},
	}, true
}
