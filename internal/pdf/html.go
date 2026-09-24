package pdf

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"kuymediabox/internal/i18n"
	"kuymediabox/internal/proc"
)

// HTMLOptions control web page → PDF.
type HTMLOptions struct {
	PageSize    string `json:"pageSize"`    // a4 letter legal f4 a3
	Orientation string `json:"orientation"` // portrait | landscape
	Margin      string `json:"margin"`      // none | small | normal
	Width       int    `json:"width"`       // browser window width in CSS pixels
	OnePage     bool   `json:"onePage"`     // the whole page on one long PDF page
	Background  bool   `json:"background"`  // print background colours and images
}

// Browser returns an installed Chromium browser (Edge or Chrome), or "".
func Browser() string {
	var cands []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		cands = append(cands,
			filepath.Join(base, "Microsoft", "Edge", "Application", "msedge.exe"),
			filepath.Join(base, "Google", "Chrome", "Application", "chrome.exe"),
		)
	}
	for _, c := range cands {
		if fileExists(c) {
			return c
		}
	}
	return ""
}

// NormalizeURL turns user input into a URL (adds https://, accepts local file paths).
func NormalizeURL(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", errors.New(i18n.L("alamat kosong", "empty address"))
	}
	if fileExists(s) {
		abs, _ := filepath.Abs(s)
		return (&url.URL{Scheme: "file", Path: "/" + filepath.ToSlash(abs)}).String(), nil
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" && u.Scheme != "file" {
		return "", fmt.Errorf(i18n.L("alamat tidak valid: %s", "invalid address: %s"), s)
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "file" {
		return "", fmt.Errorf(i18n.L("alamat tidak didukung: %s", "unsupported address: %s"), s)
	}
	return u.String(), nil
}

var reSlug = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// URLName suggests a file name for a web page.
func URLName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "halaman"
	}
	if u.Scheme == "file" {
		return strings.TrimSuffix(filepath.Base(u.Path), filepath.Ext(u.Path))
	}
	name := strings.TrimPrefix(u.Host, "www.")
	if p := strings.Trim(u.Path, "/"); p != "" {
		name += "-" + p
	}
	name = strings.Trim(reSlug.ReplaceAllString(name, "-"), "-")
	if r := []rune(name); len(r) > 80 {
		name = string(r[:80])
	}
	if name == "" {
		name = "halaman"
	}
	return name
}

type cdp struct {
	ws      *websocket.Conn
	mu      sync.Mutex
	nextID  int
	pending map[int]chan cdpMsg
	events  chan cdpMsg
	done    chan struct{}
}

type cdpMsg struct {
	ID        int             `json:"id"`
	Method    string          `json:"method"`
	SessionID string          `json:"sessionId"`
	Params    json.RawMessage `json:"params"`
	Result    json.RawMessage `json:"result"`
	Error     *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *cdp) readLoop() {
	defer close(c.done)
	for {
		var m cdpMsg
		if err := c.ws.ReadJSON(&m); err != nil {
			return
		}
		if m.ID != 0 {
			c.mu.Lock()
			ch := c.pending[m.ID]
			delete(c.pending, m.ID)
			c.mu.Unlock()
			if ch != nil {
				ch <- m
			}
			continue
		}
		select {
		case c.events <- m:
		default: // nobody listening; drop
		}
	}
}

