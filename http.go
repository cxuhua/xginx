package xginx

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/willf/bloom"
)

type IHttp interface {
	Start(ctx context.Context)
	Stop()
	Wait()
	TagFilter() gin.HandlerFunc
	GetDBImp(c *gin.Context) DBImp
}

var (
	Http IHttp = &xhttp{
		tbf:   bloom.New(50000, 10),
		dbkey: "dbkey",
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
	db := Http.GetDBImp(c)
	//校验客户端数据
	err := tag.Valid(db, cli)
	if err != nil {
		putError(c, err)
		return
	}
	tb := &SerPart{}
	//获取一个可签名的私钥
	spri := conf.GetPrivateKey()
	if spri == nil {
		putError(c, errors.New("private key mss"))
		return
	}
	//签名数据
	bb, err := tb.Sign(spri, tag, cli)
	if err != nil {
		putError(c, err)
		return
	}
	uv, err := VerifyUnit(conf, bb)
	if err != nil {
		putError(c, err)
		return
	}
	if err := uv.Save(db); err != nil {
		putError(c, err)
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", bb)
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

//在使用DBHandler之后可调用
func (h *xhttp) GetDBImp(c *gin.Context) DBImp {
	return c.MustGet(h.dbkey).(DBImp)
}

//挂接服务
func (h *xhttp) init(m *gin.Engine) {
	//添加测试标签
	h.AddTag([]byte{0x04, 0x7D, 0x14, 0x32, 0xAA, 0x61, 0x80})
	//签名接口
	m.POST("/sign/:hex", h.TagFilter(), h.DBHandler(), signAction)
}

//数据库
func (h *xhttp) DBHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		//数据库操作不能超过30秒
		ctx, cancel := context.WithTimeout(h.ctx, time.Second*30)
		defer cancel()
		_ = store.UseSession(ctx, func(db DBImp) error {
			c.Set(h.dbkey, db)
			c.Next()
			return nil
		})
	}
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
