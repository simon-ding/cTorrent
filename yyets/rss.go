package yyets

import (
	"encoding/xml"
	"io/ioutil"
	"net/http"
)

type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Channel Channel  `xml:"channel"`
}

type Channel struct {
	Description string  `xml:"description"`
	Link        string  `xml:"link"`
	Title       string  `xml:"title"`
	Item        []*Item `xml:"item"`
}

type Item struct {
	Link    string `xml:"link"`
	Title   string `xml:"title"`
	Guid    string `xml:"guid"`
	PubDate string `xml:"pubDate"`
	Magnet  string `xml:"magnet"`
	Ed2k    string `xml:"ed2k"`
}

func ParseRssURL(url string) (*Feed, error) {
	MaxRetries := 5
	var data []byte
	for i := 0; i < MaxRetries; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			data, err = ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}
	var feed Feed
	err := xml.Unmarshal(data, &feed)
	return &feed, err
}
