package main

/*
#include <opencv2/core/version.hpp>
#include <stdio.h>
*/
import "C"
import "fmt"

func main() {
	fmt.Printf("OpenCV major version: %d\n", C.CV_VERSION_MAJOR)
	fmt.Printf("OpenCV minor version: %d\n", C.CV_VERSION_MINOR)
	fmt.Printf("OpenCV version: %s\n", C.CV_VERSION)
}
