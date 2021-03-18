package mini_spider

import (
	"bufio"
	"net/url"
	"os"
	"regexp"
	"sync/atomic"
)

type WebpageSaver struct {
	// 真正存储的文件的个数
	cnt uint32
	// 需要存储的目标网页URL pattern(正则表达式)
	urlRegex *regexp.Regexp
}

func CreateWebpageSaver(targetURL string) (*WebpageSaver, error) {
	urlRegex, err := regexp.Compile(targetURL)

	if err != nil {
		return nil, err
	}

	return &WebpageSaver{
		urlRegex: urlRegex,
	}, nil
}

func urlEncode(rawURL string) string {
	return url.QueryEscape(rawURL)
}

func (ws *WebpageSaver) SaveToFile(outputDirectory, url string, content []byte) (int, error) {
	filename := urlEncode(url)
	filePath := outputDirectory + "/" + filename

	file, err := os.Create(filePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	n, err := bufio.NewWriter(file).Write(content)
	if err != nil {
		return 0, err
	}

	atomic.AddUint32(&ws.cnt, 1)
	return n, nil
}

func (ws *WebpageSaver) Total() uint32 {
	return atomic.LoadUint32(&ws.cnt)
}
