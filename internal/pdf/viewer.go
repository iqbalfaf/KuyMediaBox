package pdf

import (
	"bytes"
	"container/list"
	"context"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disintegration/imaging"
	"github.com/klippa-app/go-pdfium"

	"kuymediabox/internal/imageconv"
)

// viewer renders pages for the UI (thumbnails and the page editors). It keeps one engine
// instance with a few documents open, so browsing a document stays fast.
var viewer = &viewerT{}

type viewerT struct {
	mu    sync.Mutex
	inst  pdfium.Pdfium
	docs  []*openDoc // most recently used first
	pwMu  sync.Mutex
	pws   map[string]string
	cache imgCache
}

type openDoc struct {
	key string
	doc *Doc
}

const maxViewerDocs = 4

func (v *viewerT) with(ctx context.Context, fn func(inst pdfium.Pdfium) error) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.inst == nil {
		inst, err := instance(ctx)
		if err != nil {
			return err
		}
		v.inst = inst
	}
	return fn(v.inst)
}

func fileKey(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return strings.ToLower(filepath.Clean(path))
	}
	return fmt.Sprintf("%s|%d|%d", strings.ToLower(filepath.Clean(path)), st.Size(), st.ModTime().UnixNano())
}

// SetPassword remembers the password of a locked document for previews.
func SetPassword(path, password string) {
	viewer.pwMu.Lock()
	defer viewer.pwMu.Unlock()
	if viewer.pws == nil {
		viewer.pws = map[string]string{}
	}
	viewer.pws[strings.ToLower(filepath.Clean(path))] = password
}

// PasswordFor returns a password remembered with SetPassword.
func PasswordFor(path string) string {
	viewer.pwMu.Lock()
	defer viewer.pwMu.Unlock()
	return viewer.pws[strings.ToLower(filepath.Clean(path))]
}

// doc returns a cached open document; call only inside with().
func (v *viewerT) doc(path string) (*Doc, error) {
	key := fileKey(path)
	for i, od := range v.docs {
		if od.key == key {
			copy(v.docs[1:i+1], v.docs[:i])
			v.docs[0] = od
			return od.doc, nil
		}
	}
	d, err := openWith(v.inst, path, PasswordFor(path), true)
	if err != nil {
		return nil, err
	}
	v.docs = append([]*openDoc{{key: key, doc: d}}, v.docs...)
	if len(v.docs) > maxViewerDocs {
		v.docs[len(v.docs)-1].doc.Close()
		v.docs = v.docs[:maxViewerDocs]
	}
	return d, nil
}

// PageSize is one displayed page size in points.
type PageSize struct {
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// DocInfo lists the pages of a document for the page editors.
type DocInfo struct {
	Path      string     `json:"path"`
	Name      string     `json:"name"`
	Pages     []PageSize `json:"pages"`
	Encrypted bool       `json:"encrypted"`
}

// Describe opens a document for the page editors.
func Describe(ctx context.Context, path string) (DocInfo, error) {
	info := DocInfo{Path: path, Name: filepath.Base(path)}
	err := viewer.with(ctx, func(pdfium.Pdfium) error {
		d, err := viewer.doc(path)
		if err != nil {
			return err
		}
		info.Encrypted = d.Encrypted()
		for i := 0; i < d.Pages(); i++ {
			w, h, err := d.Size(i)
			if err != nil {
				return err
			}
			info.Pages = append(info.Pages, PageSize{w, h})
		}
		return nil
	})
	return info, err
}

// WithViewerDoc runs fn with a cached document of the viewer.
func WithViewerDoc(ctx context.Context, path string, fn func(d *Doc) error) error {
	return viewer.with(ctx, func(pdfium.Pdfium) error {
		d, err := viewer.doc(path)
		if err != nil {
			return err
		}
		return fn(d)
	})
}

// renderJPEG renders one page (0-based) to a JPEG that is w pixels wide.
func renderJPEG(ctx context.Context, path string, page, w int) ([]byte, error) {
	key := fmt.Sprintf("p|%s|%d|%d", fileKey(path), page, w)
	if b := viewer.cache.get(key); b != nil {
		return b, nil
	}
	var img image.Image
	err := WithViewerDoc(ctx, path, func(d *Doc) error {
		if page < 0 || page >= d.Pages() {
			return errors.New("page out of range")
		}
		var err error
		img, err = d.Render(page, w, w*4)
		return err
	})
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flattenWhite(img), &jpeg.Options{Quality: 85}); err != nil {
		return nil, err
	}
	viewer.cache.put(key, buf.Bytes())
	return buf.Bytes(), nil
}

// imageThumb decodes an image file into a small JPEG preview.
func imageThumb(ctx context.Context, path string, w int) ([]byte, error) {
	key := fmt.Sprintf("i|%s|%d", fileKey(path), w)
	if b := viewer.cache.get(key); b != nil {
		return b, nil
	}
	img, err := imageconv.Decode(ctx, path, "")
	if err != nil {
		return nil, err
	}
	if img.Bounds().Dx() > w {
		img = imaging.Resize(img, w, 0, imaging.Linear)
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, flattenWhite(img), &jpeg.Options{Quality: 82}); err != nil {
		return nil, err
	}
	viewer.cache.put(key, buf.Bytes())
	return buf.Bytes(), nil
}

// Middleware serves page previews at /kmb/page and image previews at /kmb/img.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.URL.Path, "/kmb/") {
			next.ServeHTTP(w, r)
			return
		}
		q := r.URL.Query()
		path := q.Get("path")
		width, _ := strconv.Atoi(q.Get("w"))
		if width <= 0 {
			width = 200
		}
		width = min(width, 2400)
		ctx, cancel := context.WithTimeout(r.Context(), time.Minute)
		defer cancel()
		var data []byte
		var err error
		switch r.URL.Path {
		case "/kmb/page":
			page, _ := strconv.Atoi(q.Get("i"))
			data, err = renderJPEG(ctx, path, page, width)
		case "/kmb/img":
			data, err = imageThumb(ctx, path, width)
		default:
			http.NotFound(w, r)
			return
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnprocessableEntity)
			return
		}
		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(data)
	})
}

// imgCache is a small LRU of rendered previews (bounded by total bytes).
type imgCache struct {
	mu    sync.Mutex
	ll    list.List
	items map[string]*list.Element
	size  int
}

type cacheEntry struct {
	key  string
	data []byte
}

const cacheLimit = 48 << 20

func (c *imgCache) get(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.items[key]; ok {
		c.ll.MoveToFront(e)
		return e.Value.(*cacheEntry).data
	}
	return nil
}

func (c *imgCache) put(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.items == nil {
		c.items = map[string]*list.Element{}
	}
	if _, ok := c.items[key]; ok {
		return
	}
	c.items[key] = c.ll.PushFront(&cacheEntry{key, data})
	c.size += len(data)
	for c.size > cacheLimit && c.ll.Len() > 1 {
		e := c.ll.Back()
		ce := e.Value.(*cacheEntry)
		c.ll.Remove(e)
		delete(c.items, ce.key)
		c.size -= len(ce.data)
	}
}
