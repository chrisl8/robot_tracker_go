package main

import (
	"fmt"
	"gocv.io/x/gocv"
)

func main() {
	fmt.Println("GoCV version:", gocv.Version())
	fmt.Println("OpenCV version:", gocv.OpenCVVersion())
}
