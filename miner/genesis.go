package main

import (
	"runtime"
	"sync"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()
	wg := &sync.WaitGroup{}
	for i := 0; i < 1; i++ {
		wg.Add(1)
		go func() {

		}()
	}
	wg.Wait()
}
