package engine

import (
	"encoding/hex"
	"fmt"
	storage2 "github.com/anacrolix/torrent/storage"
	"github.com/simon-ding/cloud-torrent/storage"
	"github.com/simon-ding/cloud-torrent/yyets"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
)

//the Engine Cloud Torrent engine, backed by anacrolix/torrent
type Engine struct {
	mut      sync.Mutex
	cacheDir string
	client   *torrent.Client
	config   Config
	ts       map[string]*Torrent
	db       *storage.DB
	lock     sync.Mutex
}

func New() *Engine {
	return &Engine{ts: map[string]*Torrent{}}
}

func (e *Engine) Config() Config {
	return e.config
}

func (e *Engine) Configure(c Config) error {
	e.db.Close()
	var reTV = regexp.MustCompile(`.*?S..E..`)
	var reMovie = regexp.MustCompile(`(19..)|(20..)`)
	//recieve config
	if e.client != nil {
		e.client.Close()
		time.Sleep(1 * time.Second)
	}
	if c.IncomingPort <= 0 {
		return fmt.Errorf("Invalid incoming port (%d)", c.IncomingPort)
	}
	tc := torrent.NewDefaultClientConfig()
	tc.ListenPort = c.IncomingPort

	tc.NoUpload = !c.EnableUpload
	tc.Seed = c.EnableSeeding
	tc.DefaultStorage = storage2.NewFileWithCustomPathMaker(c.DownloadDirectory, func(baseDir string, info *metainfo.Info, infoHash metainfo.Hash) string {
		if reTV.MatchString(info.Name) { //is a tv episode
			name := reTV.FindString(info.Name) //eg. 猎魔人.The.Witcher.S01E07.中英字幕.WEBrip.720P-人人影视.mp4
			p := strings.Split(name, ".")
			if containChinese(p[0]) && len(p) > 2 {
				p = p[1 : len(p)-1] //去掉开头和结尾，只保留英文部分
			} else {
				p = p[:len(p)-1]
			}
			return path.Join(baseDir, "TVSeries", strings.Join(p, " "))
		} else if reMovie.MatchString(info.Name) { //this is an movie
			return path.Join(baseDir, "Movies")
		}
		return baseDir
	})
	//tc.DisableEncryption = c.DisableEncryption

	client, err := torrent.NewClient(tc)
	if err != nil {
		return err
	}
	e.mut.Lock()
	e.config = c
	e.client = client
	e.mut.Unlock()
	//reset
	e.GetTorrents()

	e.db = storage.GetDB(c.DownloadDirectory)
	e.db.PutLogin(c.YYETSUsername, c.YYETSPassword)
	go e.UpdateFavs()

	go func() { //load stored mangnets
		mangnets := e.db.GetTorrents()
		for _, m := range mangnets {
			e.NewMagnet(m)
		}
	}()
	return nil
}

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (e *Engine) SetLogin(username, password string) error {
	return e.db.PutLogin(username, password)
}

func (e *Engine) UpdateFavs() error {
	m := e.db.GetLogin()

	c := yyets.Client{}
	c.SetLogin(m["username"], m["password"])
	favs, err := c.UserFavs()
	if err != nil {
		return err
	}
	e.db.PutFavs(favs)
	for _, f := range favs {
		e.downloadUpdates(f)
	}
	return nil
}

func (e *Engine) Close() {
	e.db.Close()
}

//func (e *Engine) NewRSS(rssUrl string) error {
//	log.Printf("add new rss url %s\n", rssUrl)
//	splits := strings.Split(rssUrl, "/")
//	resourceID := splits[len(splits) - 1]
//
//	return e.addFavs(resourceID)
//}
//
//func (e *Engine) addFavs(resourceID string) error {
//	return e.db.Update(func(tx *bolt.Tx) error {
//		b := tx.Bucket([]byte(e.bucket))
//		var rss []string
//		data := b.Get([]byte("favs"))
//		if data != nil {
//			if err := json.Unmarshal(data, &rss); err != nil {
//				return err
//			}
//		}
//		for _, r := range rss {
//			if resourceID == r { //already exists
//				return nil
//			}
//		}
//		rss = append(rss, resourceID)
//		data, err := json.Marshal(&rss)
//		if err != nil {
//			return err
//		}
//		return b.Put([]byte("favs"), data)
//	})
//}

func (e *Engine) persistTorrents() {
	e.lock.Lock()
	defer e.lock.Unlock()
	var mangets []string

	for _, v := range e.ts {
		meta := v.t.Metainfo()
		info, err := meta.UnmarshalInfo()
		if err != nil {
			log.Println(err)
			continue
		}
		mag := meta.Magnet(info.Name, meta.HashInfoBytes())
		mangets = append(mangets, mag.String())
	}
	e.db.PersistTorrents(mangets)
}

func (e *Engine) DownloadUpdates() error {

	favs := e.db.GetFavs()
	for _, id := range favs {
		err := e.downloadUpdates(id)
		if err != nil {
			log.Println(err)
		}
	}
	return nil
}

