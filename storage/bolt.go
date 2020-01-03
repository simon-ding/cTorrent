package storage

import (
	"encoding/json"
	"fmt"
	bolt "go.etcd.io/bbolt"
	"log"
	"path"
)

type DB struct {
	db *bolt.DB
}

const favs = "favs"
const login = "login"
const bucket = "data"
const torrents = "torrents"

func GetDB(dir string) *DB {
	db1, err := bolt.Open(path.Join(dir, ".data.bolt.db"), 0600, nil)
	if err != nil {
		return nil
	}
	db1.Update(func(tx *bolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(bucket))
		if err != nil {
			return fmt.Errorf("create bucket: %s", err)
		}
		return nil
	})
	return &DB{db: db1}
}

//GetTorrents 获取所有的磁链
func (d *DB) GetTorrents() []string {
	var res []string
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data := b.Get([]byte(torrents))
		return json.Unmarshal(data, &res)
	})
	return res
}

//PersistTorrents 存储磁链
func (d *DB) PersistTorrents(ts []string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data, err := json.Marshal(&ts)
		if err != nil {
			return err
		}
		return b.Put([]byte(torrents), data)
	})

}

func (d *DB) GetFavs() []string {
	var f []string
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data := b.Get([]byte(favs))
		return json.Unmarshal(data, &f)
	})
	return f
}

func (d *DB) PutFavs(f []string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data, err := json.Marshal(&f)
		if err != nil {
			return err
		}
		return b.Put([]byte(favs), data)
	})
}

func (d *DB) PutLogin(username, password string) error {
	return d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		user := make(map[string]string)
		user["username"] = username
		user["password"] = password
		data, err := json.Marshal(&user)
		if err != nil {
			return err
		}
		return b.Put([]byte(login), data)

	})
}

func (d *DB) GetLogin() map[string]string {
	var l = make(map[string]string)
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data := b.Get([]byte(login))
		return json.Unmarshal(data, &l)
	})
	return l
}

func (d *DB) AddDownload(resourceID, downloadID string) (exists bool) {
	var downloaded []string
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data := b.Get([]byte(resourceID))
		if data != nil {
			err := json.Unmarshal(data, &downloaded)
			if err != nil {
				log.Println(err)
			}
		}
		return nil
	})
	for _, d := range downloaded {
		if d == downloadID {
			exists = true
			return
		}
	}
	downloaded = append(downloaded, downloadID)
	d.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(bucket))
		data, err := json.Marshal(&downloaded)
		if err != nil {
			return err
		}
		return b.Put([]byte(resourceID), data)
	})

	return
}

func (d *DB) Close() {
	if d != nil {
		d.db.Close()
	}
}
