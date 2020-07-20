#xginx by cxuhua@qq.com
=====================================
如何开始:

//编译
cd server
go build -o main

修改配置文件:v1000.json

"data_dir": "数据存储目录",

"tcp_ip": "服务器对外ip地址",

"log_file": "日志文件路径",

"max_conn": 50,//最大连接

启动: ./main -conf=配置文件路径 -debug=true|false(是否处于开发模式)

更新日志
=====================================
2019-12-13
----------------
1.舍弃http服务

2.重新设计了测试区块

2019-12-7
----------------
1.交易池添加引用交易索引，账户相关索引

2.增强输入sequence开始支持支付通道

2019-12-4
----------------
1.添加txin seq字段，可以覆盖交易池中的交易，需要重新生成区块

2019-12-3
----------------
1.添加时间戳获取方法，可自定义当前时间戳

2019-12-2
----------------
1.添加了数据包广播头和响应支持

2019-11-30
----------------
1.交易池中检测失败的将会被移除

2019-11-29
----------------
1.添加了ReadFull和WriteFull防止网络数据读写不完整

2.修正了存在交易费计算merkle的错误

2019-11-27
----------------
1.加强MsgGetTxPool消息同步两节点的交易池

2.设计了新的支持多线程的hash方法进行挖矿

2019-11-26
----------------
1.交易添加locktime属性表示交易锁定时间，实现在某个高度或者时间交易才能加入打包进区块

2.可以消费交易池中的输出，但是只能在同一个区块中，并且输出必须在之前的交易中

2019-11-25
----------------
1.填加钱包账号导出和导入接口，允许选择是否导出私钥

2019-11-23
----------------
1.今天挖出了第一个区块,000000007be7626a14398ee03706080d33ac07bb18bb82254331ff6191f8c850

2.添加了区块信息http接口

2019-11-22
----------------
1.重新设计了bloom过滤器，代码来自BTC项目

2019-11-21
----------------
1.当生成新区块时先发布区块头到周围节点，周围节点再根据头下载区块数据

2.添加基于leveldb的bloom过滤器,线程安全


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