func (e *Engine) downloadUpdates(resourceID string) error {
	url := yyets.FeedURL + resourceID
	c := &yyets.Client{}
	feed, err := c.ParseRssURL(url)
	if err != nil {
		return err
	}
	for _, item := range feed.Channel.Item {
		if time.Now().Sub(item.DateFormatted) > time.Hour*24*7 { //only download recent items
			continue
		}
		var in = e.db.AddDownload(resourceID, item.Guid)

		if in { //already downloaded
			continue
		}
		log.Printf("begin downloading %s", item.Title)
		if err := e.NewMagnet(item.Magnet); err != nil {
			log.Println(err)
			continue
		}
	}
	return nil
}

func (e *Engine) GetFavs() []yyets.Detail {
	var watches = e.db.GetFavs()

	c := yyets.Client{}
	var res []yyets.Detail
	for _, w := range watches {
		detail, err := c.GetDetail(w)
		if err != nil {
			log.Println(err)
			continue
		}
		res = append(res, *detail)
	}
	return res
}
func (e *Engine) NewMagnet(magnetURI string) error {
	tt, err := e.client.AddMagnet(magnetURI)
	if err != nil {
		return err
	}
	return e.newTorrent(tt)
}

func (e *Engine) NewTorrent(spec *torrent.TorrentSpec) error {
	tt, _, err := e.client.AddTorrentSpec(spec)
	if err != nil {
		return err
	}
	return e.newTorrent(tt)
}

func (e *Engine) newTorrent(tt *torrent.Torrent) error {
	t := e.upsertTorrent(tt)
	go func() {
		<-t.t.GotInfo()
		// if e.config.AutoStart && !loaded && torrent.Loaded && !torrent.Started {
		e.StartTorrent(t.InfoHash)
		// }
	}()
	return nil
}

//GetTorrents moves torrents out of the anacrolix/torrent
//and into the local cache
func (e *Engine) GetTorrents() map[string]*Torrent {
	e.mut.Lock()
	defer e.mut.Unlock()

	if e.client == nil {
		return nil
	}
	for _, tt := range e.client.Torrents() {
		e.upsertTorrent(tt)
	}
	return e.ts
}

func (e *Engine) upsertTorrent(tt *torrent.Torrent) *Torrent {
	ih := tt.InfoHash().HexString()
	torrent, ok := e.ts[ih]
	if !ok {
		torrent = &Torrent{InfoHash: ih}
		e.ts[ih] = torrent
	}
	//update torrent fields using underlying torrent
	torrent.Update(tt)
	e.persistTorrents()
	return torrent
}

func (e *Engine) getTorrent(infohash string) (*Torrent, error) {
	ih, err := str2ih(infohash)
	if err != nil {
		return nil, err
	}
	t, ok := e.ts[ih.HexString()]
	if !ok {
		return t, fmt.Errorf("Missing torrent %x", ih)
	}
	return t, nil
}

func (e *Engine) getOpenTorrent(infohash string) (*Torrent, error) {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return nil, err
	}
	// if t.t == nil {
	// 	newt, err := e.client.AddTorrentFromFile(filepath.Join(e.cacheDir, infohash+".torrent"))
	// 	if err != nil {
	// 		return t, fmt.Errorf("Failed to open torrent %s", err)
	// 	}
	// 	t.t = &newt
	// }
	return t, nil
}

func (e *Engine) StartTorrent(infohash string) error {
	t, err := e.getOpenTorrent(infohash)
	if err != nil {
		return err
	}
	if t.Started {
		return fmt.Errorf("Already started")
	}
	t.Started = true
	for _, f := range t.Files {
		if f != nil {
			f.Started = true
		}
	}
	if t.t.Info() != nil {
		t.t.DownloadAll()
	}
	return nil
}

func (e *Engine) StopTorrent(infohash string) error {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return err
	}
	if !t.Started {
		return fmt.Errorf("Already stopped")
	}
	//there is no stop - kill underlying torrent
	t.t.Drop()
	t.Started = false
	for _, f := range t.Files {
		if f != nil {
			f.Started = false
		}
	}
	return nil
}

func (e *Engine) DeleteTorrent(infohash string) error {
	t, err := e.getTorrent(infohash)
	if err != nil {
		return err
	}
	os.Remove(filepath.Join(e.cacheDir, infohash+".torrent"))
	delete(e.ts, t.InfoHash)
	ih, _ := str2ih(infohash)
	if tt, ok := e.client.Torrent(ih); ok {
		tt.Drop()
	}
	return nil
}

func (e *Engine) StartFile(infohash, filepath string) error {
	t, err := e.getOpenTorrent(infohash)
	if err != nil {
		return err
	}
	var f *File
	for _, file := range t.Files {
		if file.Path == filepath {
			f = file
			break
		}
	}
	if f == nil {
		return fmt.Errorf("Missing file %s", filepath)
	}
	if f.Started {
		return fmt.Errorf("Already started")
	}
	t.Started = true
	f.Started = true
	//f.f.PrioritizeRegion(0, f.Size)
	return nil
}

func (e *Engine) StopFile(infohash, filepath string) error {
	return fmt.Errorf("Unsupported")
}

func str2ih(str string) (metainfo.Hash, error) {
	var ih metainfo.Hash
	e, err := hex.Decode(ih[:], []byte(str))
	if err != nil {
		return ih, fmt.Errorf("Invalid hex string")
	}
	if e != 20 {
		return ih, fmt.Errorf("Invalid length")
	}
	return ih, nil
}

func containChinese(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
