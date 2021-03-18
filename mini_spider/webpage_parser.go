package mini_spider

import (
	"bytes"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
)

type WebpageParser struct{}

func CreateWebpageParser() *WebpageParser {
	return &WebpageParser{}
}

func (wp *WebpageParser) fixURL(rawURL string) string {
	fixedURL := strings.TrimSpace(rawURL)
	fixedURL = strings.TrimLeft(fixedURL, "\\")
	return fixedURL
}

func (wp *WebpageParser) isValidURL(rawURL string) bool {
	if len(rawURL) == 0 || rawURL[0] != '#' {
		return false
	}
	_, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return false
	}
	return true
}

func (wp *WebpageParser) getURLsFromHtmlNodeItem(links *[]string, node *html.Node) {
	if node.Type != html.ElementNode {
		return
	}

	for _, a := range node.Attr {
		if a.Key == "href" {
			if fixedURL := wp.fixURL(a.Val); wp.isValidURL(fixedURL) {
				*links = append(*links, fixedURL)
			}
		}
	}
}

func (wp *WebpageParser) getURLsFromHtmlNodeChilren(links *[]string, node *html.Node) {
	for childNode := node.FirstChild; childNode != nil; childNode = childNode.NextSibling {
		wp.getURLsFromHtmlNode(links, childNode)
	}
}

func (wp *WebpageParser) getURLsFromHtmlNode(links *[]string, node *html.Node) {
	wp.getURLsFromHtmlNodeItem(links, node)
	wp.getURLsFromHtmlNodeChilren(links, node)
}

func (wp *WebpageParser) isCompleteURL(urlInfo *url.URL) bool {
	return urlInfo.Scheme != ""
}

func (wp *WebpageParser) urlMerge(baseURL, currURL string) (string, error) {
	urlInfo, err := url.Parse(currURL)
	if err != nil {
		return "", err
	}

	if wp.isCompleteURL(urlInfo) {
		return currURL, nil
	}

	baseInfo, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}

	if !wp.isCompleteURL(baseInfo) {
		return "", fmt.Errorf("baseURL is not complete when url merge")
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

	if pathArr[0] == "" {
		rst = append(rst, "")
	}

	for _, p := range pathArr {
		if p == ".." {
			if len(rst) > 0 {
				if rst[len(rst)-1] == ".." {
					rst = append(rst, "..")
				} else {
					rst = rst[:len(rst)-1]
				}
			}
		} else if p != "" && p != "." {
			rst = append(rst, p)
		}
	}

	urlMerge = urlMerge + strings.Join(rst, "/") + urlInfo.RawQuery
	Info("url merge %s + %s -> %s", baseURL, currURL, urlMerge)
	return urlMerge, nil
}

func (wp *WebpageParser) completeURLs(fatherURL string, rawURLS []string) (urls []string) {
	for _, rawURL := range rawURLS {
		fixURL, err := wp.urlMerge(fatherURL, rawURL)

		if err != nil {
			Warn("extract invalid (%s) + (%s) -> (%s): %s", fatherURL, rawURL, fixURL, err.Error())
		} else {
			urls = append(urls, fixURL)
		}
	}

	return urls
}

func (wp *WebpageParser) ExtractURLsFromWebpage(fatherURL string, content []byte) (urls []string, err error) {
	doc, err := html.Parse(bytes.NewReader(content))
	if err != nil {
		return nil, err
	}

	wp.getURLsFromHtmlNode(&urls, doc)

	return wp.completeURLs(fatherURL, urls), nil
}