func (c *cdp) call(ctx context.Context, session, method string, params any, out any) error {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	ch := make(chan cdpMsg, 1)
	c.pending[id] = ch
	msg := map[string]any{"id": id, "method": method, "params": params}
	if session != "" {
		msg["sessionId"] = session
	}
	err := c.ws.WriteJSON(msg)
	c.mu.Unlock()
	if err != nil {
		return err
	}
	select {
	case m := <-ch:
		if m.Error != nil {
			return fmt.Errorf("%s: %s", method, m.Error.Message)
		}
		if out != nil {
			return json.Unmarshal(m.Result, out)
		}
		return nil
	case <-c.done:
		return errors.New(i18n.L("browser tertutup", "the browser closed"))
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (c *cdp) waitEvent(ctx context.Context, method string) error {
	for {
		select {
		case m := <-c.events:
			if m.Method == method {
				return nil
			}
		case <-c.done:
			return errors.New(i18n.L("browser tertutup", "the browser closed"))
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

var reDevTools = regexp.MustCompile(`ws://[^\s]+`)

// HTMLToPDF loads a web page (or local HTML file) in a headless Edge/Chrome and prints it.
func HTMLToPDF(ctx context.Context, rawURL string, o HTMLOptions, out, tmpDir string) error {
	browser := Browser()
	if browser == "" {
		return errors.New(i18n.L("Microsoft Edge atau Google Chrome tidak ditemukan", "Microsoft Edge or Google Chrome was not found"))
	}
	target, err := NormalizeURL(rawURL)
	if err != nil {
		return err
	}
	profile, err := os.MkdirTemp(tmpDir, "browser-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(profile)
	ctx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	cmd := proc.Command(ctx, browser, "--headless=new", "--disable-gpu", "--no-first-run", "--no-default-browser-check",
		"--disable-extensions", "--hide-scrollbars", "--mute-audio", "--remote-debugging-port=0", "--user-data-dir="+profile, "about:blank")
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	defer func() {
		_ = cmd.Cancel()
		_ = cmd.Wait()
	}()
	wsURL := make(chan string, 1)
	go func() {
		sc := bufio.NewScanner(stderr)
		for sc.Scan() {
			if m := reDevTools.FindString(sc.Text()); m != "" {
				wsURL <- m
				break
			}
		}
		for sc.Scan() {
		}
	}()
	var endpoint string
	select {
	case endpoint = <-wsURL:
	case <-time.After(30 * time.Second):
		return errors.New(i18n.L("browser tidak merespons", "the browser didn't respond"))
	case <-ctx.Done():
		return ctx.Err()
	}
	ws, _, err := websocket.DefaultDialer.DialContext(ctx, endpoint, nil)
	if err != nil {
		return err
	}
	defer ws.Close()
	c := &cdp{ws: ws, pending: map[int]chan cdpMsg{}, events: make(chan cdpMsg, 256), done: make(chan struct{})}
	go c.readLoop()

	var created struct {
		TargetID string `json:"targetId"`
	}
	if err := c.call(ctx, "", "Target.createTarget", map[string]any{"url": "about:blank"}, &created); err != nil {
		return err
	}
	var attached struct {
		SessionID string `json:"sessionId"`
	}
	if err := c.call(ctx, "", "Target.attachToTarget", map[string]any{"targetId": created.TargetID, "flatten": true}, &attached); err != nil {
		return err
	}
	s := attached.SessionID
	if err := c.call(ctx, s, "Page.enable", map[string]any{}, nil); err != nil {
		return err
	}
	width := o.Width
	if width < 320 || width > 3840 {
		width = 1280
	}
	if err := c.call(ctx, s, "Emulation.setDeviceMetricsOverride", map[string]any{"width": width, "height": 900, "deviceScaleFactor": 1, "mobile": width < 700}, nil); err != nil {
		return err
	}
	var nav struct {
		ErrorText string `json:"errorText"`
	}
	loadCtx, loadCancel := context.WithTimeout(ctx, 60*time.Second)
	defer loadCancel()
	if err := c.call(loadCtx, s, "Page.navigate", map[string]any{"url": target}, &nav); err != nil {
		return err
	}
	if nav.ErrorText != "" {
		return fmt.Errorf(i18n.L("halaman tidak bisa dibuka (%s) — periksa alamat dan koneksi internet", "the page can't be opened (%s) — check the address and your internet connection"), nav.ErrorText)
	}
	if err := c.waitEvent(loadCtx, "Page.loadEventFired"); err != nil && ctx.Err() != nil {
		return ctx.Err()
	}
	// Scroll through the page so lazy images load, then give late content a moment.
	_ = c.call(ctx, s, "Runtime.evaluate", map[string]any{"expression": `(async()=>{for(let y=0;y<document.documentElement.scrollHeight;y+=innerHeight){scrollTo(0,y);await new Promise(r=>setTimeout(r,120))}scrollTo(0,0)})()`, "awaitPromise": true}, nil)
	time.Sleep(800 * time.Millisecond)

	size, ok := paper[o.PageSize]
	if !ok {
		size = paper["a4"]
	}
	pw, ph := size[0]/72, size[1]/72
	if o.Orientation == "landscape" {
		pw, ph = ph, pw
	}
	margin := map[string]float64{"none": 0, "small": 0.25, "normal": 0.5}[o.Margin]
	if _, known := map[string]bool{"none": true, "small": true, "normal": true}[o.Margin]; !known {
		margin = 0.4
	}
	params := map[string]any{
		"printBackground": o.Background, "landscape": false, "paperWidth": pw, "paperHeight": ph,
		"marginTop": margin, "marginBottom": margin, "marginLeft": margin, "marginRight": margin,
		"preferCSSPageSize": false,
	}
	if o.OnePage {
		var r struct {
			Result struct {
				Value float64 `json:"value"`
			} `json:"result"`
		}
		expr := `Math.max(document.documentElement.scrollHeight, document.body ? document.body.scrollHeight : 0)`
		if err := c.call(ctx, s, "Runtime.evaluate", map[string]any{"expression": expr, "returnByValue": true}, &r); err == nil && r.Result.Value > 0 {
			// CSS pixels are 1/96 inch; the page is printed at the window width.
			params["paperWidth"] = float64(width)/96 + 2*margin
			params["paperHeight"] = min(200, r.Result.Value/96+2*margin+0.1)
			params["scale"] = 1
		}
	}
	var printed struct {
		Data string `json:"data"`
	}
	if err := c.call(ctx, s, "Page.printToPDF", params, &printed); err != nil {
		return err
	}
	data, err := base64.StdEncoding.DecodeString(printed.Data)
	if err != nil {
		return err
	}
	_ = c.call(ctx, "", "Browser.close", map[string]any{}, nil)
	return os.WriteFile(out, data, 0o644)
}
