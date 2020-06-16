package xginx

import (
	"log"
	"sync"
	"testing"

	sentinel "github.com/alibaba/sentinel-golang/api"
	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/core/flow"
	"github.com/syndtr/goleveldb/leveldb/filter"
)

func TestBloom(t *testing.T) {
	b := filter.NewBloomFilter(10)
	log.Println(b.Name())
	b.NewGenerator()
}

func TestSentine(t *testing.T) {
	sentinel.InitDefault()
	_, err := flow.LoadRules([]*flow.FlowRule{
		{
			ID:                1,
			Resource:          "name111",
			MetricType:        flow.QPS,
			Count:             15,
			ControlBehavior:   flow.Throttling,
			MaxQueueingTimeMs: 5000,
		},
	})
	if err != nil {
		// 加载规则失败，进行相关处理
	}
	num := 20
	wg := sync.WaitGroup{}
	wg.Add(num)
	for i := 0; i < num; i++ {
		go func(idx int) {
			e, err := sentinel.Entry("name111", sentinel.WithTrafficType(base.Inbound))
			if err != nil {
				log.Println(idx, err)
			} else {
				log.Println(idx, "pass")
				e.Exit()
			}
			wg.Done()
		}(i)
	}
	wg.Wait()
}
