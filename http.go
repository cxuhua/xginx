package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"

	"sync"

	"github.com/gin-gonic/gin"

	"github.com/willf/bloom"
)

type IHttp interface {
	Start(ctx context.Context)
	Stop()
	Wait()
	TagFilter() gin.HandlerFunc
}

var (
	Http IHttp = &xhttp{
		tbf: bloom.New(50000, 10),
	}
)

type xhttp struct {
	tbf    *bloom.BloomFilter //标签id过滤器
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

//验证服务
func verifyAction(c *gin.Context) {
	hs := c.Param("hex")
	if len(hs) != 64 {
		putError(c, errors.New("hex hash error"))
		return
	}
	bb, err := hex.DecodeString(hs)
	if err != nil {
		putError(c, err)
		return
	}
	id := HASH256{}
	if copy(id[:], bb) != 32 {
		putError(c, errors.New("hex hash length error"))
		return
	}
	pkh, err := HasUnitash(id)
	if err != nil {
		putError(c, err)
		return
	}
	//返回用户 公钥pkh
	c.Data(http.StatusOK, "application/octet-stream", pkh[:])
}

// 签名服务
func signAction(c *gin.Context) {
	url := conf.HttpScheme + "://" + c.Request.Host + c.Request.RequestURI
	tag := NewTagInfo(url)
	//客户端服务器端都要解码
	if err := tag.DecodeURL(); err != nil {
		putError(c, err)
		return
	}
	cli := &CliPart{}
	if err := cli.Decode(c.Request.Body); err != nil {
		putError(c, err)
		return
	}
	//校验客户端数据
	err := tag.Valid(cli)
	if err != nil {
		putError(c, err)
		return
	}
	tb := NewSerPart(conf.HttpScheme + "://" + c.Request.Host + "/verify")
	ud, err := tb.Dump(tag, cli)
	if err != nil {
		putError(c, err)
		return
	}
	id := Hash256To(ud)
	if err := PutUnitHash(id, cli.CPks); err != nil {
		putError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", ud)
}

//接收数据单元
func recvUnitAction(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		putError(c, err)
		return
	}
	buf := bytes.NewReader(data)
	uv := &Unit{}
	if err := uv.Decode(buf); err != nil {
		putError(c, err)
		return
	}
	if _, err := uv.Verify(); err != nil {
		putError(c, err)
		return
	}
	if Miner != nil {
		Miner.OnUnit(uv)
	}
	c.Status(http.StatusOK)
}

func (h *xhttp) AddTag(uid []byte) {
	h.tmu.Lock()
	defer h.tmu.Unlock()
	h.tbf.Add(uid)
}

func (h *xhttp) TestTag(uid []byte) bool {
	h.tmu.RLock()
	defer h.tmu.RUnlock()
	return h.tbf.Test(uid)
}

//挂接服务
func (h *xhttp) init(m *gin.Engine) {
	//一次加载所有标签
	LoadAllTags(h.tbf)
	//签名接口
	m.POST("/sign/:hex", h.TagFilter(), signAction)
	//校验接口
	m.GET("/verify/:hex", verifyAction)
	//接收数据单元
	m.POST("/recv/unit", recvUnitAction)

}

//过滤不可用的标签
func (h *xhttp) TagFilter() gin.HandlerFunc {
	return func(c *gin.Context) {
		hex := c.Param("hex")
		if len(hex) < 64 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if len(hex) > 512 {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		tag := TagInfo{}
		if err := tag.DecodeHex([]byte(hex)); err != nil {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		if !h.TestTag(tag.TUID[:]) {
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		c.Next()
	}
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
