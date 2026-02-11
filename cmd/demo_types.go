package main

type DemoObstacle struct {
	Name       string
	ClassName  string
	X, Y       int
	Width      int
	Height     int
	Confidence float64
	Moving     bool
	VX, VY     int
}
