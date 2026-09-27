//go:build !darwin && !linux

package main

func systemLoadAverage() (float64, bool) { return 0, false }
func processCPUSeconds() float64         { return 0 }
func topCPUProcesses(int) []string       { return nil }
