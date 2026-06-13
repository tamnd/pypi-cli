// Package pypi is the library behind the pypi command line:
// the HTTP client, request shaping, and typed data models for the Python
// Package Index.
//
// Three public APIs: the JSON API at pypi.org/pypi/{name}/json for package
// metadata, HTML search at pypi.org/search/ for full-text search, and two
// RSS feeds for recent updates. All are open and require no key.
package pypi

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

// DefaultUserAgent identifies the client to PyPI.
const DefaultUserAgent = "pypi/dev (+https://github.com/tamnd/pypi-cli)"

// ErrNotFound is returned when the PyPI API returns a 404 for a package or version.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters.
type Config struct {
	BaseURL   string
	UserAgent string
	Rate      time.Duration
	Retries   int
	Workers   int
	Timeout   time.Duration
}

// DefaultConfig returns sensible defaults for the PyPI client.
func DefaultConfig() Config {
	return Config{
		BaseURL:   "https://pypi.org",
		UserAgent: DefaultUserAgent,
		Rate:      200 * time.Millisecond,
		Retries:   3,
		Workers:   8,
		Timeout:   30 * time.Second,
	}
}

// Client talks to the PyPI JSON API, search HTML, and RSS feeds.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
	rate       time.Duration
	retries    int
	workers    int
	mu         sync.Mutex
	last       time.Time
}

// NewClient returns a Client with the given config.
func NewClient(cfg Config) *Client {
	base := cfg.BaseURL
	if base == "" {
		base = "https://pypi.org"
	}
	return &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    base,
		userAgent:  cfg.UserAgent,
		rate:       cfg.Rate,
		retries:    cfg.Retries,
		workers:    cfg.Workers,
	}
}

// get fetches a URL with pacing and retries.
func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	return c.getWithHeader(ctx, rawURL, "")
}

func (c *Client) getWithHeader(ctx context.Context, rawURL, accept string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, rawURL, accept)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", rawURL, lastErr)
}

func (c *Client) do(ctx context.Context, rawURL, accept string) ([]byte, bool, error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	} else {
		req.Header.Set("Accept", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, false, fmt.Errorf("http 404")
	}
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.rate <= 0 {
		return
	}
	if wait := c.rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// getJSON fetches and JSON-decodes into v.
func (c *Client) getJSON(ctx context.Context, rawURL string, v any) error {
	body, err := c.get(ctx, rawURL)
	if err != nil {
		if strings.Contains(err.Error(), "http 404") {
			return ErrNotFound
		}
		return err
	}
	if strings.TrimSpace(string(body)) == "null" {
		return ErrNotFound
	}
	if err := json.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode %s: %w", rawURL, err)
	}
	return nil
}

// getHTML fetches a URL and returns the body as a string.
func (c *Client) getHTML(ctx context.Context, rawURL string) (string, error) {
	body, err := c.getWithHeader(ctx, rawURL, "text/html,application/xhtml+xml")
	if err != nil {
		return "", err
	}
	return string(body), nil
}

// getXML fetches a URL and XML-decodes into v.
func (c *Client) getXML(ctx context.Context, rawURL string, v any) error {
	body, err := c.getWithHeader(ctx, rawURL, "application/xml,text/xml")
	if err != nil {
		return err
	}
	if err := xml.Unmarshal(body, v); err != nil {
		return fmt.Errorf("decode xml %s: %w", rawURL, err)
	}
	return nil
}

// ─── wire types ──────────────────────────────────────────────────────────────

type pypiInfo struct {
	Name           string            `json:"name"`
	Version        string            `json:"version"`
	Summary        string            `json:"summary"`
	Author         string            `json:"author"`
	AuthorEmail    string            `json:"author_email"`
	License        string            `json:"license"`
	RequiresPython string            `json:"requires_python"`
	HomePage       string            `json:"home_page"`
	ProjectURLs    map[string]string `json:"project_urls"`
	RequiresDist   []string          `json:"requires_dist"`
}

type pypiFileWire struct {
	Filename    string `json:"filename"`
	PackageType string `json:"packagetype"`
	PythonVer   string `json:"python_version"`
	Size        int64  `json:"size"`
	UploadTime  string `json:"upload_time"`
	URL         string `json:"url"`
}

type pypiResponse struct {
	Info     pypiInfo                  `json:"info"`
	Releases map[string][]pypiFileWire `json:"releases"`
	URLs     []pypiFileWire            `json:"urls"`
}

