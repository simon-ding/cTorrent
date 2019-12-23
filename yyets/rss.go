package yyets

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"net/http"
	"time"
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
	Link          string    `xml:"link"`
	Title         string    `xml:"title"`
	Guid          string    `xml:"guid"`
	PubDate       string    `xml:"pubDate"`
	DateFormatted time.Time `xml:"date"`
	Magnet        string    `xml:"magnet"`
	Ed2k          string    `xml:"ed2k"`
}

func (c *Client) ParseRssURL(url string) (*Feed, error) {
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
	if err != nil {
		return nil, err
	}
	for _, item := range feed.Channel.Item {
		item.DateFormatted, _ = time.Parse("Mon, 2 Jan 2006 15:04:05 -0700", item.PubDate)
	}
	return &feed, err
}

type Detail struct {
	ID            string `json:"id"`
	Cnname        string `json:"cnname"`
	Enname        string `json:"enname"`
	Channel       string `json:"channel"`
	ChannelCN     string `json:"channel_cn"`
	Category      string `json:"category"`
	CloseResource string `json:"close_resource"`
	PlayStatus    string `json:"play_status"`
	Poster        string `json:"poster"`
	URL           string `json:"url"`
}

func (c *Client) GetDetail(resourceID string) (*Detail, error) {
	var res struct {
		Data struct {
			Detail Detail `json:"detail"`
		} `json:"data"`
	}
	MaxRetries := 5
	var data []byte
	url := DetailURL + resourceID
	for i := 0; i < MaxRetries; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			data, err = ioutil.ReadAll(resp.Body)
			resp.Body.Close()
			break
		}
		resp.Body.Close()
	}
	err := json.Unmarshal(data, &res)
	if err != nil {
		return nil, err
	}
	res.Data.Detail.URL = ResourceURL + resourceID
	return &res.Data.Detail, nil
}
