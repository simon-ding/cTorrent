package yyets

import (
	"encoding/json"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
)

type Client struct {
	username string
	password string
	cookies  []*http.Cookie
}

func (c *Client) SetLogin(username, password string) {
	c.username = username
	c.password = password
}

func (c *Client) UserFavs() ([]string, error) {
	if err := c.login(); err != nil {
		return nil, err
	}
	req, err := http.NewRequest("GET", FavURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("User-Agent", UserAgent)

	for _, cc := range c.cookies {
		req.AddCookie(cc)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code %s", resp.Status)
	}
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	var res []string
	doc.Find(".user-favlist .fl-img ").Each(func(i int, selection *goquery.Selection) {
		href := selection.Find("a").AttrOr("href", "")
		splits := strings.Split(href, "/")
		resourceID := splits[len(splits)-1]
		res = append(res, resourceID)
	})

	return res, nil
}

func (c *Client) login() error {
	if c.username == "" || c.password == "" {
		return fmt.Errorf("username and password is needed")
	}
	v := url.Values{}
	v.Add("account", c.username)
	v.Add("password", c.password)
	v.Add("remember", "1")
	v.Add("url_back", FavURL)
	req, err := http.NewRequest("POST", LoginURL, strings.NewReader(v.Encode()))
	if err != nil {
		return err
	}

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Add("User-Agent", UserAgent)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := ioutil.ReadAll(resp.Body)
	var r struct {
		Status int    `json:"status"`
		Info   string `json:"info"`
	}
	json.Unmarshal(data, &r)

	if resp.StatusCode != 200 || r.Status != 1 {
		return fmt.Errorf("http status code: %s, response %v", resp.Status, r)
	}
	c.cookies = resp.Cookies()[2:]
	return nil
}
