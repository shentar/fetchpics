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
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
	"time"
)

const (
	RSSHUB_URL = "https://rsshub.rssforever.com/twitter/user/"
)

type oneItem struct {
	url  string
	desc string
}

type Account struct {
	Dir   string   `yaml:"dir"`
	Seeds []string `yaml:"seeds"`
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
}

func callerPrettyfierForLogrus(caller *runtime.Frame) (string, string) {
	fileName := filepath.Base(caller.File)
	funcName := filepath.Base(caller.Function)
	return funcName, fmt.Sprintf("%s:%d", fileName, caller.Line)
}

func main() {
	customFormatter := &log.TextFormatter{CallerPrettyfier: callerPrettyfierForLogrus}
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)
	log.SetReportCaller(true)

	config, err := getConf()
	if err != nil {
		log.Error(err.Error())
		return
	}

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

	rsshubUrl := config.RssHubUrl
	if rsshubUrl == "" {
		rsshubUrl = RSSHUB_URL
	}

	sg := sync.WaitGroup{}
	_ = os.MkdirAll("libpics", os.ModeDir|0755)
	for _, account := range config.Accounts {
		_ = os.MkdirAll(account.Dir, os.ModeDir|0755)
		for _, seed := range account.Seeds {
			sg.Add(1)
			s := seed
			folder := account.Dir
			go func() {
				dealWithOneUrl(client, rsshubUrl, s, folder)
				sg.Done()
			}()
		}
	}

	sg.Wait()
}

func getConf() (*Conf, error) {
	c := Conf{}

	f, err := os.OpenFile("conf.yaml", os.O_RDONLY, 0755)
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

func dealWithOneUrl(client *http.Client, rsshubUrl, seed, dir string) {
	s := time.Now()
	parser := gofeed.NewParser()
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
		hashdir := fmt.Sprintf("libpics%c%s%c%s", os.PathSeparator, fileName[:1], os.PathSeparator, fileName[1:2])
		_ = os.MkdirAll(hashdir, os.ModeDir|0755)
		libfilePath := fmt.Sprintf("%s%c%s", hashdir, os.PathSeparator, fileName)

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
			log.Warn(err)
			return err
		}
		res, err := client.Do(req)
		if err != nil {
			log.Error(err)
			return err
		}

		if res.StatusCode != 200 {
			log.Warnf("some err: %v", res)
			return nil
		}

		pic, err := ioutil.ReadAll(res.Body)
		if err != nil {
			log.Warn(err)
			return err
		}

		ft := getFileType(pic)
		var p riimage.MediaParser
		if ft == ".jpg" {
			p = jpgs.NewJpegMediaParser()
			intfc, err := p.ParseBytes(pic)
			if err != nil {
				log.Error(err)
				return err
			}

			sl := intfc.(*jpgs.SegmentList)
			rootIb, _ := sl.ConstructExifBuilder()
			rootIb, err = addExif(item, rootIb)
			if err != nil {
				log.Error(err)
				return err
			}

			err = sl.SetExif(rootIb)
			if err != nil {
				log.Error(err)
				return err
			}

			b := new(bytes.Buffer)
			err = sl.Write(b)
			if err != nil {
				log.Error(err)
				return err
			}
			pic = b.Bytes()
		} else {
			p = pngs.NewPngMediaParser()
			intfc, err := p.ParseBytes(pic)
			if err != nil {
				log.Error(err)
				return err
			}

			cl := intfc.(*pngs.ChunkSlice)
			rootIb, _ := cl.ConstructExifBuilder()
			rootIb, err = addExif(item, rootIb)
			if err != nil {
				log.Error(err)
				return err
			}

			err = cl.SetExif(rootIb)
			if err != nil {
				log.Error(err)
				return err
			}
			b := new(bytes.Buffer)
			err = cl.WriteTo(b)
			if err != nil {
				log.Error(err)
				return err
			}
			pic = b.Bytes()
		}

		dataDir := time.Now().Format("20060102")
		fileDir := fmt.Sprintf("%s%c%s", dir, os.PathSeparator, dataDir)
		_ = os.MkdirAll(fileDir, os.ModeDir|0755)
		filePath := fmt.Sprintf("%s%c%s%s", fileDir, os.PathSeparator, seed+"_" +fileName, getFileType(pic))
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

		_ = os.MkdirAll(dir, os.ModeDir|0755)
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
			reg := regexp.MustCompile(`<img style src="(.*?=orig)"+`)
			ss := reg.FindAllStringSubmatch(i.Description, -1)
			if len(ss) == 0 {
				continue
			}

			for _, j := range ss {
				if len(j) == 2 {
					it := &oneItem{url: j[1], desc: i.Description + fmt.Sprintf("@%s", seed)}
					err = getOne(it)
					if err != nil {
						log.Warnf("some error: %s, picUrl: %s, desc: %s", seed, it.url, i.Description)
					}
				}
			}
		}
	}
	log.Warnf("[%s], done one round costs: %dms", seed, time.Since(s)/time.Millisecond)
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
