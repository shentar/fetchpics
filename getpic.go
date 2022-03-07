package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"github.com/dsoprea/go-exif/v3"
	exifcommon "github.com/dsoprea/go-exif/v3/common"
	"github.com/dsoprea/go-exif/v3/undefined"
	jpgs "github.com/dsoprea/go-jpeg-image-structure/v2"
	pngs "github.com/dsoprea/go-png-image-structure/v2"
	"github.com/dsoprea/go-utility/v2/image"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	RssHubTwitterUrl   = "https://rsshub.rssforever.com/twitter"
	RssHubTelegramUrl  = "https://rsshub.rssforever.com/telegram/channel"
	DefaultDir         = "."
	TelegramChannelRss = "telegramchannel"
)

type oneItem struct {
	url  string
	desc string
}

type Account struct {
	Dir    string   `yaml:"dir"`
	Seeds  []string `yaml:"seeds"`
	Type   string   `yaml:"type"`
	NoDesc bool     `yaml:"no_desc"`
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

func callerPrettyfierForLogrus(caller *runtime.Frame) (string, string) {
	fileName := filepath.Base(caller.File)
	funcName := filepath.Base(caller.Function)
	return funcName, fmt.Sprintf("%s:%d", fileName, caller.Line)
}

var (
	config    *Conf
	rootDir   string
	libPicDir string
	photoDir  string
	dateStr   string
)

type parser func(string, string) []*oneItem

func parseOneTwitterItem(des, seed string) []*oneItem {
	reg := regexp.MustCompile(`<img style.*? src="(.*?=orig)"+`)
	ss := reg.FindAllStringSubmatch(des, -1)
	if len(ss) == 0 {
		return nil
	}

	var items []*oneItem
	for _, j := range ss {
		if len(j) == 2 {
			urlDecoded := strings.Replace(j[1], "&amp;", "&", -1)
			it := &oneItem{url: urlDecoded, desc: des + fmt.Sprintf("@%s", seed)}

			items = append(items, it)
		}
	}

	return items
}

func parseOneTelegramItem(des, seed string) []*oneItem {
	reg := regexp.MustCompile(`<img src="(.*?\.(jpg|png))" referrerpolicy="no-referrer">+`)
	ss := reg.FindAllStringSubmatch(des, -1)
	if len(ss) == 0 {
		return nil
	}

	var items []*oneItem
	for _, j := range ss {
		if len(j) == 3 {
			urlDecoded := strings.Replace(j[1], "&amp;", "&", -1)
			it := &oneItem{url: urlDecoded, desc: des + fmt.Sprintf("@%s", seed)}

			items = append(items, it)
		}
	}

	return items
}

type OneUser struct {
	account   string
	folder    string
	rsshubUrl string
	parser    parser
	noDesc    bool
	client    *http.Client
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "format" {
		err := formatConf(os.Args[2])
		if err != nil {
			fmt.Println(err.Error())
		}

		return
	}

	customFormatter := &log.TextFormatter{CallerPrettyfier: callerPrettyfierForLogrus}
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetReportCaller(true)
	dateStr = time.Now().Format("20060102")
	var err error
	config, err = getConf("conf.yaml")
	if err != nil {
		log.Error(err.Error())
		return
	}

	rootDir = config.PhotoDir
	if rootDir == "" {
		rootDir = DefaultDir
	}

	libPicDir = fmt.Sprintf("%s%c%s", rootDir, os.PathSeparator, "libpics")
	photoDir = fmt.Sprintf("%s%c%s", rootDir, os.PathSeparator, "pics")

	client := &http.Client{
		Timeout: 45 * time.Second,
	}

	if config.Proxy.UseProxy {
		proxyStr := config.Proxy.Protocol + "://"
		if config.Proxy.User != "" && config.Proxy.Password != "" {
			proxyStr += fmt.Sprintf("%s@%s", config.Proxy.User, config.Proxy.Password)
		}
		proxyStr += fmt.Sprintf("%s:%d", config.Proxy.Host, config.Proxy.Port)
		proxyUrl, err := url.Parse(proxyStr)
		if err == nil {
			var tr = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			}
			tr.Proxy = http.ProxyURL(proxyUrl)
			client.Transport = tr
		} else {
			log.Warnf("the format of http proxy is invalid.")
			return
		}
	}

	sg := sync.WaitGroup{}
	ch := make(chan *OneUser, 40)
	for i := 0; i < 20; i++ {
		sg.Add(1)
		go func() {
			doOneTask(ch)
			sg.Done()
		}()
	}

	c := 0
	for _, account := range config.Accounts {
		for _, seed := range account.Seeds {
			if account.Type == TelegramChannelRss {
				o := &OneUser{
					account:   seed,
					folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
					rsshubUrl: fmt.Sprintf("%s/", RssHubTelegramUrl),
					parser:    parseOneTelegramItem,
					client:    client,
					noDesc:    account.NoDesc,
				}
				ch <- o
				c++
				checkAndSleep(c)
			} else {
				for _, t := range []string{"media", "user"} {
					o := &OneUser{
						account:   seed,
						folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
						rsshubUrl: fmt.Sprintf("%s/%s/", RssHubTwitterUrl, t),
						parser:    parseOneTwitterItem,
						client:    client,
						noDesc:    account.NoDesc,
					}
					ch <- o
					c++
					checkAndSleep(c)
				}
			}
		}
	}

	close(ch)
	sg.Wait()
}

