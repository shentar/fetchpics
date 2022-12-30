package main

import (
	"fmt"
	"os"
)

func NewThirtyFivePhoto(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
		rsshubUrl: fmt.Sprintf("%s/%s", ThirtyFivePhotoUrl, seed),
		parser:    parseOneTelegramItem,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}

	account.ContentUseProxy = true
	SetHttpClient(u, account)
	return u
}

func NewTelegramChannel(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
		rsshubUrl: fmt.Sprintf("%s/%s", RssHubTelegramUrl, seed),
		parser:    parseOneTelegramItem,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}

	account.ContentUseProxy = true
	SetHttpClient(u, account)
	return u
}

func NewWikiDailyPhoto(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    account.Dir,
		rsshubUrl: "https://zh.wikipedia.org/w/api.php?action=featuredfeed&feed=potd&feedformat=atom",
		parser:    parseOneWikiPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}

	account.ContentUseProxy = true
	account.FeedUseProxy = true
	SetHttpClient(u, account)

	return u
}

func NewDailyArt(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    account.Dir,
		rsshubUrl: "https://rsshub.rssforever.com/dailyart/zh",
		parser:    parseDailyArt,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}

	account.ContentUseProxy = true
	SetHttpClient(u, account)
	return u
}

func NewDouyin(seed string, account *Account) *OneUser {
	u := &OneUser{
		account: seed,
		folder:  account.Dir,
		parser:  parseDouyinVideo,
		noDesc:  account.NoDesc,
		aType:   account.Type,
	}

	url := RsshubDouyinUrl
	if len(account.Url) > 0 {
		url = account.Url
	}

	u.rsshubUrl = fmt.Sprintf("%s/%s", url, seed)

	SetHttpClient(u, account)

	return u
}

func SetHttpClient(u *OneUser, account *Account) {
	u.feedParserClient = clientWithoutProxy
	u.contentClient = clientWithoutProxy
	if account.FeedUseProxy {
		u.feedParserClient = client
	}

	if account.ContentUseProxy {
		u.contentClient = client
	}
}

func NewTwitter(seed string, account *Account, mType string) *OneUser {
	folder := account.Dir
	if !account.NoDate {
		folder += fmt.Sprintf("%c/%s", os.PathSeparator, dateStr)
	}

	u := &OneUser{
		account:   seed,
		folder:    folder,
		rsshubUrl: fmt.Sprintf("%s/%s/%s", RssHubTwitterUrl, mType, seed),
		parser:    parseOneTwitterItem,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}

	account.ContentUseProxy = true
	SetHttpClient(u, account)

	return u
}

func NewCNU(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		rsshubUrl: "https://rsshub.rssforever.com/cnu/selected",
	}

	SetHttpClient(u, account)

	return u
}

func NewMMFan(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		rsshubUrl: "https://rsshub.rssforever.com/95mm/tab/热门",
	}

	SetHttpClient(u, account)
	return u
}

func NewWallPaper(seed string, account *Account) *OneUser {
	u := &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		rsshubUrl: "https://rsshub.app/konachan/post/popular_recent/1w",
	}

	account.ContentUseProxy = true
	account.FeedUseProxy = true
	SetHttpClient(u, account)
	return u
}
