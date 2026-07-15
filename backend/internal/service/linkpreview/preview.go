package linkpreview

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type Preview struct {
	URL         string `json:"url"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Image       string `json:"image,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
}

type Service struct {
	client *http.Client
	mu     sync.Mutex
	cache  map[string]cacheEntry
}

type cacheEntry struct {
	preview Preview
	at      time.Time
	ok      bool
	errMsg  string
}

const cacheTTL = 30 * time.Minute
const failCacheTTL = 2 * time.Minute

func New() *Service {
	return &Service{
		client: &http.Client{
			Timeout: 6 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return errors.New("too many redirects")
				}
				return assertSafeURL(req.URL)
			},
		},
		cache: map[string]cacheEntry{},
	}
}

func (s *Service) Fetch(ctx context.Context, raw string) (*Preview, error) {
	return s.FetchOpt(ctx, raw, false)
}

func (s *Service) FetchOpt(ctx context.Context, raw string, refresh bool) (*Preview, error) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, errors.New("链接无效")
	}
	if err := assertSafeURL(u); err != nil {
		return nil, err
	}
	key := u.String()
	if !refresh {
		s.mu.Lock()
		if e, ok := s.cache[key]; ok {
			if e.ok && time.Since(e.at) < cacheTTL {
				p := e.preview
				s.mu.Unlock()
				return &p, nil
			}
			if !e.ok && time.Since(e.at) < failCacheTTL {
				msg := e.errMsg
				s.mu.Unlock()
				if msg == "" {
					msg = "无法抓取该链接"
				}
				return nil, errors.New(msg)
			}
		}
		s.mu.Unlock()
	} else {
		s.mu.Lock()
		delete(s.cache, key)
		s.mu.Unlock()
	}

	p, err := s.fetchLive(ctx, u, key)
	s.mu.Lock()
	if err != nil {
		s.cache[key] = cacheEntry{at: time.Now(), ok: false, errMsg: err.Error()}
	} else {
		s.cache[key] = cacheEntry{preview: *p, at: time.Now(), ok: true}
	}
	s.mu.Unlock()
	return p, err
}

func (s *Service) fetchLive(ctx context.Context, u *url.URL, key string) (*Preview, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "SquirtleChat-LinkPreview/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml")

	res, err := s.client.Do(req)
	if err != nil {
		return nil, errors.New("无法抓取该链接")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 400 {
		return nil, errors.New("链接无法访问")
	}
	ct := res.Header.Get("Content-Type")
	if ct != "" && !strings.Contains(strings.ToLower(ct), "html") && !strings.Contains(strings.ToLower(ct), "text/") {
		return nil, errors.New("非网页链接")
	}
	body, err := io.ReadAll(io.LimitReader(res.Body, 512<<10))
	if err != nil {
		return nil, err
	}
	html := string(body)
	p := Preview{
		URL:         key,
		Title:       firstMeta(html, "og:title", "twitter:title"),
		Description: firstMeta(html, "og:description", "twitter:description", "description"),
		Image:       absURL(u, firstMeta(html, "og:image", "twitter:image")),
		SiteName:    firstMeta(html, "og:site_name"),
	}
	if p.Title == "" {
		p.Title = firstTitle(html)
	}
	if p.Title == "" {
		p.Title = u.Hostname()
	}
	return &p, nil
}

func assertSafeURL(u *url.URL) error {
	host := u.Hostname()
	if host == "" || strings.EqualFold(host, "localhost") {
		return errors.New("不允许的地址")
	}
	ips, err := net.LookupIP(host)
	if err != nil {
		return errors.New("无法解析域名")
	}
	for _, ip := range ips {
		if isPrivateIP(ip) {
			return errors.New("不允许访问内网地址")
		}
	}
	return nil
}

func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsPrivate() || ip.IsUnspecified() {
		return true
	}
	// AWS/GCP metadata
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 169 && ip4[1] == 254 {
			return true
		}
	}
	return false
}

var (
	metaProp = regexp.MustCompile(`(?i)<meta[^>]+(?:property|name)=["']([^"']+)["'][^>]+content=["']([^"']*)["'][^>]*>`)
	metaProp2 = regexp.MustCompile(`(?i)<meta[^>]+content=["']([^"']*)["'][^>]+(?:property|name)=["']([^"']+)["'][^>]*>`)
	titleRe   = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

func firstMeta(html string, keys ...string) string {
	want := map[string]bool{}
	for _, k := range keys {
		want[strings.ToLower(k)] = true
	}
	for _, m := range metaProp.FindAllStringSubmatch(html, -1) {
		if want[strings.ToLower(m[1])] {
			return strings.TrimSpace(htmlUnescape(m[2]))
		}
	}
	for _, m := range metaProp2.FindAllStringSubmatch(html, -1) {
		if want[strings.ToLower(m[2])] {
			return strings.TrimSpace(htmlUnescape(m[1]))
		}
	}
	return ""
}

func firstTitle(html string) string {
	m := titleRe.FindStringSubmatch(html)
	if len(m) < 2 {
		return ""
	}
	t := regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(htmlUnescape(m[1])), " ")
	if len([]rune(t)) > 120 {
		t = string([]rune(t)[:120]) + "…"
	}
	return t
}

func absURL(base *url.URL, raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return base.ResolveReference(u).String()
}

func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
	)
	return r.Replace(s)
}
