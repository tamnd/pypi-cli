package pypi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/tamnd/pypi-cli/pypi"
)

func newTestClient(t *testing.T, mux *http.ServeMux) *pypi.Client {
	t.Helper()
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	cfg := pypi.DefaultConfig()
	cfg.BaseURL = ts.URL
	cfg.Rate = 0
	return pypi.NewClient(cfg)
}

func TestPackage(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/requests/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info": map[string]any{
				"name": "requests", "version": "2.31.0",
				"summary": "HTTP library", "author": "Kenneth Reitz",
				"license": "Apache 2.0", "requires_python": ">=3.7",
				"home_page": "https://requests.readthedocs.io",
			},
			"releases": map[string]any{},
			"urls":     []any{},
		})
	})
	c := newTestClient(t, mux)
	pkg, err := c.Package(context.Background(), "requests")
	if err != nil {
		t.Fatal(err)
	}
	if pkg.Name != "requests" {
		t.Errorf("got %q, want %q", pkg.Name, "requests")
	}
	if pkg.Version != "2.31.0" {
		t.Errorf("version got %q, want %q", pkg.Version, "2.31.0")
	}
}

func TestPackage404(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/nonexistent/json", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	c := newTestClient(t, mux)
	_, err := c.Package(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
}

func TestReleases(t *testing.T) {
	payload := map[string]any{
		"info": map[string]any{"name": "mylib", "version": "2.0.0"},
		"releases": map[string]any{
			"1.0.0": []map[string]any{
				{"filename": "mylib-1.0.0.tar.gz", "upload_time": "2023-01-01T10:00:00",
					"packagetype": "sdist", "python_version": "source", "size": 1000,
					"url": "https://example.com/1.0.0"},
			},
			"2.0.0": []map[string]any{
				{"filename": "mylib-2.0.0.tar.gz", "upload_time": "2024-06-01T10:00:00",
					"packagetype": "sdist", "python_version": "source", "size": 2000,
					"url": "https://example.com/2.0.0"},
			},
			"1.5.0": []map[string]any{
				{"filename": "mylib-1.5.0.tar.gz", "upload_time": "2023-12-01T10:00:00",
					"packagetype": "sdist", "python_version": "source", "size": 1500,
					"url": "https://example.com/1.5.0"},
			},
		},
		"urls": []any{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/mylib/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(payload)
	})
	c := newTestClient(t, mux)
	releases, err := c.Releases(context.Background(), "mylib", 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(releases) != 3 {
		t.Fatalf("got %d releases, want 3", len(releases))
	}
	// Newest first — 2.0.0 (2024) should be first.
	if releases[0].Version != "2.0.0" {
		t.Errorf("first release = %q, want 2.0.0", releases[0].Version)
	}
}

func TestFiles(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/requests/2.31.0/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info":     map[string]any{"name": "requests", "version": "2.31.0"},
			"releases": map[string]any{},
			"urls": []map[string]any{
				{"filename": "requests-2.31.0-py3-none-any.whl", "packagetype": "bdist_wheel",
					"python_version": "py3", "size": 62574,
					"upload_time": "2023-05-22T14:00:00",
					"url":         "https://files.pythonhosted.org/whl"},
				{"filename": "requests-2.31.0.tar.gz", "packagetype": "sdist",
					"python_version": "source", "size": 130508,
					"upload_time": "2023-05-22T14:00:00",
					"url":         "https://files.pythonhosted.org/tar"},
			},
		})
	})
	c := newTestClient(t, mux)
	files, err := c.Files(context.Background(), "requests", "2.31.0")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 2 {
		t.Fatalf("got %d files, want 2", len(files))
	}
	if files[0].PackageType != "bdist_wheel" {
		t.Errorf("packagetype = %q, want bdist_wheel", files[0].PackageType)
	}
}

func TestDeps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/pypi/requests/json", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"info": map[string]any{
				"name": "requests", "version": "2.31.0",
				"requires_dist": []string{
					"charset-normalizer (>=2,<4)",
					"idna (>=2.5,<4)",
					"urllib3 (>=1.21.1,<3)",
					"certifi (>=2017.4.17)",
				},
			},
			"releases": map[string]any{},
			"urls":     []any{},
		})
	})
	c := newTestClient(t, mux)
	deps, err := c.Deps(context.Background(), "requests")
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 4 {
		t.Fatalf("got %d deps, want 4", len(deps))
	}
	if deps[0].Name != "charset-normalizer" {
		t.Errorf("first dep name = %q, want charset-normalizer", deps[0].Name)
	}
}

func TestUpdates(t *testing.T) {
	rssBody := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>PyPI recent updates</title>
    <item>
      <title>requests 2.32.3</title>
      <link>https://pypi.org/project/requests/2.32.3/</link>
      <pubDate>Wed, 29 May 2024 15:37:49 GMT</pubDate>
      <description>Python HTTP for Humans.</description>
    </item>
    <item>
      <title>flask 3.0.3</title>
      <link>https://pypi.org/project/flask/3.0.3/</link>
      <pubDate>Mon, 13 May 2024 12:00:00 GMT</pubDate>
      <description>A simple framework for building complex web applications.</description>
    </item>
  </channel>
</rss>`
	mux := http.NewServeMux()
	mux.HandleFunc("/rss/updates.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(rssBody))
	})
	c := newTestClient(t, mux)
	updates, err := c.Updates(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(updates) != 2 {
		t.Fatalf("got %d updates, want 2", len(updates))
	}
	if updates[0].Title != "requests 2.32.3" {
		t.Errorf("title = %q, want 'requests 2.32.3'", updates[0].Title)
	}
	if updates[0].Version != "2.32.3" {
		t.Errorf("version = %q, want 2.32.3", updates[0].Version)
	}
}

func TestNewest(t *testing.T) {
	rssBody := `<?xml version="1.0" encoding="UTF-8"?>
<rss version="2.0">
  <channel>
    <title>Newest PyPI packages</title>
    <item>
      <title>newpkg 0.1.0</title>
      <link>https://pypi.org/project/newpkg/0.1.0/</link>
      <pubDate>Sun, 14 Jun 2026 08:00:00 GMT</pubDate>
      <description>A brand new package.</description>
    </item>
  </channel>
</rss>`
	mux := http.NewServeMux()
	mux.HandleFunc("/rss/packages.xml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(rssBody))
	})
	c := newTestClient(t, mux)
	newest, err := c.Newest(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(newest) != 1 {
		t.Fatalf("got %d entries, want 1", len(newest))
	}
	if newest[0].Version != "0.1.0" {
		t.Errorf("version = %q, want 0.1.0", newest[0].Version)
	}
}
