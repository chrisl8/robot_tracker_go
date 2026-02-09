package main

type DemoObstacle struct {
	name       string
	className  string
	x, y       int
	width      int
	height     int
	confidence float64
	moving     bool
	vx, vy     int
}
