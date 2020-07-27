package main

import (
	"context"
	"log"
	"testing"
	"time"
)

func TestCallApi(t *testing.T) {
	lis := &shoplistener{}

	lis.startrpc("tcp://127.0.0.1:9335")

	ctx := context.Background()
	pool := NewApiClientPool(ctx, "127.0.0.1:9335")
	defer pool.Close(ctx)

	time.Sleep(time.Second)

	req := ApiType{"bb": 1}
	res, err := CallApi(pool, "Invoke", req)

	log.Println(err, res)
}
