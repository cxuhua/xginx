package xginx

//FLocker 文件锁多平台实现
type FLocker interface {
	Lock() error
	Release() error
}