func checkAndSleep(c int) {
	if c%20 == 0 {
		time.Sleep(time.Second)
	}
}

func doOneTask(ch chan *OneUser) {
	for o := range ch {
		dealWithOneUrl(o)
	}
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
	defer f.Close()

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
	defer f.Close()

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

func dealWithOneUrl(user *OneUser) {
	var (
		client               = user.client
		rsshubUrl, seed, dir = user.rsshubUrl, user.account, user.folder
	)
	s := time.Now()
	parser := gofeed.NewParser()
	//parser.Client = user.client
	parser.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"
	feedUrl := rsshubUrl + seed
	feed, err := parser.ParseURL(feedUrl)
	if err != nil {
		log.Errorf("%s, err: %s", feedUrl, err.Error())
		return
	}
	log.Warnf("[%s], get all feeds cost %dms", feedUrl, time.Since(s)/time.Millisecond)

	getOne := func(item *oneItem) error {
		u := item.url
		h := md5.Sum([]byte(u))
		fileName := hex.EncodeToString(h[:])
		hashdir := fmt.Sprintf("%s%c%s%c%s", libPicDir, os.PathSeparator, fileName[:1], os.PathSeparator, fileName[1:2])
		_ = os.MkdirAll(hashdir, os.ModeDir|0755)
		libfilePath := fmt.Sprintf("%s%c%s", hashdir, os.PathSeparator, fileName)

		var err error
		flag, err := isFileExist(libfilePath)
		if err != nil {
			log.Warnf("%s, err: %s, file: %s", feedUrl, libfilePath, err.Error())
			return err
		}

		if flag {
			// log.Warnf("the file: account: %s, u: %s was downloaded.", seed, u)
			return nil
		}

		start := time.Now()
		req, err := http.NewRequest(http.MethodGet, u, nil)
		if err != nil {
			log.Warnf("failed to get pic: %s", u)
			log.Warn(err)
			return err
		}
		res, err := client.Do(req)
		if err != nil {
			log.Warnf("failed to get pic: %s", u)
			log.Error(err)
			return err
		}

		if res.StatusCode != 200 {
			log.Warnf("failed to get pic: %s", u)
			log.Warnf("some err: %v", res)
			return nil
		}

		pic, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Warn(err)
			return err
		}

		pic, err = addDesc(item, user, pic)
		if err != nil {
			return err
		}

		fileDir := fmt.Sprintf("%s%c%s", photoDir, os.PathSeparator, dir)
		_ = os.MkdirAll(fileDir, os.ModeDir|0755)
		filePath := fmt.Sprintf("%s%c%s%s", fileDir, os.PathSeparator, seed+"_"+fileName, getFileType(pic))
		f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			log.Error(err)
			return err
		}
		w := bufio.NewWriter(f)
		n, err := w.Write(pic)

		if err != nil || n != len(pic) {
			log.Error(err)
			_ = os.Remove(filePath)
			return err
		}
		_ = w.Flush()
		_ = f.Close()

		_, err = os.OpenFile(libfilePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			log.Error(err)
			_ = os.Remove(filePath)
			return err
		}

		log.Warnf("done one account: %s, u: %s, file: %s, cost: %dms", seed, u, filePath, time.Since(start)/time.Millisecond)
		return nil
	}

	if len(feed.Items) > 0 {
		for _, i := range feed.Items {
			items := user.parser(i.Description, seed)
			if len(items) == 0 {
				continue
			}
			for _, j := range items {
				err = getOne(j)
				if err != nil {
					log.Warnf("some error: %s, picUrl: %s, desc: %s", seed, j.url, i.Description)
				}
			}
		}
	}

	log.Warnf("[%s], done one round costs: %dms", seed, time.Since(s)/time.Millisecond)
}

