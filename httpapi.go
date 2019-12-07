package xginx

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

func alertMsgApi(c *gin.Context) {
	args := struct {
		Msg string `form:"msg"` //数据hex编码
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	msg := NewMsgAlert(args.Msg, nil)
	Server.BroadMsg(msg)
}

func setHeaderApi(c *gin.Context) {
	args := struct {
		Hex string `form:"hex"` //数据hex编码
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	if Miner == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  "miner not running",
		})
		return
	}
	b, err := hex.DecodeString(args.Hex)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	err = Miner.SetHeader(b)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 103,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  "OK",
	})
}

func getHeaderApi(c *gin.Context) {
	if Miner == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  "miner not running",
		})
		return
	}
	b, err := Miner.GetHeader()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  hex.EncodeToString(b),
	})
}

//导入账号
func importAccountApi(c *gin.Context) {
	args := struct {
		Str string `form:"str"` //账号字符串
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
	acc, err := wallet.ImportAccount(args.Str)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	addr, err := acc.GetAddress()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  string(addr),
	})
}

//导出账号
func exportAccountApi(c *gin.Context) {
	args := struct {
		Addr  Address `form:"addr"`  //账号地址
		IsPri bool    `form:"ispri"` //是否包含私钥
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
	str, err := wallet.ExportAccount(args.Addr, args.IsPri)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  str,
	})
}

//修改管理员密码
func updateUserPass(c *gin.Context) {
	args := struct {
		User    string `form:"user"`    //用户名
		OldPass string `form:"oldpass"` //旧密码
		NewPass string `form:"newpass"` //新密码
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
	_, opv, err := wallet.GetAdminInfo(args.User)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	err = wallet.SetAdminInfo(args.User, args.OldPass, args.NewPass, opv)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	token := c.GetHeader("X-Access-Token")
	db.xhp.cache.Delete(token)
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  "OK",
	})
}

func listAddrTxs(c *gin.Context) {
	addr := c.Param("addr")
	bi := GetBlockIndex()
	ds, err := bi.ListTxs(Address(addr))
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	type index struct {
		Tx  string `json:"tx"`
		Blk string `json:"blk"`
		Idx int    `json:"idx"`
	}
	type result struct {
		Code int     `json:"code"`
		Idxs []index `json:"idxs"`
	}
	res := result{}
	for _, v := range ds {
		i := index{}
		i.Tx = v.TxId.String()
		i.Idx = v.Value.TxIdx.ToInt()
		i.Blk = v.Value.BlkId.String()
		res.Idxs = append(res.Idxs, i)
	}
	c.JSON(http.StatusOK, res)
}

//获取一个地址的余额
func listCoins(c *gin.Context) {
	addr := c.Param("addr")
	bi := GetBlockIndex()
	ds, err := bi.ListCoins(Address(addr))
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	type coin struct {
		Tx       string `json:"tx"`
		Idx      int    `json:"idx"`
		Value    Amount `json:"value"`
		Pool     bool   `json:"pool"`
		Coinbase bool   `json:"coinbase"`
		Height   uint32 `json:"height"`
	}
	type result struct {
		Code   int    `json:"code"`
		Coins  []coin `json:"coins"`
		Amount Amount `json:"amount"`
	}
	res := result{}
	total := Amount(0)
	for _, v := range ds {
		i := coin{}
		i.Tx = v.TxId.String()
		i.Idx = v.Index.ToInt()
		i.Value = v.Value
		i.Pool = v.pool
		i.Coinbase = v.Base == 1
		i.Height = v.Height.ToUInt32()
		res.Coins = append(res.Coins, i)
		total += v.Value
	}
	res.Amount = total
	c.JSON(http.StatusOK, res)
}

