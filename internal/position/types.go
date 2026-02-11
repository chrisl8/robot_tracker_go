package position

import "fmt"

type Point2D struct {
	X, Y float64
}

type Point3D struct {
	X, Y, Z float64
}

type Pose struct {
	Position Point2D
	Rotation float64
}

type Velocity struct {
	VX, VY float64
}

type RobotState struct {
	RobotID   int
	Position  Point2D
	Velocity  Velocity
	Timestamp float64
	Diameter  float64
}

type Obstacle struct {
	Name              string
	WorldTopLeft      Point2D
	WorldBottomRight  Point2D
	PixelsTopLeft     [2]int
	PixelsBottomRight [2]int
}

func (p *Point2D) DistanceTo(other Point2D) float64 {
	dx := p.X - other.X
	dy := p.Y - other.Y
	return sqrt(dx*dx + dy*dy)
}

func (p *Point2D) Add(other Point2D) Point2D {
	return Point2D{X: p.X + other.X, Y: p.Y + other.Y}
}

func (p *Point2D) Sub(other Point2D) Point2D {
	return Point2D{X: p.X - other.X, Y: p.Y - other.Y}
}

func (p *Point2D) Scale(factor float64) Point2D {
	return Point2D{X: p.X * factor, Y: p.Y * factor}
}

func (p *Point2D) Length() float64 {
	return sqrt(p.X*p.X + p.Y*p.Y)
}

func (p *Point2D) Normalize() Point2D {
	length := p.Length()
	if length == 0 {
		return Point2D{}
	}
	return p.Scale(1.0 / length)
}

func (p *Point2D) Dot(other Point2D) float64 {
	return p.X*other.X + p.Y*other.Y
}

func (p *Point2D) String() string {
	return fmt.Sprintf("(%.3f, %.3f)", p.X, p.Y)
}

func sqrt(x float64) float64 {
	if x < 0 {
		return 0
	}
	result := 1.0
	for i := 0; i < 50; i++ {
		result = (result + x/result) / 2
	}
	return result
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
