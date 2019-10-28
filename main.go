package xginx

import (
	"context"
	"log"
	"time"
)

func Main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer func() {
		cancel()
	}()
	s, err := NewServer(ctx, conf)
	if err != nil {
		panic(err)
	}
	go s.run()

	time.Sleep(time.Second * 2)

	d := NetAddr{}
	if err := d.From("127.0.0.1:9333"); err != nil {
		panic(err)
	}
	c := NewClient(ctx)
	err = c.connect(d)

	log.Println(err)

	go c.loop()

	time.Sleep(time.Second * 5)

	c.Close()

	time.Sleep(time.Hour)
}
