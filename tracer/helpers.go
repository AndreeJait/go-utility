package tracer

import (
	"reflect"
	"runtime"
	"strings"
)

func GetFuncName(fn any) string {
	if fn == nil {
		return "unknown"
	}
	f := runtime.FuncForPC(reflect.ValueOf(fn).Pointer())
	if f == nil {
		return "unknown"
	}
	return f.Name()
}

func ShortFuncName(fn any) string {
	full := GetFuncName(fn)
	if i := strings.LastIndex(full, "/"); i >= 0 {
		full = full[i+1:]
	}
	return full
}
