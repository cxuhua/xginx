xginx
=====================================

2019-11-20
----------------
1.基本解决分叉问题，按照链高胜出为原则,并且需要提供区块头列表作为证据

2019-11-19
----------------
1.锁定脚本可以增加自自定义数据，但大小不能超过4K

2.尚未解决链分叉的情况

2019-11-18
----------------
1.添加了区块链迭代器，废弃链上的迭代方法

2019-11-17
----------------
1.添加了日志输出方法,LogInfo(f),LogError(f),LogWarn(f)

2.重新修改了数据编码解码方式

2019-11-16
----------------
1.添加了交易池，如果一笔金额在未确认的交易池中，下个交易引用将会失败

2.不能消费使用未确认的交易输出了，也就是说不能引用交易池中的交易输出