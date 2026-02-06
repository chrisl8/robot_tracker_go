package main

/*
#cgo CGO_CPPFLAGS: -IC:/opencv/build/install/include
#cgo CGO_LDFLAGS: -LC:/opencv/build/install/x64/mingw/lib -lopencv_core4130
#include <opencv2/opencv2.hpp>
*/
import "C"
import "fmt"

func main() {
	fmt.Println("OpenCV version:", C.CV_VERSION)
}
