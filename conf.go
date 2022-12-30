package main

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

const (
	RssHubTwitterUrl   = "https://rsshub.rssforever.com/twitter"
	RssHubTelegramUrl  = "https://rsshub.rssforever.com/telegram/channel"
	ThirtyFivePhotoUrl = "https://rsshub.rssforever.com/35photo"
	RsshubDouyinUrl    = "https://rsshub.codefine.site:6870/douyin/user"
	DefaultDir         = "."

	Douyin             = "douyin"
	ThirtyFivePhotoRss = "35photo"
	TelegramChannelRss = "telegramchannel"
	WikiDailyPhotoRSS  = "wikidailyphotorss"
	DailyArt           = "dailyart"
	MMFan              = "mmfan"
	CNU                = "cnu"
	WallPaper          = "wallpaper"
)

func callerPrettyFieldForLogrus(caller *runtime.Frame) (string, string) {
	fileName := filepath.Base(caller.File)
	funcName := filepath.Base(caller.Function)
	return funcName, fmt.Sprintf("%s:%d", fileName, caller.Line)
}

type Account struct {
	Dir             string   `yaml:"dir"`
	Seeds           []string `yaml:"seeds"`
	Type            string   `yaml:"type"`
	NoDesc          bool     `yaml:"no_desc"`
	NoDate          bool     `yaml:"no_date"`
	Url             string   `yaml:"url"`
	FeedUseProxy    bool     `yaml:"feed_use_proxy"`
	ContentUseProxy bool     `yaml:"content_use_proxy"`
}

type HttpProxy struct {
	UseProxy bool   `yaml:"use_proxy"`
	Host     string `yaml:"host"`
	Port     uint16 `yaml:"port"`
	Protocol string `yaml:"protocol"`
	User     string `yaml:"user"`
	Password string `yaml:"password"`
}

type Conf struct {
	Accounts  []Account `yaml:"accounts"`
	Proxy     HttpProxy `yaml:"http_proxy"`
	RssHubUrl string    `yaml:"rsshub_url"`
	PhotoDir  string    `yaml:"photo_dir"`
}

func formatConf(fPath string) error {
	c, err := getConf(fPath)
	if err != nil {
		return err
	}

	for _, a := range c.Accounts {
		sort.Slice(a.Seeds, func(i, j int) bool {
			return a.Seeds[i] < a.Seeds[j]
		})
	}

	o, err := yaml.Marshal(c)
	if err != nil {
		return err
	}

	fmt.Println(string(o))

	f, err := os.OpenFile(fPath+".tmp", os.O_WRONLY|os.O_CREATE, 0755)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	fe := yaml.NewEncoder(f)
	fe.SetIndent(2)
	err = fe.Encode(c)
	if err != nil {
		return err
	}
	_ = fe.Close()

	err = os.Rename(fPath+".tmp", fPath)
	if err != nil {
		return err
	}

	return nil
}

func getConf(fPath string) (*Conf, error) {
	c := Conf{}

	f, err := os.OpenFile(fPath, os.O_RDONLY, 0755)
	if err != nil {
		return nil, err
	}
	defer func(f *os.File) {
		_ = f.Close()
	}(f)

	d, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(d, &c)
	if err != nil {
		return nil, err
	}

	return &c, nil
}
