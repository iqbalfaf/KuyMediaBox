package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	wruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"kuymediabox/internal/appdir"
	"kuymediabox/internal/downloader"
	"kuymediabox/internal/i18n"
)

// Subscriptions: channels, playlists, profiles and boards checked every few hours; new
// items are downloaded with the options chosen when subscribing.

type subscription struct {
	ID         string             `json:"id"`
	URL        string             `json:"url"`
	Title      string             `json:"title"`
	Source     string             `json:"source"`
	Type       string             `json:"type"`
	Thumbnail  string             `json:"thumbnail"`
	Options    downloader.Options `json:"options"`
	Tabs       []string           `json:"tabs"` // channel tabs to follow (videos, shorts, streams); empty = all
	EveryHours int                `json:"everyHours"`
	Enabled    bool               `json:"enabled"`
	Created    int64              `json:"created"`
	LastCheck  int64              `json:"lastCheck"`
	LastNew    int                `json:"lastNew"`
	TotalNew   int                `json:"totalNew"`
	LastError  string             `json:"lastError"`
	Seen       []string           `json:"seen"` // item IDs already handled
}

// SubscriptionInfo is a subscription as the UI shows it.
type SubscriptionInfo struct {
	ID         string   `json:"id"`
	URL        string   `json:"url"`
	Title      string   `json:"title"`
	Source     string   `json:"source"`
	Type       string   `json:"type"`
	Thumbnail  string   `json:"thumbnail"`
	Mode       string   `json:"mode"` // video | audio
	Tabs       []string `json:"tabs"`
	EveryHours int      `json:"everyHours"`
	Enabled    bool     `json:"enabled"`
	LastCheck  int64    `json:"lastCheck"`
	NextCheck  int64    `json:"nextCheck"`
	LastNew    int      `json:"lastNew"`
	TotalNew   int      `json:"totalNew"`
	LastError  string   `json:"lastError"`
	Checking   bool     `json:"checking"`
}

type subStore struct {
	mu       sync.Mutex
	path     string
	subs     []*subscription
	checking map[string]bool
}

func newSubStore() *subStore {
	s := &subStore{path: filepath.Join(appdir.DataDir(), "subscriptions.json"), checking: map[string]bool{}}
	if data, err := os.ReadFile(s.path); err == nil {
		data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF}) // edited by hand in Notepad
		_ = json.Unmarshal(data, &s.subs)
	}
	return s
}

func (s *subStore) saveLocked() {
	data, _ := json.MarshalIndent(s.subs, "", " ")
	tmp := s.path + ".tmp"
	if os.WriteFile(tmp, data, 0o644) == nil {
		_ = os.Rename(tmp, s.path)
	}
}

func (s *subStore) find(id string) *subscription {
	for _, sub := range s.subs {
		if sub.ID == id {
			return sub
		}
	}
	return nil
}

func (s *subscription) info(checking bool) SubscriptionInfo {
	next := int64(0)
	if s.Enabled {
		next = s.LastCheck + int64(s.EveryHours)*3600
	}
	return SubscriptionInfo{ID: s.ID, URL: s.URL, Title: s.Title, Source: s.Source, Type: s.Type, Thumbnail: s.Thumbnail, Mode: s.Options.Mode,
		Tabs: s.Tabs, EveryHours: s.EveryHours, Enabled: s.Enabled, LastCheck: s.LastCheck, NextCheck: next, LastNew: s.LastNew,
		TotalNew: s.TotalNew, LastError: s.LastError, Checking: checking}
}

// subscribable collection types.
var subTypes = map[string]bool{downloader.TypeChannel: true, downloader.TypePlaylist: true, downloader.TypeProfile: true,
	downloader.TypeBoard: true, downloader.TypeArtist: true, downloader.TypeAlbum: true}

// ListSubscriptions returns every subscription, newest first.
func (a *App) ListSubscriptions() []SubscriptionInfo {
	a.subs.mu.Lock()
	defer a.subs.mu.Unlock()
	out := make([]SubscriptionInfo, 0, len(a.subs.subs))
	for _, s := range a.subs.subs {
		out = append(out, s.info(a.subs.checking[s.ID]))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Title < out[j].Title })
	return out
}

func (a *App) subsChanged() { a.emit("subscriptions:changed", a.ListSubscriptions()) }

// AddSubscription follows the link of a checked collection. backfill also downloads what
// the link holds now; otherwise only items published from now on are downloaded.
func (a *App) AddSubscription(key string, everyHours int, tabs []string, o downloader.Options, backfill bool) (SubscriptionInfo, error) {
	a.colMu.Lock()
	col := a.collections[key]
	a.colMu.Unlock()
	if col == nil {
		return SubscriptionInfo{}, errors.New(i18n.L("Data link sudah kedaluwarsa, periksa link lagi", "Link data expired, check the link again"))
	}
	if !subTypes[col.Type] {
		return SubscriptionInfo{}, errors.New(i18n.L("Hanya channel, playlist, profil, board, album, atau artis yang bisa dilanggan", "Only channels, playlists, profiles, boards, albums or artists can be followed"))
	}
	switch everyHours {
	case 1, 3, 6, 12, 24, 72, 168:
	default:
		everyHours = 6
	}
	o.Normalize(col.Source)
	o.SkipExisting = true
	o.SectionStart, o.SectionEnd = "", ""
	sub := &subscription{ID: fmt.Sprintf("s%d", time.Now().UnixNano()), URL: col.URL, Title: col.Title, Source: col.Source, Type: col.Type,
		Thumbnail: col.Thumbnail, Options: o, Tabs: tabs, EveryHours: everyHours, Enabled: true, Created: time.Now().Unix(), LastCheck: time.Now().Unix()}
	var start []string
	for _, e := range col.Entries {
		if backfill && !e.Unavailable && !e.Archived && tabAllowed(sub.Tabs, e.Tab) {
			start = append(start, e.ID)
		} else {
			sub.Seen = append(sub.Seen, e.ID)
		}
	}
	a.subs.mu.Lock()
	for _, s := range a.subs.subs {
		if s.URL == sub.URL {
			a.subs.mu.Unlock()
			return SubscriptionInfo{}, errors.New(i18n.L("Link ini sudah dilanggan", "This link is already followed"))
		}
	}
	a.subs.subs = append(a.subs.subs, sub)
	a.subs.saveLocked()
	a.subs.mu.Unlock()
	if len(start) > 0 {
		if _, err := a.StartDownloads(key, start, o); err == nil {
			a.subs.mu.Lock()
			sub.Seen = append(sub.Seen, start...)
			sub.LastNew, sub.TotalNew = len(start), len(start)
			a.subs.saveLocked()
			a.subs.mu.Unlock()
		}
	}
	a.subsChanged()
	return sub.info(false), nil
}

