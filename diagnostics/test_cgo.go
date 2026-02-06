package main

import "C"

import "fmt"

/*
#cgo CGO_CPPFLAGS: -IC:/opencv/build/install/include
#cgo CGO_LDFLAGS: -LC:/opencv/build/install/x64/mingw/lib -lopencv_core4130 -lopencv_imgproc4130
#include <opencv2/opencv2.hpp>
#include <stdio.h>
*/

func main() {
	fmt.Println("Test CGO compilation")
}
