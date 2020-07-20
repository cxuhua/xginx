package xginx

import (
	"fmt"
	lru "github.com/hashicorp/golang-lru"
	"github.com/stretchr/testify/require"
	"log"
	"math/rand"
	"testing"
	"time"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/syndtr/goleveldb/leveldb/filter"
)

func TestLRUCache(t *testing.T) {
	c ,err:= lru.New(4096)
	if err != nil {
		require.NoError(t,err)
	}
	id1 := HASH256{1}
	id11 := HASH256{1}
	id2 := HASH256{2}
	c.Add(id1,"1111")
	v,ok := c.Get(id11)
	require.True(t,ok)
	require.Equal(t,v.(string),"1111")
	v,ok = c.Get(id2)
	require.False(t,ok)
	require.Nil(t,v)
}

func TestBloom(t *testing.T) {
	b := filter.NewBloomFilter(10)
	log.Println(b.Name())
	b.NewGenerator()
}

func TestSentine(t *testing.T) {

	// 务必先进行初始化
	err := sentinel.InitDefault()
	if err != nil {
		log.Fatal(err)
	}

	// 配置一条限流规则
	_, err = flow.LoadRules([]*flow.FlowRule{
		{
			Resource:        "/api/users/123",
			MetricType:      flow.QPS,
			Count:           10,
			ControlBehavior: flow.Reject,
		},
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	ch := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			for {
				// 埋点逻辑，埋点资源名为 some-test
				e, b := sentinel.Entry("/api/users/123",
					sentinel.WithResourceType(base.ResTypeWeb),
				)
				if b != nil {
					// 请求被拒绝，在此处进行处理
					time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)
				} else {
					// 请求允许通过，此处编写业务逻辑
					fmt.Println(util.CurrentTimeMillis(), "Passed")
					time.Sleep(time.Duration(rand.Uint64()%10) * time.Millisecond)

					// 务必保证业务结束后调用 Exit
					e.Exit()
				}

			}
		}()
	}
	<-ch
}
