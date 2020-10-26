package main

import (
	"log"
	"testing"
)

func TestA(t *testing.T) {
	//1-100
	f := []float32{1, 50, 20, 40, 78, 34, 100, 1}
	x := float32(0.0)
	for _, v := range f {
		x += v
	}
	s := x / float32(len(f))
	for i, v := range f {
		f[i] = (v - s) / 100
	}
	log.Println(f)
}
