package main

import (
	"fmt"
	"os"
)

func NewThirtyFivePhoto(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
		rsshubUrl: fmt.Sprintf("%s/%s", ThirtyFivePhotoUrl, seed),
		parser:    parseOneTelegramItem,
		noDesc:    account.NoDesc,
		client:    client,
		aType:     account.Type,
	}
}

func NewTelegramChannel(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    fmt.Sprintf("%s%c%s", account.Dir, os.PathSeparator, dateStr),
		rsshubUrl: fmt.Sprintf("%s/%s", RssHubTelegramUrl, seed),
		parser:    parseOneTelegramItem,
		client:    client,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}
}

func NewWikiDailyPhoto(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		rsshubUrl: "https://zh.wikipedia.org/w/api.php?action=featuredfeed&feed=potd&feedformat=atom",
		parser:    parseOneWikiPhoto,
		client:    client,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}
}

func NewDailyArt(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		rsshubUrl: "https://rsshub.rssforever.com/dailyart/zh",
		parser:    parseDailyArt,
		client:    client,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}
}

func NewDouyin(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		rsshubUrl: fmt.Sprintf("%s/%s", RsshubDouyinUrl, seed),
		parser:    parseDouyinVideo,
		client:    clientWithoutProxy,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}
}

func NewTwitter(seed string, account *Account, mType string) *OneUser {
	folder := account.Dir
	if !account.NoDate {
		folder += fmt.Sprintf("%c/%s", os.PathSeparator, dateStr)
	}

	return &OneUser{
		account:   seed,
		folder:    folder,
		rsshubUrl: fmt.Sprintf("%s/%s/%s", RssHubTwitterUrl, mType, seed),
		parser:    parseOneTwitterItem,
		client:    client,
		noDesc:    account.NoDesc,
		aType:     account.Type,
	}
}

func NewCNU(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		rsshubUrl: "https://rsshub.rssforever.com/cnu/selected",
		client:    clientWithoutProxy,
	}
}

func NewMMFan(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		rsshubUrl: "https://rsshub.rssforever.com/95mm/tab/热门",
		client:    clientWithoutProxy,
	}
}

func NewWallPaper(seed string, account *Account) *OneUser {
	return &OneUser{
		account:   seed,
		folder:    account.Dir,
		parser:    parseCommonPhoto,
		noDesc:    account.NoDesc,
		aType:     account.Type,
		client:    client,
		rsshubUrl: "https://rsshub.app/konachan/post/popular_recent/1w",
	}
}