func tabAllowed(tabs []string, tab string) bool {
	if len(tabs) == 0 || tab == "" {
		return true
	}
	for _, t := range tabs {
		if t == tab {
			return true
		}
	}
	return false
}

// RemoveSubscription stops following a link.
func (a *App) RemoveSubscription(id string) {
	a.subs.mu.Lock()
	for i, s := range a.subs.subs {
		if s.ID == id {
			a.subs.subs = append(a.subs.subs[:i], a.subs.subs[i+1:]...)
			break
		}
	}
	a.subs.saveLocked()
	a.subs.mu.Unlock()
	a.subsChanged()
}

// SetSubscription pauses/resumes a subscription and changes how often it is checked.
func (a *App) SetSubscription(id string, enabled bool, everyHours int) {
	a.subs.mu.Lock()
	if s := a.subs.find(id); s != nil {
		s.Enabled = enabled
		switch everyHours {
		case 1, 3, 6, 12, 24, 72, 168:
			s.EveryHours = everyHours
		}
		a.subs.saveLocked()
	}
	a.subs.mu.Unlock()
	a.subsChanged()
}

// CheckSubscription looks for new items now (in the background).
func (a *App) CheckSubscription(id string) {
	go a.checkSub(id)
}

// checkSub reads a subscribed link and queues its new items.
func (a *App) checkSub(id string) {
	a.subs.mu.Lock()
	s := a.subs.find(id)
	if s == nil || a.subs.checking[id] {
		a.subs.mu.Unlock()
		return
	}
	a.subs.checking[id] = true
	url, opts, tabs := s.URL, s.Options, append([]string{}, s.Tabs...)
	seen := map[string]bool{}
	for _, v := range s.Seen {
		seen[v] = true
	}
	a.subs.mu.Unlock()
	a.subsChanged()

	var fresh []string
	var all []string
	col, err := a.AnalyzeLink(url)
	if err == nil {
		for _, e := range col.Entries {
			all = append(all, e.ID)
			if !seen[e.ID] && !e.Unavailable && !e.Archived && tabAllowed(tabs, e.Tab) {
				fresh = append(fresh, e.ID)
			}
		}
		if len(fresh) > 0 {
			_, err = a.StartDownloads(col.Key, fresh, opts)
		}
		a.ForgetCollection(col.Key)
	}

	a.subs.mu.Lock()
	delete(a.subs.checking, id)
	if s := a.subs.find(id); s != nil {
		s.LastCheck = time.Now().Unix()
		if err != nil {
			s.LastError = firstLineOf(err.Error())
		} else {
			s.LastError = ""
			s.LastNew = len(fresh)
			s.TotalNew += len(fresh)
			for _, v := range all {
				if !seen[v] {
					seen[v] = true
					s.Seen = append(s.Seen, v)
				}
			}
			// Keep the list bounded: IDs no longer listed can't come back as "new".
			if len(s.Seen) > 20000 {
				s.Seen = s.Seen[len(s.Seen)-20000:]
			}
		}
		title := s.Title
		a.subs.saveLocked()
		a.subs.mu.Unlock()
		if err == nil && len(fresh) > 0 {
			a.notify(i18n.L("Langganan", "Subscription"), fmt.Sprintf(i18n.L("%d item baru dari %s sedang diunduh", "%d new item(s) from %s are downloading"), len(fresh), title))
		}
	} else {
		a.subs.mu.Unlock()
	}
	a.subsChanged()
}

// runSubscriptions checks due subscriptions every few minutes while the app runs.
func (a *App) runSubscriptions(ctx context.Context) {
	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()
	first := time.After(30 * time.Second) // let the app settle after starting
	for {
		select {
		case <-ctx.Done():
			return
		case <-first:
		case <-tick.C:
		}
		now := time.Now().Unix()
		var due []string
		a.subs.mu.Lock()
		for _, s := range a.subs.subs {
			if s.Enabled && !a.subs.checking[s.ID] && now-s.LastCheck >= int64(s.EveryHours)*3600 {
				due = append(due, s.ID)
			}
		}
		a.subs.mu.Unlock()
		for _, id := range due {
			if ctx.Err() != nil {
				return
			}
			a.checkSub(id) // one at a time: sites dislike bursts
		}
	}
}

// notify shows a Windows notification when they are enabled.
func (a *App) notify(title, body string) {
	if !a.notifyOK || !a.cfg.Get().Notify || a.ctx == nil {
		return
	}
	_ = wruntime.SendNotification(a.ctx, wruntime.NotificationOptions{ID: fmt.Sprintf("kmb-%d", time.Now().UnixNano()), Title: title, Body: strings.TrimSpace(body)})
}
