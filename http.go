package xginx

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/patrickmn/go-cache"
)

type IHttp interface {
	Start(ctx context.Context, lis IListener)
	Stop()
	Wait()
}

var (
	Http IHttp = &xhttp{
		cache: cache.New(time.Hour*1, time.Hour*4),
	}
)

type xhttp struct {
	tmu    sync.RWMutex
	shttp  *http.Server
	ctx    context.Context
	cancel context.CancelFunc
	cache  *cache.Cache
}

const (
	apidbkey = "apidbkey"
	userkey  = "userkey"
)

type apidb struct {
	lis IListener
	xhp *xhttp
}

func ApiGetDB(c *gin.Context) *apidb {
	return c.MustGet(apidbkey).(*apidb)
}

func NewApiDB(db *apidb) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set(apidbkey, db)
		c.Next()
	}
}

type ApiResult struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

func genToken() (HASH160, error) {
	token := HASH160{}
	pri, err := NewPrivateKey()
	if err != nil {
		return token, err
	}
	return pri.PublicKey().Hash(), nil
}

func loginApi(c *gin.Context) {
	args := struct {
		User string `form:"user"`
		Pass string `form:"pass"`
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	db := ApiGetDB(c)
	wallet := db.lis.GetWallet()
	hv, flags, err := wallet.GetAdminInfo(args.User)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	lv := Hash256([]byte(args.Pass))
	if !bytes.Equal(lv, hv) {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  "password error",
		})
		return
	}
	token, err := genToken()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 103,
			Msg:  err.Error(),
		})
		return
	}
	key := hex.EncodeToString(token[:])
	db.xhp.cache.Set(key, flags, time.Minute*30)
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  key,
	})
}

//登陆后存在
func getApiUser(c *gin.Context) string {
	return c.MustGet(userkey).(string)
}

func isLoginApi(c *gin.Context) {
	token := c.GetHeader("X-Access-Token")
	if token == "" {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	db := ApiGetDB(c)
	flags, has := db.xhp.cache.Get(token)
	if !has {
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}
	c.Set(userkey, flags.(uint32))
	c.Next()
}

//挂接服务
func (h *xhttp) init(m *gin.Engine, lis IListener) {
	m.Use(NewApiDB(&apidb{lis: lis, xhp: h}))
	//登陆钱包
	m.POST("/login", loginApi)
	//管理接口
	mgr := m.Group("/mgr", isLoginApi)
	//获取钱包地址列表
	mgr.GET("/list/address", listAddressApi)
	//创建新账号
	mgr.POST("/new/account", newAccountApi)
	//设置矿工账号
	mgr.POST("/set/miner", setMinerApi)
	//获取矿工奖励地址
	mgr.GET("/get/miner", getMinerApi)
	//创建一个区块
	mgr.POST("/new/block", newBlockApi)
	//转账
	mgr.POST("/transfer", transferFee)
	//数据浏览接口
	v1 := m.Group("v1")
	//获取链接列表
	v1.GET("/list/conn", listClients)
	//获取状态
	v1.GET("/state", getStatusApi)
	//获取区块信息
	v1.GET("/block/:id", getBlockInfoApi)
	//获取交易信息
	v1.GET("/tx/:id", getTxInfoApi)
	//获取最新区块列表
	v1.GET("/list/block", listBestBlock)
	//获取某地址的余额
	v1.GET("/coins/:addr", listCoins)

	lis.OnInitHttp(m)
}

func (h *xhttp) Start(ctx context.Context, lis IListener) {
	h.ctx, h.cancel = context.WithCancel(ctx)
	addr := fmt.Sprintf(":%d", conf.HttpPort)
	m := gin.Default()
	h.init(m, lis)
	h.shttp = &http.Server{
		Addr:    addr,
		Handler: m,
	}
	go func() {
		LogInfo("start http server", addr)
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
		LogInfo("stop http done")
	}
}
