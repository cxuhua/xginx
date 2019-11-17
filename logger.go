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
	_ = log.Output(3, fmt.Sprintf(l+f+"\n", v...))
}

func LogInfo(v ...interface{}) {
	logger("[INFO]", v...)
}

func LogError(v ...interface{}) {
	logger("[ERROR]", v...)
}

func LogWarn(v ...interface{}) {
	logger("[WARN]", v...)
}

func LogInfof(f string, v ...interface{}) {
	loggerf("[INFO]", f, v...)
}

func LogErrorf(f string, v ...interface{}) {
	loggerf("[ERROR]", f, v...)
}

func LogWarnf(f string, v ...interface{}) {
	loggerf("[WARN]", f, v...)
}
