package models

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

type FastData struct {
	Token      string     `json:"token"`
	RecordInfo RecordInfo `json:"recordInfo"`
}
type FastDataJson struct {
	DataList []FastData `json:"dataList"`
	LastId   int        `json:"lastId"`
}
type FastStore struct {
	mu   sync.RWMutex
	path string
	data FastDataJson
}

func NewFastStore(path string, startID int) (*FastStore, error) {
	s := &FastStore{path: path, data: FastDataJson{LastId: startID}}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return nil, err
		}
		if err := s.saveLocked(); err != nil {
			return nil, err
		}
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(b, &s.data); err != nil {
		return nil, fmt.Errorf("快速 DDNS 数据损坏 %s: %w", path, err)
	}
	return s, nil
}

func (s *FastStore) saveLocked() error {
	b, err := json.Marshal(&s.data)
	if err != nil {
		return err
	}
	dir := filepath.Dir(s.path)
	f, err := os.CreateTemp(dir, ".fast-*.tmp")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(b)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(tmp, s.path)
}

func (s *FastStore) WithWrite(fn func(*FastDataJson) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	before, _ := json.Marshal(s.data)
	if err := fn(&s.data); err != nil {
		return err
	}
	if err := s.saveLocked(); err != nil {
		_ = json.Unmarshal(before, &s.data)
		return err
	}
	return nil
}
func (s *FastStore) FindIP(ip string) (FastData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.data.DataList {
		if d.RecordInfo.RecordContent == ip {
			return d, true
		}
	}
	return FastData{}, false
}
func (s *FastStore) FindToken(token string) (FastData, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, d := range s.data.DataList {
		if subtle.ConstantTimeCompare([]byte(d.Token), []byte(token)) == 1 {
			return d, true
		}
	}
	return FastData{}, false
}
func (s *FastStore) LastID() int { s.mu.RLock(); defer s.mu.RUnlock(); return s.data.LastId }

func NewFastToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
