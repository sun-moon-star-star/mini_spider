package main

import (
	"main/mini_spider"
	"runtime"
	"time"
)

func main() {
	// 开始时间
	begin_time := time.Now().UnixNano()
	// 加载配置
	mini_spider.LoadConfig()
	// 设置最大的线程数
	runtime.GOMAXPROCS(runtime.NumCPU() * 2)
	// 初始化Extractor
	mini_spider.InitExtractor(655360000)
	// 初始化urls（包括去重）
	mini_spider.GetExtractor().CreateTasks(0, mini_spider.GetConfig().InitialUrlList)
	// 开始抓取
	mini_spider.GetExtractor().Main(mini_spider.GetConfig().ThreadCount)
	// 结束时间
	end_time := time.Now().UnixNano()
	mini_spider.Info("extract %d web pages, save %d web pages, cost time %d ms",
		mini_spider.GetExtractor().RecordURLsCnt(),
		mini_spider.GetExtractor().SaveURLCnt(),
		(end_time-begin_time)/1e6)
	// 日志资源清理
	mini_spider.LoggerClose()

}
