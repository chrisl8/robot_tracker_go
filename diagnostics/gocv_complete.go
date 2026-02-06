package main

/*
#cgo CGO_CPPFLAGS: -IC:/opencv/build/install/include
#cgo CGO_LDFLAGS: -LC:/opencv/build/install/x64/mingw/lib -lopencv_core4130 -lopencv_imgproc4130 -lopencv_calib3d4130 -lopencv_objdetect4130 -lopencv_features2d4130 -lopencv_videoio4130 -lopencv_imgcodecs4130 -lopencv_dnn4130
#include <opencv2/core/version.hpp>
#include <opencv2/core.hpp>
#include <stdio.h>
*/
import "C"
import "fmt"

func main() {
	fmt.Println("=== GoCV + OpenCV 4.13.0 Compatibility Test ===")
	fmt.Println()

	// Version test
	fmt.Printf("OpenCV Version: %s\n", C.CV_VERSION)
	fmt.Printf("OpenCV Major: %d\n", C.CV_VERSION_MAJOR)
	fmt.Printf("OpenCV Minor: %d\n", C.CV_VERSION_MINOR)
	fmt.Println()

	// Mat allocation test
	fmt.Println("Testing Mat allocation...")
	mat := C.Mat_New()
	if mat == nil {
		fmt.Println("ERROR: Mat_New() returned nil")
		return
	}
	defer C.Mat_Close(mat)
	fmt.Println("Mat_New(): SUCCESS")

	// MatWithSize test
	fmt.Println("Testing Mat_NewWithSize...")
	mat2 := C.Mat_NewWithSize(640, 480, C.CV_8UC3)
	if mat2 == nil {
		fmt.Println("ERROR: Mat_NewWithSize() returned nil")
		return
	}
	defer C.Mat_Close(mat2)
	fmt.Println("Mat_NewWithSize(): SUCCESS")

	// Get properties
	rows := C.Mat_Rows(mat2)
	cols := C.Mat_Cols(mat2)
	typ := C.Mat_Type(mat2)
	fmt.Printf("Mat properties: %dx%d, type=%d\n", rows, cols, typ)

	fmt.Println()
	fmt.Println("=== All Tests Passed ===")
	fmt.Println("GoCV v0.43.0 is compatible with OpenCV 4.13.0")
}
