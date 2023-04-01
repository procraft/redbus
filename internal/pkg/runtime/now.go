package runtime

import (
	"fmt"
	"time"
)

type NowFn func() time.Time

var defaultNow = func() time.Time {
	return time.Now()
}

var nowFn NowFn

func Now() time.Time {
	return nowFn()
}

func SetNowFn(fn NowFn) {
	nowFn = fn
}

func SetStatic(str string) {
	nowFn = func() time.Time {
		t, err := time.Parse(time.RFC3339, str)
		if err != nil {
			panic(fmt.Sprintf("Can't parse date string %v: %v", str, err))
		}
		return t
	}
}

func ResetNowFn() {
	nowFn = defaultNow
}

func init() {
	ResetNowFn()
}
