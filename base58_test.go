package xginx

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"testing"
	"time"
)

var (
	testjson = `
[
	["", ""],
	["61", "2g"],
	["626262", "a3gV"],
	["636363", "aPEr"],
	["73696d706c792061206c6f6e6720737472696e67", "2cFupjhnEsSn59qHXstmK2ffpLv2"],
	["00eb15231dfceb60925886b67d065299925915aeb172c06647", "1NS17iag9jJgTHD1VXjvLCEnZuQ3rJDE9L"],
	["516b6fcd0f", "ABnLTmg"],
	["bf4f89001e670274dd", "3SEo3LWLoPntC"],
	["572e4794", "3EFU7m"],
	["ecac89cad93923c02321", "EJDM8drfXA6uyA"],
	["10c8511e", "Rt5zm"],
	["00000000000000000000", "1111111111"],
	["000111d38e5fc9071ffcd20b4a763cc9ae4f252bb4e48fd66a835e252ada93ff480d6dd43dc62a641155a5", "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"],
	["000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f404142434445464748494a4b4c4d4e4f505152535455565758595a5b5c5d5e5f606162636465666768696a6b6c6d6e6f707172737475767778797a7b7c7d7e7f808182838485868788898a8b8c8d8e8f909192939495969798999a9b9c9d9e9fa0a1a2a3a4a5a6a7a8a9aaabacadaeafb0b1b2b3b4b5b6b7b8b9babbbcbdbebfc0c1c2c3c4c5c6c7c8c9cacbcccdcecfd0d1d2d3d4d5d6d7d8d9dadbdcdddedfe0e1e2e3e4e5e6e7e8e9eaebecedeeeff0f1f2f3f4f5f6f7f8f9fafbfcfdfeff", "1cWB5HCBdLjAuqGGReWE3R3CguuwSjw6RHn39s2yuDRTS5NsBgNiFpWgAnEx6VQi8csexkgYw3mdYrMHr8x9i7aEwP8kZ7vccXWqKDvGv3u1GxFKPuAkn8JCPPGDMf3vMMnbzm6Nh9zh1gcNsMvH3ZNLmP5fSG6DGbbi2tuwMWPthr4boWwCxf7ewSgNQeacyozhKDDQQ1qL5fQFUW52QKUZDZ5fw3KXNQJMcNTcaB723LchjeKun7MuGW5qyCBZYzA1KjofN1gYBV3NqyhQJ3Ns746GNuf9N2pQPmHz4xpnSrrfCvy6TVVz5d4PdrjeshsWQwpZsZGzvbdAdN8MKV5QsBDY"]
]`
)

func TestBitconJsonScript(t *testing.T) {
	ret := [][]string{}
	err := json.Unmarshal([]byte(testjson), &ret)
	if err != nil {
		log.Println(ret)
		t.Errorf("unmarshal error %v", err)
	}
	for _, v := range ret {
		if len(v) != 2 {
			continue
		}
		d, err := hex.DecodeString(v[0])
		if err != nil {
			t.Errorf("decode v0 error %v", err)
		}
		x := B58Encode(d, BitcoinAlphabet)
		if x != v[1] {
			t.Errorf("b58 bitcoin encode error %s != %s", x, v[1])
		}
		dd, err := B58Decode(v[1], BitcoinAlphabet)
		if err != nil {
			t.Errorf("b58 bitcoin decode error %v , %s", err, v[1])
		}
		if hex.EncodeToString(dd) != v[0] {
			t.Errorf("b58 bitcoin decode error %s , %s", v[0], v[1])
		}
	}
}

func TestAlphabetImplStringer(t *testing.T) {
	// interface: Stringer {String()}
	alphabetStr := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	alphabet := NewAlphabet(alphabetStr)
	if alphabet.String() != alphabetStr {
		t.Errorf("alphabet.String() should be %s, but %s", alphabetStr, alphabet.String())
	}
}

func TestAlphabetFix58Length(t *testing.T) {
	crashed := false
	alphabetStr := "sfdjskdf"

	defer func() {
		if err := recover(); err != nil {
			crashed = true
		}
		if !crashed {
			t.Errorf("NewAlphabet(%s) should crash, but ok", alphabetStr)
		}
	}()

	NewAlphabet(alphabetStr)
}

func TestUnicodeAlphabet(t *testing.T) {
	myAlphabet := NewAlphabet("一二三四五六七八九十壹贰叁肆伍陆柒捌玖零拾佰仟万亿圆甲乙丙丁戊己庚辛壬癸子丑寅卯辰巳午未申酉戌亥金木水火土雷电风雨福")

	testCases := []struct {
		input  []byte
		should string
	}{
		{[]byte{0}, "一"},
		{[]byte{0, 0}, "一一"},
		{[]byte{1}, "二"},
		{[]byte{0, 1}, "一二"},
		{[]byte{1, 1}, "五圆"},
	}

	for _, testItem := range testCases {
		result := B58Encode(testItem.input, myAlphabet)
		if result != testItem.should {
			t.Errorf("encodeBase58(%v) should %s, but %s", testItem.input, testItem.should, result)
		}

		resultInput, err := B58Decode(testItem.should, myAlphabet)
		if err != nil {
			t.Errorf("decodeBase58(%s) error : %s", testItem.should, err.Error())
		} else if !bytes.Equal(resultInput, testItem.input) {
			t.Errorf("decodeBase58(%s) should %v, but %v", testItem.should, testItem.input, resultInput)
		}
	}
}

