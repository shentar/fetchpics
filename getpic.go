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
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	config       *Conf
	rootDir      string
	libPicDir    string
	photoDir     string
	dateStr      string
	allTaskCount int

	client, clientWithoutProxy *http.Client
)

type oneItem struct {
	url      string
	desc     string
	guid     string
	fileName string
}

type OneUser struct {
	account   string
	folder    string
	rsshubUrl string
	parser    Parser
	noDesc    bool
	client    *http.Client
	aType     string
}

func main() {
	if len(os.Args) == 3 && os.Args[1] == "format" {
		err := formatConf(os.Args[2])
		if err != nil {
			fmt.Println(err.Error())
		}

		return
	}

	customFormatter := &log.TextFormatter{CallerPrettyfier: callerPrettyFieldForLogrus}
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

	client = &http.Client{
		Timeout: 900 * time.Second,
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

	clientWithoutProxy = &http.Client{
		Timeout: 900 * time.Second,
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

	for _, account := range config.Accounts {
		for _, seed := range account.Seeds {
			if account.Type == ThirtyFivePhotoRss {
				addOneTask(ch, NewThirtyFivePhoto(seed, &account))
			} else if account.Type == TelegramChannelRss {
				addOneTask(ch, NewTelegramChannel(seed, &account))
			} else if account.Type == WikiDailyPhotoRSS {
				addOneTask(ch, NewWikiDailyPhoto(seed, &account))
			} else if account.Type == DailyArt {
				addOneTask(ch, NewDailyArt(seed, &account))
			} else if account.Type == "" {
				for _, t := range []string{"media", "user"} {
					addOneTask(ch, NewTwitter(seed, &account, t))
				}
			} else if account.Type == Douyin {
				addOneTask(ch, NewDouyin(seed, &account))
			} else {
				switch account.Type {
				case CNU:
					addOneTask(ch, NewCNU(seed, &account))
				case MMFan:
					addOneTask(ch, NewMMFan(seed, &account))
				case WallPaper:
					addOneTask(ch, NewWallPaper(seed, &account))
				}
			}
		}
	}

	close(ch)
	sg.Wait()
}

func addOneTask(ch chan *OneUser, o *OneUser) int {
	ch <- o
	allTaskCount++
	checkAndSleep(allTaskCount)
	return 1
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

func dealWithOneUrl(user *OneUser) {
	var (
		cli                = user.client
		feedUrl, seed, dir = user.rsshubUrl, user.account, user.folder
	)
	s := time.Now()
	p := gofeed.NewParser()
	if user.aType == WikiDailyPhotoRSS || user.aType == WallPaper || user.aType == Douyin {
		p.Client = client
	}

	p.UserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/97.0.4692.71 Safari/537.36"
	feed, err := p.ParseURL(feedUrl)
	if err != nil {
		log.Errorf("%s, err: %s", feedUrl, err.Error())
		return
	}
	log.Warnf("[%s], get all feeds cost %dms", feedUrl, time.Since(s)/time.Millisecond)

	getOne := func(item *oneItem) error {
		u := item.url
		guid := item.guid
		if guid == "" {
			guid = u
		}
		h := md5.Sum([]byte(guid))
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
		res, err := cli.Do(req)
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

		var filePath string
		if item.fileName != "" {
			filePath = fmt.Sprintf("%s%c%s.tmp", fileDir, os.PathSeparator, item.fileName)
		} else {
			filePath = fmt.Sprintf("%s%c%s%s.tmp", fileDir, os.PathSeparator,
				strings.Replace(seed, "/", "", -1)+"_"+fileName, getFileType(pic))
		}

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

		// 转正文件。
		fstat, err := os.Stat(filePath)
		if err != nil {
			return err
		}
		dstFilePath := strings.TrimRight(filePath, ".tmp")
		fdstat, err := os.Stat(dstFilePath)
		if err == nil {
			// 同名文件的size比新文件小，则覆盖，否则保留
			if fdstat.Size() < fstat.Size() {
				_ = os.Remove(dstFilePath)
				_ = os.Rename(filePath, dstFilePath)
			} else {
				_ = os.Remove(filePath)
			}
		} else {
			// 新文件则直接转正。
			if os.IsNotExist(err) {
				// 丢弃小于8MB的MP4文件。
				if strings.HasSuffix(filePath, ".mp4.tmp") && fstat.Size() < 1048576*8 {
					_ = os.Remove(filePath)
				} else {
					_ = os.Rename(filePath, dstFilePath)
				}
			} else {
				log.Warnf("some error: %s", err.Error())
				return err
			}
		}

		log.Warnf("done one account: %s, u: %s, file: %s, cost: %dms", seed, u, dstFilePath, time.Since(start)/time.Millisecond)
		return nil
	}

	if len(feed.Items) > 0 {
		for _, i := range feed.Items {
			items := user.parser(i, seed)
			if len(items) == 0 {
				continue
			}
			for sn, j := range items {
				if user.aType == TelegramChannelRss {
					j.guid = fmt.Sprintf("%s-%d", i.GUID, sn)
				}
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
