package xginx

import (
	"fmt"
	"log"
)

//日志处理
func logger(l string, v ...interface{}) {
	vs := append([]interface{}{l}, v...)
	_ = log.Output(3, fmt.Sprintln(vs...))
}

func loggerf(l string, f string, v ...interface{}) {
	_ = log.Output(3, fmt.Sprintf(l+" "+f+"\n", v...))
}

//LogInfo 信息日志
func LogInfo(v ...interface{}) {
	logger("[INFO]", v...)
}

//LogError 错误日志
func LogError(v ...interface{}) {
	logger("[ERROR]", v...)
}

//LogWarn 警告日志
func LogWarn(v ...interface{}) {
	logger("[WARN]", v...)
}

//LogInfof 信息日志带格式化
func LogInfof(f string, v ...interface{}) {
	loggerf("[INFO]", f, v...)
}

//LogErrorf 错误日志带格式化
func LogErrorf(f string, v ...interface{}) {
	loggerf("[ERROR]", f, v...)
}

//LogWarnf 警告日志带格式化
func LogWarnf(f string, v ...interface{}) {
	loggerf("[WARN]", f, v...)
}
