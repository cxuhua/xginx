package main

import (
	"log"

	"github.com/cxuhua/xginx"
)

func main() {
	c, err := xginx.NewRpcClient("127.0.0.1", 9330)
	if err != nil {
		panic(err)
	}
	//log.Println(c.ListAccount())
	//log.Println(c.CreateAccount(1, 1, false))
	//log.Println(c.RemoveAccount("st1q28p9gn35n8at26q3u2xpjtasamf0qrryre4lvz"))
	log.Println(c.NewBlock(1))
}