//转账
func transferFee(c *gin.Context) {
	args := struct {
		Src    Address `form:"src"`    //从src地址
		Keep   int     `form:"keep"`   //找零地址索引
		Dst    Address `form:"dst"`    //转到dst地址
		Amount string  `form:"amount"` //转账金额 单位:1.2 30.2
		Fee    string  `form:"fee"`    //交易费
		Ext    string  `form:"ext"`    //扩展信息
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	bi := GetBlockIndex()
	amt, err := ParseMoney(args.Amount)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	fee, err := ParseMoney(args.Fee)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	ext, err := hex.DecodeString(args.Ext)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 103,
			Msg:  err.Error(),
		})
		return
	}
	if len(ext) > MAX_EXT_SIZE {
		c.JSON(http.StatusOK, ApiResult{
			Code: 103,
			Msg:  "ext too big",
		})
		return
	}
	db := ApiGetDB(c)
	acc, err := db.lis.GetWallet().GetAccount(args.Src)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 104,
			Msg:  err.Error(),
		})
		return
	}
	mi := bi.NewMulTrans()
	mi.Acts = []*Account{acc}
	mi.Spent = bi.NextHeight()
	mi.Keep = args.Keep
	mi.Dst = []Address{args.Dst}
	mi.Amts = []Amount{amt}
	mi.Fee = fee
	mi.Ext = ext
	//创建未签名的交易
	tx, err := mi.NewTx(true)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 105,
			Msg:  err.Error(),
		})
		return
	}
	id, err := tx.ID()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 106,
			Msg:  err.Error(),
		})
		return
	}
	//广播交易
	mi.BroadTx(tx)
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  id.String(),
	})
}

//获取最后生成的10个区块
func listBestBlock(c *gin.Context) {
	type item struct {
		Id     string `json:"id"`
		Prev   string `json:"prev"`
		Time   string `json:"time"`
		Amount Amount `json:"amount"`
		Merkle string `json:"merkle"`
		Size   int    `json:"size"`
		Height uint32 `json:"height"`
	}
	type result struct {
		Code  int    `json:"code"`
		Items []item `json:"items"`
	}
	res := result{}
	bi := GetBlockIndex()
	iter := bi.NewIter()
	if !iter.Last() {
		c.JSON(http.StatusOK, res)
		return
	}
	for i := 0; iter.Prev() && i < 15; i++ {
		blk, err := bi.LoadBlock(iter.ID())
		if err != nil {
			panic(err)
		}
		ele := iter.Curr()
		i := item{}
		i.Id = iter.ID().String()
		i.Prev = ele.Prev.String()
		i.Time = time.Unix(int64(ele.Time), 0).Format("2006-01-02 15:04:05")
		amount, err := blk.GetIncome(bi)
		if err != nil {
			panic(err)
		}
		i.Amount = amount
		i.Size = ele.Blk.Len.ToInt()
		i.Height = ele.Height
		i.Merkle = ele.Merkle.String()
		res.Items = append(res.Items, i)
	}
	c.JSON(http.StatusOK, res)
}

