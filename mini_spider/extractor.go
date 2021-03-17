package mini_spider

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

type Task struct {
	Depth uint32
	URL   string
}

type Extractor struct {
	// 抓取的URL的个数
	recordURLCnt uint32
	// 存储的URL的个数
	saveURLCnt uint32
	// 正在extracting的Groutine数量
	extractingGroutineCnt int32
	// 需要存储的目标网页URL pattern(正则表达式)
	saveFileURLRegex *regexp.Regexp
	// 抓取的urls集合
	recordURLs sync.Map
	// 将被抓取的urls
	readyURLs chan Task
}

var extractor Extractor

func InitExtractor(size uint32) {
	extractor.init(size)
}

func GetExtractor() *Extractor {
	return &extractor
}

func urlEncode(rawURL string) string {
	return url.QueryEscape(rawURL)
}

func (e *Extractor) init(size uint32) {
	e.readyURLs = make(chan Task, size)

	var err error
	e.saveFileURLRegex, err = regexp.Compile(GetConfig().TargetURL)
	if err != nil {
		err = fmt.Errorf("compile targetURL regexp(%s): %s", GetConfig().TargetURL, err.Error())
		panic(Error(err))
	}
}

func (e *Extractor) RecordURLsCnt() uint32 {
	return atomic.LoadUint32(&e.recordURLCnt)
}

func urlMerge(baseUrl, currUrl string) string {
	urlInfo, err := url.Parse(currUrl)

	if err != nil || urlInfo.Scheme != "" {
		return ""
	}

	baseInfo, err := url.Parse(baseUrl)
	if err != nil {
		return ""
	}

	var urlMerge string
	if urlInfo.Host != "" {
		urlMerge = baseInfo.Scheme + "://" + urlInfo.Host
	} else {
		urlMerge = baseInfo.Scheme + "://" + baseInfo.Host
	}

	var path string
	if strings.Index(urlInfo.Path, "/") == 0 {
		path = urlInfo.Path
	} else {
		path = filepath.Dir(baseInfo.Path) + "/" + urlInfo.Path
	}

	rst := make([]string, 0)
	pathArr := strings.Split(path, "/")

	// 如果path是已/开头，那在rst加入一个空元素
	if pathArr[0] == "" {
		rst = append(rst, "")
	}
	for _, p := range pathArr {
		if p == ".." {
			if rst[len(rst)-1] == ".." {
				rst = append(rst, "..")
			} else {
				rst = rst[:len(rst)-1]
			}
		} else if p != "" && p != "." {
			rst = append(rst, p)
		}
	}

	urlMerge = urlMerge + strings.Join(rst, "/") + urlInfo.RawQuery
	Info("url merge %s + %s -> %s", baseUrl, currUrl, urlMerge)
	return urlMerge
}

func (e *Extractor) CreateTask(depth uint32, url string) int32 {
	_, exist := e.recordURLs.LoadOrStore(url, nil)

	if exist {
		return 0
	}

	if depth == GetConfig().MaxDepth && !e.saveFileURLRegex.MatchString(url) {
		return 0
	}

	e.readyURLs <- Task{Depth: depth, URL: url}

	return 1
}

func (e *Extractor) CreateTasks(depth uint32, urls []string) (cnt int32) {
	for _, url := range urls {
		cnt += e.CreateTask(depth, url)
	}
	return cnt
}

func (e *Extractor) getURLsFromHtmlNode(links *[]string, n *html.Node) {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "href" {
				*links = append(*links, a.Val)
			}
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		e.getURLsFromHtmlNode(links, c)
	}
}

func (e *Extractor) extractURLs(fatherURL string, content []byte) (urls []string, err error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	var docsURLs []string
	e.getURLsFromHtmlNode(&docsURLs, doc)

	// URL 填充
	for _, value := range docsURLs {
		if len(value) == 0 {
			continue
		}
		if value[0] == '.' || value[0] == '/' {
			value = urlMerge(fatherURL, value)
		}
		_, err := url.ParseRequestURI(value)
		if err != nil {
			err = fmt.Errorf("extract invalid url(%s) from father url (%s): %s",
				value, fatherURL, err.Error())
			Error(err)
			continue
		}
		urls = append(urls, value)
	}

	return urls, nil
}

func (e *Extractor) extractWebPage(url string) ([]byte, error) {
	timeout := time.Duration(1 * time.Second)
	client := http.Client{
		Timeout: timeout,
	}
	resp, err := client.Get(url)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (e *Extractor) saveToFile(url string, content []byte) error {
	filename := urlEncode(url)

	file, err := os.Create(GetConfig().OutputDirectory + "/" + filename)
	if err != nil {
		return err
	}
	defer file.Close()

	w := bufio.NewWriter(file)

	n, err := w.Write(content)

	if err != nil {
		return err
	}

	Info("save file(%s): %d bytes", url, n)

	return nil
}

func (e *Extractor) workMain(ctx context.Context, cancel context.CancelFunc) {
	for {
		select {
		case task := <-e.readyURLs:
			atomic.AddInt32(&e.extractingGroutineCnt, 1)
			atomic.AddUint32(&e.recordURLCnt, 1)

			Info("try to extract url %s", task.URL)
			content, err := e.extractWebPage(task.URL)
			if err != nil {
				Error(fmt.Errorf("extract webpage(%s): %s", task.URL, err.Error()))
				continue
			}

			Info("extract url %s complete, get %d bytes", task.URL, len(content))
			if e.saveFileURLRegex.MatchString(task.URL) {
				err = e.saveToFile(task.URL, content)
				if err != nil {
					Error(fmt.Errorf("save file(%s): %s", task.URL, err.Error()))
				}
			}

			if task.Depth != GetConfig().MaxDepth {
				urls, err := e.extractURLs(task.URL, content)
				if err != nil {
					Error(fmt.Errorf("extract urls from %s content: %s", task.URL, err.Error()))
					continue
				}

				e.CreateTasks(task.Depth+1, urls)
			}

			if len(e.readyURLs) == 0 && atomic.LoadInt32(&e.extractingGroutineCnt) == 1 {
				cancel()
			}
			atomic.AddInt32(&e.extractingGroutineCnt, -1)
		case <-ctx.Done():
			return
		}
	}
}

func (e *Extractor) Main(threadCount uint32) {
	if len(e.readyURLs) == 0 {
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	for i := 0; i < int(threadCount); i++ {
		go e.workMain(ctx, cancel)
	}

	<-ctx.Done()
}
