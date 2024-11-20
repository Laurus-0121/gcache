package geecache

import (
	"fmt"
	"runtime"
	"strings"
)

// 显示错误时运行栈堆
func trace(errorMessage string) string {
	var pctack [32]uintptr
	n := runtime.Callers(3, pctack[:])

	var str strings.Builder
	str.WriteString(errorMessage + "\nTraceback: ")
	for _, pc := range pctack[:n] {
		function := runtime.FuncForPC(pc)
		file, line := function.FileLine(pc)
		str.WriteString(fmt.Sprintf("\n\t%s:%d", file, line))
	}
	return str.String()
}

func validPeerAddr(addr string) bool {
	token1 := strings.Split(addr, ":")
	if len(token1) != 2 {
		return false
	}
	token2 := strings.Split(token1[0], ".")
	if token1[0] != "localhost" && len(token2) != 4 {
		return false
	}
	return true
}
