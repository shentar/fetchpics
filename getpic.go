package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"github.com/dsoprea/go-exif/v3"
	"github.com/dsoprea/go-exif/v3/undefined"
	jpgs "github.com/dsoprea/go-jpeg-image-structure/v2"
	"github.com/mmcdole/gofeed"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

type oneItem struct {
	url  string
	desc string
}

type Conf struct {
	Accounts []string `yaml:"accounts"`
}

func main() {
	var tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	proxyUrl, err := url.Parse("http://22.22.22.14:10080")
	if err == nil {
		tr.Proxy = http.ProxyURL(proxyUrl)
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   45 * time.Second,
	}

	config, err := getConf()
	if err != nil {
		log.Error(err.Error())
		return
	}

	for _, account := range config.Accounts {
		dealWithOneUrl(client, "https://rsshub.rssforever.com/twitter/user/"+account)
	}
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

func dealWithOneUrl(client *http.Client, url string) {
	s := time.Now()
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL(url)
	if err != nil {
		log.Error(err)
		return
	}
	log.Warnf("[%s], get all feeds cost %dms\n", url, time.Since(s)/time.Millisecond)

	_ = os.MkdirAll("pics", os.ModeDir|0755)
	_ = os.MkdirAll("libpics", os.ModeDir|0755)

	getOne := func(item *oneItem) error {
		u := item.url
		h := md5.Sum([]byte(u))
		fileName := hex.EncodeToString(h[:])
		dir := fmt.Sprintf("libpics%c%s%c%s", os.PathSeparator, fileName[:1], os.PathSeparator, fileName[1:2])
		libfilePath := fmt.Sprintf("%s%c%s", dir, os.PathSeparator, fileName)

		flag, err := isFileExist(libfilePath)
		if err != nil {
			log.Warn(err)
			return err
		}

		if flag {
			log.Warnf("the file: %s was downloaded.", u)
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
		if ft == ".jpg" {
			p := jpgs.NewJpegMediaParser()
			intfc, err := p.ParseBytes(pic)
			if err != nil {
				log.Error(err)
				return err
			}

			sl := intfc.(*jpgs.SegmentList)
			rootIb, err := sl.ConstructExifBuilder()
			if err != nil {
				log.Error(err)
				return err
			}
			exifIb, err := exif.GetOrCreateIbFromRootIb(rootIb, "IFD/Exif")
			if err != nil {
				log.Error(err)
				return err
			}
			uc := exifundefined.Tag9286UserComment{
				EncodingType:  exifundefined.TagUndefinedType_9286_UserComment_Encoding_ASCII,
				EncodingBytes: []byte(item.desc),
			}
			err = exifIb.SetStandardWithName("UserComment", uc)
			if err != nil {
				log.Error(err)
				return err
			}

			exifIb, err = exif.GetOrCreateIbFromRootIb(rootIb, "IFD")
			if err != nil {
				log.Error(err)
				return err
			}
			err = exifIb.SetStandardWithName("Make", "None")
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
			pic = b.Bytes()
		}

		filePath := fmt.Sprintf("pics%c%s%s", os.PathSeparator, fileName, getFileType(pic))
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

		log.Warnf("done one u: %s, file: %s, cost: %dms\n", u, filePath, time.Since(start)/time.Millisecond)
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
					_ = getOne(&oneItem{url: j[1], desc: i.Description})
				}
			}
		}
	}
	log.Warnf("[%s], done one round costs: %dms\n", url, time.Since(s)/time.Millisecond)
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
