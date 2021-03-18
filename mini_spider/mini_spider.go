package mini_spider

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/html"
)

type Task struct {
	Depth uint32
	URL   string
}

type Spider struct {
	config *Config

	saver       *WebpageSaver
	parser      *WebpageParser
	urlRecorder *URLRecorder

	// 将被抓取的urls
	queue chan Task
}

func CreateSpider(config *Config) (*Spider, error) {
	saver, err := CreateWebpageSaver(config.TargetURL)
	if err != nil {
		return nil, err
	}

	spider := &Spider{
		config:      config,
		saver:       saver,
		parser:      CreateWebpageParser(),
		urlRecorder: CreateURLRecorder(),
		queue:       make(chan Task, 65535),
	}

	return spider, nil
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
	atomic.AddUint32(&e.produceURLCnt, uint32(cnt))
	return cnt
}

func (e *Extractor) getURLsFromHtmlNode(links *[]string, n *html.Node) {
	if n.Type == html.ElementNode {
		for _, a := range n.Attr {
			if a.Key == "href" {
				a.Val = strings.TrimSpace(a.Val)
				if len(a.Val) > 0 && a.Val[0] != '#' {
					*links = append(*links, a.Val)
				}
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
	for _, rawURL := range docsURLs {
		fixURL, err := urlMerge(fatherURL, rawURL)
		if err == nil {
			_, err = url.ParseRequestURI(fixURL)
		}
		if err != nil {
			err = fmt.Errorf("extract invalid (%s) + (%s) -> (%s): %s",
				fatherURL, rawURL, fixURL, err.Error())
			Error(err)
			continue
		}
		urls = append(urls, fixURL)
	}

	return urls, nil
}

func (e *Extractor) extractWebPage(url string) ([]byte, error) {
	timeout := time.Duration(time.Duration(GetConfig().CrawlTimeout) * time.Second)
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

func (e *Extractor) workMain(ctx context.Context, cancel context.CancelFunc) {
	for {
		select {
		case task := <-e.readyURLs:
			content, err := e.extractWebPage(task.URL)
			if err != nil {
				Error(fmt.Errorf("extract webpage(%s): %s", task.URL, err.Error()))
			} else {
				Info("extract webpage(%s) complete, get %d bytes", task.URL, len(content))
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
					} else {
						e.CreateTasks(task.Depth+1, urls)
					}
				}
			}

			atomic.AddUint32(&e.consumeURLCnt, 1)
			if atomic.LoadUint32(&e.consumeURLCnt) == atomic.LoadUint32(&e.produceURLCnt) {
				cancel()
			} else {
				time.Sleep(time.Duration(GetConfig().CrawlInterval) * time.Second)
			}

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
	for i := 0; i < int(3*threadCount); i++ {
		go e.workMain(ctx, cancel)
	}

	<-ctx.Done()
}
