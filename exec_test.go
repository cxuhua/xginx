package xginx

import (
	"log"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func init() {
	//测试模式下开启
	*IsDebug = true

	DefaultTxScript = []byte(`
	return true
`)

	DefaultInputScript = []byte(`
	map_set("a",11)
	return true
`)

	DefaultLockedScript = []byte(`
	return verify_addr() and verify_sign() and map_get("a") == 11;
`)

}

func TestLimitStep(t *testing.T) {
	step := uint32(100)
	timev := uint32(200)
	limit := PackExeLimit(step, timev)
	step1, time1 := ParseExeLimit(limit)
	assert.Equal(t, step1, step)
	assert.Equal(t, time1, timev)
	step2, time2 := GetExeLimit(limit)
	assert.Equal(t, time2, time.Microsecond*200)
	assert.Equal(t, uint32(step2), step)
}

func TestFloatVal(t *testing.T) {
	v := float64(1.000000000)
	i, b := math.Modf(v)
	log.Println(i, b == 0)
}

func TestCheckScript(t *testing.T) {
	err := CheckScript(DefaultInputScript)
	if err != nil {
		t.Fatal(err)
	}
	err = CheckScript([]byte(`&763743`))
	if err == nil {
		t.Fatal("error script ")
	}
}
