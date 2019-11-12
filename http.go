package xginx

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
)

type IHttp interface {
	Start(ctx context.Context)
	Stop()
	Wait()
}

var (
	Http IHttp = &xhttp{}
)

type xhttp struct {
	tmu    sync.RWMutex
	shttp  *http.Server
	ctx    context.Context
	cancel context.CancelFunc
	dbkey  string
}

//输出错误
func putError(c *gin.Context, err error) {
	c.String(http.StatusBadRequest, err.Error())
	c.Abort()
}

//挂接服务
func (h *xhttp) init(m *gin.Engine) {

}

func (h *xhttp) Start(ctx context.Context) {
	h.ctx, h.cancel = context.WithCancel(ctx)
	addr := fmt.Sprintf(":%d", conf.HttpPort)
	m := gin.Default()
	h.init(m)
	h.shttp = &http.Server{
		Addr:    addr,
		Handler: m,
	}
	go func() {
		log.Println("start http server", addr)
		_ = h.shttp.ListenAndServe()
	}()
}

func (h *xhttp) Stop() {
	h.cancel()
	_ = h.shttp.Shutdown(h.ctx)
}

func (h *xhttp) Wait() {
	select {
	case <-h.ctx.Done():
		log.Println("stop http done")
	}
}