func getBlockInfoApi(c *gin.Context) {
	bi := GetBlockIndex()
	ids := c.Param("id")
	if ids == "" {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  "ids error",
		})
		return
	}
	iter := bi.NewIter()
	has := false
	if len(ids) == 64 {
		has = iter.SeekID(NewHASH256(ids))
	} else if h, err := strconv.ParseInt(ids, 10, 32); err == nil {
		has = iter.SeekHeight(uint32(h))
	}
	if !has {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  ids + " not found",
		})
		return
	}
	ele := iter.Curr()
	blk, err := bi.LoadBlock(iter.ID())
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	income, err := blk.GetIncome(bi)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	type txin struct {
		OutTx    string `json:"otx"`
		OutIdx   int    `json:"oidx"`
		Addr     string `json:"addr,omitempty"`
		Amount   Amount `json:"amount,omitempty"`
		Script   string `json:"script,omitempty"`
		Sequence uint32 `json:"sequence"`
	}
	type txout struct {
		Addr   string `json:"addr"`
		Amount Amount `json:"amount"`
		Script string `json:"script"`
	}
	type tx struct {
		Id       string  `json:"id"`
		Ins      []txin  `json:"ins"`
		Outs     []txout `json:"outs"`
		Coinbase bool    `json:"coinbase"`
		LockTime uint32  `json:"lock_time"`
		Confirm  int     `json:"confirm"`
		Fee      Amount  `json:"fee"`
	}
	type block struct {
		Id      string `json:"id"`
		Prev    string `json:"prev"`
		Next    string `json:"next"`
		Height  uint32 `json:"height"`
		Ver     string `json:"ver"`
		Bits    string `json:"bits"`
		Size    uint32 `json:"size"`
		Nonce   string `json:"nonce"`
		Time    string `json:"time"`
		Merkle  string `json:"merkle"`
		Confirm int    `json:"confirm"`
		Income  Amount `json:"income"`
		Txs     []tx   `json:"txs"`
	}
	b := block{}
	b.Id = iter.ID().String()
	b.Prev = ele.Prev.String()
	b.Height = ele.Height
	b.Ver = fmt.Sprintf("0x%08x", ele.Ver)
	b.Bits = fmt.Sprintf("0x%08x", ele.Bits)
	b.Size = ele.Blk.Len.ToUInt32()
	b.Nonce = fmt.Sprintf("0x%08x", ele.Nonce)
	b.Time = time.Unix(int64(ele.Time), 0).Format("2006-01-02 15:04:05")
	b.Merkle = ele.Merkle.String()
	b.Income = income
	b.Confirm = bi.GetBlockConfirm(iter.ID())
	for _, v := range blk.Txs {
		xv := tx{}
		tid, err := v.ID()
		if err != nil {
			panic(err)
		}
		xv.Id = tid.String()
		xv.Coinbase = v.IsCoinBase()
		xv.Ins = []txin{}
		xv.Outs = []txout{}
		xv.Confirm = bi.GetTxConfirm(tid)
		xv.LockTime = v.LockTime
		fee, err := v.GetTransFee(bi)
		if err == nil {
			xv.Fee = fee
		}
		for _, iv := range v.Ins {
			xvi := txin{}
			xvi.Sequence = iv.Sequence
			xvi.Script = hex.EncodeToString(iv.Script)
			xvi.OutTx = iv.OutHash.String()
			xvi.OutIdx = iv.OutIndex.ToInt()
			if iv.IsCoinBase() {
				xv.Ins = append(xv.Ins, xvi)
				continue
			}
			ov, err := iv.LoadTxOut(bi)
			if err != nil {
				panic(err)
			}
			xvi.Amount = ov.Value
			addr, err := ov.Script.GetAddress()
			if err != nil {
				panic(err)
			}
			xvi.Addr = string(addr)
			xv.Ins = append(xv.Ins, xvi)
		}
		for _, ov := range v.Outs {
			xvo := txout{}
			xvo.Amount = ov.Value
			addr, err := ov.Script.GetAddress()
			if err != nil {
				panic(err)
			}
			xvo.Addr = string(addr)
			xvo.Script = hex.EncodeToString(ov.Script)
			xv.Outs = append(xv.Outs, xvo)
		}
		b.Txs = append(b.Txs, xv)
	}
	if iter.Next() && iter.Next() {
		b.Next = iter.ID().String()
	} else {
		b.Next = ZERO256.String()
	}
	c.JSON(http.StatusOK, b)
}

func listTxPoolApi(c *gin.Context) {
	bi := GetBlockIndex()
	txp := bi.GetTxPool()
	type txin struct {
		OutTx    string `json:"otx"`
		OutIdx   int    `json:"oidx"`
		Addr     string `json:"addr,omitempty"`
		Amount   Amount `json:"amount"`
		Script   string `json:"script,omitempty"`
		Sequence uint32 `json:"sequence"`
	}
	type txout struct {
		Addr   string `json:"addr"`
		Amount Amount `json:"amount"`
		Script string `json:"script"`
	}
	type tx struct {
		Id       string  `json:"id"`
		Ins      []txin  `json:"ins"`
		Outs     []txout `json:"outs"`
		Coinbase bool    `json:"coinbase"`
		LockTime uint32  `json:"lock_time"`
		Fee      Amount  `json:"fee"`
	}
	type result struct {
		Code int  `json:"code"`
		Txs  []tx `json:"txs"`
	}
	res := result{Txs: []tx{}}
	txs := txp.AllTxs()
	for _, tv := range txs {
		tid, err := tv.ID()
		if err != nil {
			panic(err)
		}
		xv := tx{}
		xv.Id = tid.String()
		xv.Coinbase = tv.IsCoinBase()
		xv.Ins = []txin{}
		xv.Outs = []txout{}
		xv.LockTime = tv.LockTime
		fee, err := tv.GetTransFee(bi)
		if err == nil {
			xv.Fee = fee
		}
		for _, iv := range tv.Ins {
			xvi := txin{}
			xvi.Sequence = iv.Sequence
			xvi.OutTx = iv.OutHash.String()
			xvi.OutIdx = iv.OutIndex.ToInt()
			xvi.Script = hex.EncodeToString(iv.Script)
			if iv.IsCoinBase() {
				xv.Ins = append(xv.Ins, xvi)
				continue
			}
			ov, err := iv.LoadTxOut(bi)
			if err != nil {
				panic(err)
			}
			xvi.Amount = ov.Value
			addr, err := ov.Script.GetAddress()
			if err != nil {
				panic(err)
			}
			xvi.Addr = string(addr)
			xv.Ins = append(xv.Ins, xvi)
		}
		for _, ov := range tv.Outs {
			xvo := txout{}
			xvo.Amount = ov.Value
			addr, err := ov.Script.GetAddress()
			if err != nil {
				panic(err)
			}
			xvo.Addr = string(addr)
			xvo.Script = hex.EncodeToString(ov.Script)
			xv.Outs = append(xv.Outs, xvo)
		}
		res.Txs = append(res.Txs, xv)
	}
	c.JSON(http.StatusOK, res)
}