func addDesc(item *oneItem, user *OneUser, pic []byte) ([]byte, error) {
	if user.noDesc {
		return pic, nil
	}

	ft := getFileType(pic)
	var p riimage.MediaParser
	if ft == ".jpg" {
		p = jpgs.NewJpegMediaParser()
		intfc, err := p.ParseBytes(pic)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		sl := intfc.(*jpgs.SegmentList)
		rootIb, _ := sl.ConstructExifBuilder()
		rootIb, err = addExif(item, rootIb)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		err = sl.SetExif(rootIb)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		b := new(bytes.Buffer)
		err = sl.Write(b)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		pic = b.Bytes()
	} else {
		p = pngs.NewPngMediaParser()
		intfc, err := p.ParseBytes(pic)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		cl := intfc.(*pngs.ChunkSlice)
		rootIb, _ := cl.ConstructExifBuilder()
		rootIb, err = addExif(item, rootIb)
		if err != nil {
			log.Error(err)
			return nil, err
		}

		err = cl.SetExif(rootIb)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		b := new(bytes.Buffer)
		err = cl.WriteTo(b)
		if err != nil {
			log.Error(err)
			return nil, err
		}
		pic = b.Bytes()
	}
	return pic, nil
}

func addExif(item *oneItem, rootIb *exif.IfdBuilder) (*exif.IfdBuilder, error) {
	if rootIb == nil {
		im, err := exifcommon.NewIfdMappingWithStandard()
		if err != nil {
			log.Error(err)
			return nil, err
		}
		ti := exif.NewTagIndex()
		rootIb = exif.NewIfdBuilder(im, ti, exifcommon.IfdStandardIfdIdentity, exifcommon.EncodeDefaultByteOrder)
	}

	exifIb, err := exif.GetOrCreateIbFromRootIb(rootIb, "IFD/Exif")
	if err != nil {
		log.Error(err)
		return nil, err
	}
	uc := exifundefined.Tag9286UserComment{
		EncodingType:  exifundefined.TagUndefinedType_9286_UserComment_Encoding_ASCII,
		EncodingBytes: []byte(item.desc),
	}
	err = exifIb.SetStandardWithName("UserComment", uc)
	if err != nil {
		log.Error(err)
		return nil, err
	}

	exifIb, err = exif.GetOrCreateIbFromRootIb(rootIb, "IFD")
	if err != nil {
		log.Error(err)
		return nil, err
	}
	err = exifIb.SetStandardWithName("Make", "None")
	if err != nil {
		log.Error(err)
		return nil, err
	}
	return rootIb, nil
}

func isFileExist(filePath string) (bool, error) {
	_, err := os.Stat(filePath)
	if err == nil {
		return true, nil
	}

	if os.IsExist(err) {
		return true, nil
	} else if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func getFileType(content []byte) string {
	// png 89 50 4E 47 0D 0A 1A 0A
	if bytes.HasPrefix(content, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		return ".png"
	}

	return ".jpg"
}