type rssItem struct {
	Title   string `xml:"title"`
	Link    string `xml:"link"`
	PubDate string `xml:"pubDate"`
	Desc    string `xml:"description"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

// ─── public methods ───────────────────────────────────────────────────────────

// Package returns the latest-version metadata for the named package.
func (c *Client) Package(ctx context.Context, name string) (Package, error) {
	var resp pypiResponse
	u := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, url.PathEscape(name))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return Package{}, err
	}
	info := resp.Info
	author := info.Author
	if author == "" {
		author = info.AuthorEmail
	}
	updated := ""
	if len(resp.URLs) > 0 {
		updated = formatUploadTime(resp.URLs[0].UploadTime)
	}
	return Package{
		Rank:    1,
		Name:    info.Name,
		Version: info.Version,
		Summary: info.Summary,
		Author:  author,
		License: info.License,
		Python:  info.RequiresPython,
		Updated: updated,
		URL:     fmt.Sprintf("https://pypi.org/project/%s/", url.PathEscape(info.Name)),
	}, nil
}

// Releases returns all releases for the named package sorted newest-first.
func (c *Client) Releases(ctx context.Context, name string, limit int) ([]Release, error) {
	var resp pypiResponse
	u := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, url.PathEscape(name))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}

	type entry struct {
		version string
		date    string
		files   int
	}
	var entries []entry
	for ver, files := range resp.Releases {
		if len(files) == 0 {
			continue
		}
		entries = append(entries, entry{
			version: ver,
			date:    formatUploadTime(files[0].UploadTime),
			files:   len(files),
		})
	}
	// Sort newest-first: ISO 8601 dates sort lexicographically.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].date > entries[j].date
	})

	if limit > 0 && limit < len(entries) {
		entries = entries[:limit]
	}
	out := make([]Release, len(entries))
	for i, e := range entries {
		out[i] = Release{
			Version: e.version,
			Date:    e.date,
			Files:   e.files,
		}
	}
	return out, nil
}

// Files returns the distribution files for a specific release.
// If version is empty, uses the latest version.
func (c *Client) Files(ctx context.Context, name, version string) ([]File, error) {
	if version == "" {
		// Get latest version from the package endpoint first.
		var resp pypiResponse
		u := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, url.PathEscape(name))
		if err := c.getJSON(ctx, u, &resp); err != nil {
			return nil, err
		}
		version = resp.Info.Version
	}

	var resp pypiResponse
	u := fmt.Sprintf("%s/pypi/%s/%s/json", c.baseURL, url.PathEscape(name), url.PathEscape(version))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	out := make([]File, len(resp.URLs))
	for i, f := range resp.URLs {
		out[i] = File(f)
	}
	return out, nil
}

// Deps returns the declared dependencies for the latest release.
func (c *Client) Deps(ctx context.Context, name string) ([]Dep, error) {
	var resp pypiResponse
	u := fmt.Sprintf("%s/pypi/%s/json", c.baseURL, url.PathEscape(name))
	if err := c.getJSON(ctx, u, &resp); err != nil {
		return nil, err
	}
	var out []Dep
	for _, req := range resp.Info.RequiresDist {
		out = append(out, parseDep(req))
	}
	return out, nil
}

// Updates returns recent package updates from the RSS feed.
func (c *Client) Updates(ctx context.Context, limit int) ([]Update, error) {
	return c.fetchRSS(ctx, c.baseURL+"/rss/updates.xml", limit)
}

// Newest returns the newest packages added to PyPI.
func (c *Client) Newest(ctx context.Context, limit int) ([]Update, error) {
	return c.fetchRSS(ctx, c.baseURL+"/rss/packages.xml", limit)
}

func (c *Client) fetchRSS(ctx context.Context, feedURL string, limit int) ([]Update, error) {
	var feed rssFeed
	if err := c.getXML(ctx, feedURL, &feed); err != nil {
		return nil, err
	}
	items := feed.Channel.Items
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}
	out := make([]Update, len(items))
	for i, item := range items {
		version := ""
		if parts := strings.Fields(item.Title); len(parts) > 0 {
			version = parts[len(parts)-1]
		}
		date := parsePubDate(item.PubDate)
		out[i] = Update{
			Rank:    i + 1,
			Title:   item.Title,
			Version: version,
			Date:    date,
			URL:     item.Link,
		}
	}
	return out, nil
}

// Search searches PyPI by scraping the HTML search results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = 20
	}
	var out []SearchResult
	for page := 1; len(out) < limit; page++ {
		u := fmt.Sprintf("%s/search/?q=%s&page=%d", c.baseURL, url.QueryEscape(query), page)
		html, err := c.getHTML(ctx, u)
		if err != nil {
			if len(out) > 0 {
				break
			}
			return nil, err
		}
		snippets := parseSearchSnippets(html, len(out)+1)
		if len(snippets) == 0 {
			break
		}
		out = append(out, snippets...)
		if len(out) >= limit {
			break
		}
	}
	if limit > 0 && limit < len(out) {
		out = out[:limit]
	}
	return out, nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

// parseSearchSnippets extracts package-snippet records from PyPI search HTML.
// startRank is the rank of the first result found.
func parseSearchSnippets(html string, startRank int) []SearchResult {
	var out []SearchResult
	rank := startRank
	remaining := html
	anchor := `class="package-snippet"`
	for {
		idx := strings.Index(remaining, anchor)
		if idx < 0 {
			break
		}
		// Find the end of this snippet: next anchor or </ul>.
		rest := remaining[idx:]
		end := strings.Index(rest[len(anchor):], anchor)
		var block string
		if end < 0 {
			block = rest
		} else {
			block = rest[:end+len(anchor)]
		}
		remaining = remaining[idx+len(anchor):]

		name := extractSpanContent(block, "package-snippet__name")
		version := extractSpanContent(block, "package-snippet__version")
		desc := extractSpanContent(block, "package-snippet__description")
		if name == "" {
			continue
		}
		out = append(out, SearchResult{
			Rank:        rank,
			Name:        name,
			Version:     version,
			Description: strings.TrimSpace(desc),
			URL:         fmt.Sprintf("https://pypi.org/project/%s/", url.PathEscape(name)),
		})
		rank++
	}
	return out
}

// extractSpanContent finds the content of a span with the given class substring.
func extractSpanContent(block, class string) string {
	marker := `class="` + class + `"`
	idx := strings.Index(block, marker)
	if idx < 0 {
		return ""
	}
	rest := block[idx:]
	open := strings.Index(rest, ">")
	if open < 0 {
		return ""
	}
	rest = rest[open+1:]
	close := strings.Index(rest, "<")
	if close < 0 {
		return ""
	}
	return strings.TrimSpace(rest[:close])
}

// parseDep parses a PEP 508 requirement string into a Dep.
func parseDep(s string) Dep {
	extra := ""
	parts := strings.SplitN(s, ";", 2)
	reqStr := strings.TrimSpace(parts[0])
	if len(parts) == 2 {
		marker := parts[1]
		if strings.Contains(marker, "extra ==") {
			// Extract the quoted value after 'extra =='
			markerStr := strings.TrimSpace(marker)
			eqIdx := strings.Index(markerStr, "extra ==")
			if eqIdx >= 0 {
				after := strings.TrimSpace(markerStr[eqIdx+len("extra =="):])
				// Strip enclosing quotes
				if len(after) >= 2 && (after[0] == '"' || after[0] == '\'') {
					q := after[0]
					endQ := strings.IndexByte(after[1:], q)
					if endQ >= 0 {
						extra = after[1 : endQ+1]
					}
				}
			}
		}
	}

	// Split name from constraint.
	// PEP 508 allows two forms:
	//   "requests (>=2.0)"  — parenthesized constraint
	//   "requests>=2.0"     — bare constraint (newer style)
	name := reqStr
	constraint := ""
	parenIdx := strings.Index(reqStr, "(")
	if parenIdx >= 0 {
		name = strings.TrimSpace(reqStr[:parenIdx])
		constraint = strings.TrimSpace(reqStr[parenIdx:])
		// Remove enclosing parens
		if strings.HasPrefix(constraint, "(") && strings.HasSuffix(constraint, ")") {
			constraint = constraint[1 : len(constraint)-1]
		}
	} else {
		// Find the first version operator by scanning for the earliest occurrence.
		// Two-char operators must be checked before single-char to avoid splitting ">=".
		firstIdx := -1
		for _, op := range []string{"!=", "~=", ">=", "<=", "==", "<", ">"} {
			idx := strings.Index(reqStr, op)
			if idx > 0 && (firstIdx < 0 || idx < firstIdx) {
				firstIdx = idx
			}
		}
		if firstIdx > 0 {
			name = strings.TrimSpace(reqStr[:firstIdx])
			constraint = strings.TrimSpace(reqStr[firstIdx:])
		}
	}

	// Strip extras from name like 'requests[socks]'
	if bracketIdx := strings.Index(name, "["); bracketIdx >= 0 {
		name = strings.TrimSpace(name[:bracketIdx])
	}

	return Dep{
		Name:       name,
		Constraint: constraint,
		Extra:      extra,
	}
}

// formatUploadTime converts PyPI's upload_time to RFC 3339.
// PyPI returns "2024-05-29T15:37:49" (no timezone, implicitly UTC).
func formatUploadTime(s string) string {
	if s == "" {
		return ""
	}
	// Try parsing with 'T' separator
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return s
	}
	return t.UTC().Format(time.RFC3339)
}

// parsePubDate converts RSS pubDate (RFC 1123) to RFC 3339.
func parsePubDate(s string) string {
	if s == "" {
		return ""
	}
	layouts := []string{time.RFC1123, time.RFC1123Z, "Mon, 02 Jan 2006 15:04:05 -0700"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, s)
		if err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}
