package pypi

// Package is the record returned by the package command.
type Package struct {
	Rank    int    `json:"rank"`
	Name    string `json:"name"`
	Version string `json:"version"`
	Summary string `json:"summary"`
	Author  string `json:"author"`
	License string `json:"license"`
	Python  string `json:"requires_python"`
	Updated string `json:"updated"`
	URL     string `json:"url"`
}

// Release is one entry in the releases list for a package.
type Release struct {
	Version string `json:"version"`
	Date    string `json:"date"`
	Files   int    `json:"files"`
}

// File is one distribution file for a release.
type File struct {
	Filename    string `json:"filename"`
	PackageType string `json:"packagetype"`
	PythonVer   string `json:"python_version"`
	Size        int64  `json:"size"`
	UploadTime  string `json:"upload_time"`
	URL         string `json:"url"`
}

// Dep is one parsed dependency from requires_dist.
type Dep struct {
	Name       string `json:"name"`
	Constraint string `json:"constraint"`
	Extra      string `json:"extra"`
}

// Update is an entry from the RSS update or newest feeds.
type Update struct {
	Rank    int    `json:"rank"`
	Title   string `json:"title"`
	Version string `json:"version"`
	Date    string `json:"date"`
	URL     string `json:"url"`
}

// SearchResult is one hit from the PyPI HTML search page.
type SearchResult struct {
	Rank        int    `json:"rank"`
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
}
