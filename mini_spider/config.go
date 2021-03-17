package mini_spider

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type Config struct {
	// 最大抓取深度(种子为0级)
	MaxDepth uint32
	// 抓取间隔. 单位: 秒
	CrawlInterval uint32
	// 抓取超时. 单位: 秒
	CrawlTimeout uint32
	// 抓取routine数
	ThreadCount uint32
	// 种子文件路径
	URLListFile string
	// 抓取结果存储目录
	OutputDirectory string
	// 需要存储的目标网页URL pattern(正则表达式)
	TargetURL string
	// 日志记录目录
	LogPath string
	// 配置文件目录
	ConfFile string
	// 初始化url列表
	InitialUrlList []string
}

var config Config

var rootCmd = &cobra.Command{
	Use:     "mini_spider",
	Short:   "A Mini Spider(Web Crawler) System in Golang.",
	Version: "1.0.0",
	Example: "./mini_spider -c ./conf -l ./log",
	Run: func(cmd *cobra.Command, args []string) {
	},
}

func LoadConfig() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVarP(&config.ConfFile, "conf_file", "c", "./conf/spider.ini", "config file (only ini)")
	rootCmd.PersistentFlags().StringVarP(&config.LogPath, "log_path", "l", "./log/", "log path")
}

/*
文件格式如下:
[
	"{item-1}",
	"{item-2}",
	...
]
*/
func readStringArrayFromJsonFile(filePath string) ([]string, error) {
	file, err := os.Open(filePath)

	if err != nil {
		return nil, err
	}

	defer file.Close()

	content, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var arr []string
	err = json.Unmarshal(content, &arr)
	if err != nil {
		return nil, err
	}

	return arr, nil
}

func checkConfig() error {
	if config.ThreadCount == 0 {
		return errors.New("threadCount must be greater than zero")
	}

	if config.OutputDirectory == "" {
		return errors.New("outputDirectory cannot be empty")
	}
	return nil
}

func initConfig() {
	viper.SetConfigType("ini")
	viper.SetConfigFile(config.ConfFile)

	if err := viper.ReadInConfig(); err != nil {
		err = fmt.Errorf("read in config(%s): %s", config.ConfFile, err.Error())
		panic(Error(err))
	}

	config.MaxDepth = viper.GetUint32("spider.maxDepth")
	config.CrawlInterval = viper.GetUint32("spider.crawlInterval")
	config.CrawlTimeout = viper.GetUint32("spider.crawlTimeout")
	config.ThreadCount = viper.GetUint32("spider.threadCount")
	config.URLListFile = viper.GetString("spider.urlListFile")
	config.OutputDirectory = viper.GetString("spider.outputDirectory")
	config.TargetURL = viper.GetString("spider.TargetURL")

	var err error
	config.InitialUrlList, err = readStringArrayFromJsonFile(config.URLListFile)
	if err != nil {
		err = fmt.Errorf("initial urllist(%s): %s", config.URLListFile, err.Error())
		panic(Error(err))
	}

	err = checkConfig()
	if err != nil {
		err = fmt.Errorf("check config(%+v): %s", config, err.Error())
		panic(Error(err))
	}

	Info("config(%s): %+v", config.ConfFile, config)
}

func GetConfig() *Config {
	return &config
}
