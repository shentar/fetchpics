package main

import (
	"fmt"
	"github.com/mmcdole/gofeed"
	"regexp"
	"strings"
)

type Parser func(item *gofeed.Item, seed string) []*oneItem

func parseOneTwitterItem(item *gofeed.Item, seed string) []*oneItem {
	des := item.Description
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

func parseOneTelegramItem(item *gofeed.Item, seed string) []*oneItem {
	des := item.Description
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

func parseOneWikiPhoto(item *gofeed.Item, seed string) []*oneItem {
	des := item.Description
	reg := regexp.MustCompile(`.*<img.*?src="(//.*?.jpg)/+`)
	ss := reg.FindAllStringSubmatch(des, -1)
	if len(ss) == 0 {
		return nil
	}

	var items []*oneItem
	for _, j := range ss {
		if len(j) == 2 {
			urlDecoded := strings.Replace(j[1], "&amp;", "&", -1)
			urlDecoded = strings.Replace(urlDecoded, "/thumb/", "/", -1)
			it := &oneItem{url: "https:" + urlDecoded, desc: des}
			items = append(items, it)
		}
	}

	return items
}

func parseDouyinVideo(item *gofeed.Item, seed string) []*oneItem {
	des := item.Description
	reg := regexp.MustCompile(`.*<a href="(.*)" rel="noreferrer">视频直链</a>`)
	ss := reg.FindAllStringSubmatch(des, -1)
	if len(ss) == 0 {
		return nil
	}

	var items []*oneItem

	if len(ss) != 1 || len(ss[0]) != 2 {
		return items
	}

	urlDecoded := strings.Replace(ss[0][1], "&amp;", "&", -1)
	it := &oneItem{url: urlDecoded, desc: des}
	replacer := strings.NewReplacer(
		"!", "_", "@", "_", "#", "_",
		"$", "_", "%", "_", "^", "_", "*", "_",
		"&", "_", ".", "_", ",", "_",
		"\\", "_", "/", "_", "|", "_",
		"~", "_", "?", "_", "]", "_",
		"[", "_", "{", "_", "}", "_",
		"<", "_", ">", "_", "|", " ",
	)
	title := replacer.Replace(item.Title)

	prefix := seed
	if len(seed) > 5 {
		prefix = seed[len(seed)-5:]
	}

	it.fileName = fmt.Sprintf("%s_%s.mp4", prefix, title)
	items = append(items, it)

	return items
}

func parseCommonPhoto(item *gofeed.Item, seed string) []*oneItem {
	des := item.Description
	reg := regexp.MustCompile(`.*?<img src="(.*?://.*?.jpg)".*?>+`)
	ss := reg.FindAllStringSubmatch(des, -1)
	if len(ss) == 0 {
		return nil
	}

	var items []*oneItem
	for _, j := range ss {
		if len(j) == 2 {
			urlDecoded := strings.Replace(j[1], "&amp;", "&", -1)
			it := &oneItem{url: urlDecoded, desc: des}
			items = append(items, it)
		}
	}

	return items
}

func parseDailyArt(item *gofeed.Item, seed string) []*oneItem {
	guid := item.GUID
	urlDecoded := strings.Replace(guid, "&amp;", "&", -1)
	it := &oneItem{url: urlDecoded, desc: item.Description + fmt.Sprintf("@%s", seed)}

	return []*oneItem{it}
}