func TestRandCases(t *testing.T) {
	randSeed := time.Now().UnixNano()
	r := rand.New(rand.NewSource(randSeed))

	// generate 256 bytes
	testBytes := make([]byte, r.Intn(1000))
	for idx := range testBytes {
		testBytes[idx] = byte(r.Intn(256))
	}

	alphabet := BitcoinAlphabet
	redix58Bytes, _ := redixTrans256and58(testBytes, 256, 58)
	should58Runes := make([]rune, len(redix58Bytes))
	for idx, num := range redix58Bytes {
		should58Runes[idx] = alphabet.encodeTable[num]
	}

	logTag := fmt.Sprintf("rand[%d]", randSeed)
	// Encode
	calc58Str := B58Encode(testBytes, alphabet)
	if calc58Str != string(should58Runes) {
		t.Errorf("%s encodeBase58(%v) should %s, but %s", logTag, testBytes, string(should58Runes), calc58Str)
	}

	// Decode
	decodeBytes, err := B58Decode(string(should58Runes), alphabet)
	if err != nil {
		t.Errorf("%s decodeBase58(%s) error : %s", logTag, string(should58Runes), err.Error())
	} else if !bytes.Equal(decodeBytes, testBytes) {
		t.Errorf("%s decodeBase58(%s) should %v, but %v", logTag, string(should58Runes), testBytes, decodeBytes)
	}
}

// test: [0]: input bytes  [1]: encoded string
var testCases [][][]byte

func init() {
	caseCount := 1000000
	testCases = make([][][]byte, caseCount)
	for i := 0; i < caseCount; i++ {
		data := make([]byte, 32)
		rand.Read(data)
		testCases[i] = [][]byte{data, []byte(B58Encode(data, BitcoinAlphabet))}
	}
}

func BenchmarkEncode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		B58Encode([]byte(testCases[i%len(testCases)][0]), BitcoinAlphabet)
	}
}

func BenchmarkDecode(b *testing.B) {
	for i := 0; i < b.N; i++ {
		B58Decode(string(testCases[i%len(testCases)][1]), BitcoinAlphabet)
	}
}

////////////////////////////////////////////////////////////////////////////////
// redix trans
func redixTrans256and58(input []byte, fromRedix uint32, toRedix uint32) ([]byte, error) {
	capacity := int(math.Log(float64(fromRedix))/math.Log(float64(toRedix))) + 1

	zeros := 0
	for zeros < len(input) && input[zeros] == 0 {
		zeros++
	}

	output := make([]byte, 0, capacity)
	for inputPos := zeros; inputPos < len(input); inputPos++ {
		carry := uint32(input[inputPos])
		if carry >= fromRedix {
			return nil, fmt.Errorf("input[%d]=%d invalid for target redix(%d)", inputPos, carry, fromRedix)
		}
		for idx, num := range output {
			carry += fromRedix * uint32(num)
			output[idx] = byte(carry % uint32(toRedix))
			carry /= toRedix
		}

		for carry != 0 {
			output = append(output, byte(carry%toRedix))
			carry /= toRedix
		}

	}

	for i := 0; i < zeros; i++ {
		output = append(output, 0)
	}

	// reverse
	for idx := 0; idx < len(output)/2; idx++ {
		output[len(output)-idx-1], output[idx] = output[idx], output[len(output)-idx-1]
	}
	return output, nil
}

func TestTransRedix(t *testing.T) {
	for _, testItem := range []struct {
		num256 []byte
		num58  []byte
	}{
		{[]byte{0}, []byte{0}},
		{[]byte{0, 0}, []byte{0, 0}},
		{[]byte{0, 0, 0}, []byte{0, 0, 0}},
		{[]byte{1}, []byte{1}},
		{[]byte{57}, []byte{57}},
		{[]byte{58}, []byte{1, 0}},
		{[]byte{1, 0}, []byte{4, 24}},
		{[]byte{14, 239}, []byte{1, 7, 53}},
	} {
		calc58, err := redixTrans256and58(testItem.num256, 256, 58)
		if err != nil {
			t.Errorf("redix256to58: %v should be %v, but error %s", testItem.num256, testItem.num58, err.Error())
		} else if !bytes.Equal(calc58, testItem.num58) {
			t.Errorf("redix256to58: %v should be %v, but %v", testItem.num256, testItem.num58, calc58)
		}

		calc256, err := redixTrans256and58(testItem.num58, 58, 256)
		if err != nil {
			t.Errorf("redix58to256: %v should be %v, but error %s", testItem.num58, testItem.num256, err.Error())
		} else if !bytes.Equal(calc256, testItem.num256) {
			t.Errorf("redix58to256: %v should be %v, but %v", testItem.num58, testItem.num256, calc256)
		}
	}
}
