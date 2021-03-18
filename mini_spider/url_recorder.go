package mini_spider

import (
	"sync"
	"sync/atomic"
)

const (
	URL_RECORD_PENDING  = 0
	URL_RECORD_COMPLETE = 1
)

type URLRecorder struct {
	produceCnt uint32
	consumeCnt uint32
	// 抓取的urls集合
	recordURLs sync.Map
}

func CreateURLRecorder() *URLRecorder {
	return &URLRecorder{}
}

func (ur *URLRecorder) Exists(rawURL string) bool {
	_, exist := ur.recordURLs.Load(rawURL)
	return exist
}

func (ur *URLRecorder) Produce(rawURL string) uint32 {
	_, exist := ur.recordURLs.LoadOrStore(rawURL, URL_RECORD_PENDING)
	if exist {
		return 0
	}
	atomic.AddUint32(&ur.produceCnt, 1)
	return 1
}

func (ur *URLRecorder) Consume(rawURL string) uint32 {
	value, exist := ur.recordURLs.Load(rawURL)

	if exist && value == URL_RECORD_PENDING {
		ur.recordURLs.Store(rawURL, URL_RECORD_COMPLETE)
		atomic.AddUint32(&ur.consumeCnt, 1)
		return 1
	}

	return 0
}

func (ur *URLRecorder) AllURLsComplete() bool {
	return atomic.LoadUint32(&ur.consumeCnt) == atomic.LoadUint32(&ur.produceCnt)
}

func (ur *URLRecorder) Total() uint32 {
	return atomic.LoadUint32(&ur.produceCnt)
}