func getTxInfoApi(c *gin.Context) {
	bi := GetBlockIndex()
	ids := c.Param("id")
	id := NewHASH256(ids)
	tp, err := bi.LoadTX(id)
	if err != nil {
		tp, err = bi.GetTxPool().Get(id)
	}
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  ids + " not found",
		})
		return
	}
	type txin struct {
		OutTx    string `json:"otx"`
		OutIdx   int    `json:"oidx"`
		Addr     string `json:"addr,omitempty"`
		Amount   Amount `json:"amount,omitempty"`
		Sequence uint32 `json:"sequence"`
		Script   string `json:"script,omitempty"`
	}
	type txout struct {
		Addr   string `json:"addr"`
		Amount Amount `json:"amount"`
		Script string `json:"script"`
	}
	type tx struct {
		Code     int     `json:"code"`
		Id       string  `json:"id"`
		Ins      []txin  `json:"ins"`
		Outs     []txout `json:"outs"`
		Coinbase bool    `json:"coinbase"`
		Pool     bool    `json:"pool"`
		LockTime uint32  `json:"lock_time"`
		Confirm  int     `json:"confirm"`
		Fee      Amount  `json:"fee"`
	}
	xv := tx{}
	tid, err := tp.ID()
	if err != nil {
		panic(err)
	}
	xv.Pool = tp.pool
	xv.Id = tid.String()
	xv.Coinbase = tp.IsCoinBase()
	xv.Ins = []txin{}
	xv.Outs = []txout{}
	xv.LockTime = tp.LockTime
	fee, err := tp.GetTransFee(bi)
	if err == nil {
		xv.Fee = fee
	}
	if !tp.pool {
		xv.Confirm = bi.GetTxConfirm(tid)
	} else {
		xv.Confirm = 0
	}
	for _, iv := range tp.Ins {
		xvi := txin{}
		xvi.Sequence = iv.Sequence
		xvi.OutTx = iv.OutHash.String()
		xvi.OutIdx = iv.OutIndex.ToInt()
		xvi.Script = hex.EncodeToString(iv.Script)
		if iv.IsCoinBase() {
			xv.Ins = append(xv.Ins, xvi)
			continue
		}
		ov, err := iv.LoadTxOut(bi)
		if err != nil {
			panic(err)
		}
		xvi.Amount = ov.Value
		addr, err := ov.Script.GetAddress()
		if err != nil {
			panic(err)
		}
		xvi.Addr = string(addr)
		xv.Ins = append(xv.Ins, xvi)
	}
	for _, ov := range tp.Outs {
		xvo := txout{}
		xvo.Amount = ov.Value
		addr, err := ov.Script.GetAddress()
		if err != nil {
			panic(err)
		}
		xvo.Addr = string(addr)
		xvo.Script = hex.EncodeToString(ov.Script)
		xv.Outs = append(xv.Outs, xvo)
	}
	c.JSON(http.StatusOK, xv)
}

func listAddrs(c *gin.Context) {
	type item struct {
		Ip   string `json:"ip"`
		Port int    `json:"port"`
	}
	type result struct {
		Code  int    `json:"code"`
		Addrs []item `json:"addrs"`
	}
	ds := Server.Addrs()
	res := result{Addrs: []item{}}
	for _, v := range ds {
		i := item{}
		i.Ip = v.addr.ip.String()
		i.Port = int(v.addr.port)
		res.Addrs = append(res.Addrs, i)
	}
	c.JSON(http.StatusOK, res)
}

