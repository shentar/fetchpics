package main

import (
	"bufio"
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"github.com/mmcdole/gofeed"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"
)

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

	s := time.Now()
	parser := gofeed.NewParser()
	feed, err := parser.ParseURL("https://rsshub.rssforever.com/twitter/user/seanwei001")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("get all feeds cost %dms\n", time.Since(s)/time.Millisecond)

	_ = os.MkdirAll("pics", os.ModeDir|0755)
	_ = os.MkdirAll("libpics", os.ModeDir|0755)

	getOne := func(url string) error {
		h := md5.Sum([]byte(url))
		fileName := hex.EncodeToString(h[:])
		dir := fmt.Sprintf("libpics%c%s%c%s", os.PathSeparator, fileName[:1], os.PathSeparator, fileName[1:2])
		libfilePath := fmt.Sprintf("%s%c%s", dir, os.PathSeparator, fileName)

		flag, err := isFileExist(libfilePath)
		if err != nil {
			fmt.Println(err)
			return err
		}

		if flag {
			fmt.Printf("the file: %s was downloaded.\n", url)
			return nil
		}

		start := time.Now()
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			fmt.Println(err)
			return err
		}
		res, err := client.Do(req)
		if err != nil {
			fmt.Println(err)
			return err
		}

		if res.StatusCode != 200 {
			fmt.Println("some err: ", res)
			return nil
		}

		pic, err := ioutil.ReadAll(res.Body)
		if err != nil {
			fmt.Println(err)
			return err
		}

		filePath := fmt.Sprintf("pics%c%s%s", os.PathSeparator, fileName, getFileType(pic))
		f, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			fmt.Println(err)
			return err
		}
		w := bufio.NewWriter(f)
		n, err := w.Write(pic)
		if err != nil || n != len(pic) {
			fmt.Println(err)
			_ = os.Remove(filePath)
			return err
		}
		_ = w.Flush()
		_ = f.Close()

		_ = os.MkdirAll(dir, os.ModeDir|0755)
		_, err = os.OpenFile(libfilePath, os.O_RDWR|os.O_CREATE, 0755)
		if err != nil {
			fmt.Println(err)
			_ = os.Remove(filePath)
			return err
		}

		fmt.Printf("done one url: %s, file: %s, cost: %dms\n", url, filePath, time.Since(start)/time.Millisecond)
		return nil
	}

	if len(feed.Items) > 0 {
		for _, i := range feed.Items {
			reg := regexp.MustCompile(`<img style src="(.*?=orig)"`)
			ss := reg.FindAllStringSubmatch(i.Description, 1)
			if len(ss) == 1 && len(ss[0]) == 2 {
				_ = getOne(ss[0][1])
			}
		}
	}
	fmt.Printf("done one round costs: %dms\n", time.Since(s)/time.Millisecond)

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
