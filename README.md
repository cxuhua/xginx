##xginx

###2019-11-16
#####1.添加了交易池，如果一笔金额在未确认的交易池中，下个交易引用将会失败
#####2.不能消费使用未确认的交易输出了，也就是说不能引用交易池中的交易输出