func listClients(c *gin.Context) {
	type item struct {
		Ip      string `json:"ip"`
		Port    int    `json:"port"`
		Ver     uint32 `json:"ver"`
		Service uint32 `json:"service"`
		Ping    int    `json:"ping"`
		Id      string `json:"id"`
		Type    int    `json:"type"`
		Height  uint32 `json:"height"`
	}
	type result struct {
		Code int    `json:"code"`
		Ips  []item `json:"Ips"`
	}
	res := result{Ips: []item{}}
	cs := Server.Clients()
	for _, v := range cs {
		i := item{}
		i.Ip = v.Addr.ip.String()
		i.Port = int(v.Addr.port)
		i.Ver = v.Ver
		i.Service = v.Service
		i.Ping = v.ping
		i.Id = fmt.Sprintf("%x", v.id)
		i.Type = v.typ
		i.Height = v.Height
		res.Ips = append(res.Ips, i)
	}
	c.JSON(http.StatusOK, res)
}

//停止当前创建的区块
func stopBlockApi(c *gin.Context) {
	if Miner == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  "miner not running",
		})
		return
	}
	err := Miner.ResetMiner()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  "OK",
	})
}

//开始创建一个区块
func newBlockApi(c *gin.Context) {
	if Miner == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  "miner not running",
		})
		return
	}
	args := struct {
		Ver uint32 `form:"ver"` //矿工奖励地址
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	ps := GetPubSub()
	ps.Pub(MinerAct{
		Opt: OptGenBlock,
		Arg: args.Ver,
	}, NewMinerActTopic)
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  "OK",
	})
}

//获取矿工奖励地址
func getMinerApi(c *gin.Context) {
	if Miner == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  "miner not running",
		})
		return
	}
	acc := Miner.GetMiner()
	if acc == nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  "miner not set",
		})
		return
	}
	addr, err := acc.GetAddress()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  string(addr) + " " + acc.String(),
	})
}

//设置矿工账号
func setMinerApi(c *gin.Context) {
	args := struct {
		Addr Address `form:"addr"` //矿工奖励地址
	}{}
	if err := c.ShouldBind(&args); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	if err := args.Addr.Check(); err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	db := ApiGetDB(c)
	wallet := db.lis.GetWallet()
	acc, err := wallet.GetAccount(args.Addr)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 102,
			Msg:  err.Error(),
		})
		return
	}
	err = wallet.SetMiner(args.Addr)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 103,
			Msg:  err.Error(),
		})
		return
	}
	if Miner != nil {
		err = Miner.SetMiner(acc)
	}
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 104,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  "OK",
	})
}

//创建一个新账号
func newAccountApi(c *gin.Context) {
	args := struct {
		Num  uint8 `form:"num"`  //密钥数量
		Less uint8 `form:"less"` //至少需要的签名数量
		Arb  bool  `form:"arb"`  //是否启用仲裁
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
	addr, err := wallet.NewAccount(args.Num, args.Less, args.Arb)
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 101,
			Msg:  err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, ApiResult{
		Code: 0,
		Msg:  string(addr),
	})
}

func listAddressApi(c *gin.Context) {
	db := ApiGetDB(c)
	wallet := db.lis.GetWallet()
	addrs, err := wallet.ListAccount()
	if err != nil {
		c.JSON(http.StatusOK, ApiResult{
			Code: 100,
			Msg:  err.Error(),
		})
		return
	}
	type item struct {
		Addr   string `json:"addr"`
		Type   string `json:"type"`
		HasPri bool   `json:"haspri"`
	}
	type result struct {
		Code  int    `json:"code"`
		Addrs []item `json:"addrs"`
	}
	res := result{Code: 0}
	for _, v := range addrs {
		i := item{}
		i.Addr = string(v)
		acc, err := wallet.GetAccount(v)
		if err != nil {
			panic(err)
		}
		i.Type = acc.String()
		i.HasPri = acc.HasPrivate()
		res.Addrs = append(res.Addrs, i)
	}
	c.JSON(http.StatusOK, res)
}
