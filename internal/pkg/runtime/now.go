package runtime

import "time"

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

func ResetNowFn() {
	nowFn = defaultNow
}

func init() {
	ResetNowFn()
}
