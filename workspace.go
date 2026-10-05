package main

import (
	"io/fs"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// The project folder, from the initialize request. Empty if the editor didn't send one.
var rootPath string

type InitializeParams struct {
	RootURI          string `json:"rootUri"`
	WorkspaceFolders []struct {
		URI string `json:"uri"`
	} `json:"workspaceFolders"`
}

// A file in the project: its URI and current text
type ProjectFile struct {
	URI  string
	Text string
}

// Converts /a/b c.patch to file:///a/b%20c.patch, the form editors use
func pathToURI(path string) string {
	return (&url.URL{Scheme: "file", Path: path}).String()
}

func uriToPath(uri string) string {
	u, err := url.Parse(uri)
	if err != nil {
		return ""
	}
	return u.Path
}

// Every .patch file under the project folder, plus any open file outside it.
// Open files use their text from the editor, which includes unsaved changes.
func projectFiles() []ProjectFile {
	byPath := map[string]ProjectFile{}

	if rootPath != "" {
		filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // unreadable folder: skip it and keep going
			}
			if d.IsDir() && path != rootPath && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir // .git and other hidden folders
			}
			if !d.IsDir() && filepath.Ext(path) == ".patch" {
				text, err := os.ReadFile(path)
				if err != nil {
					log.Println("couldn't read ", path, ": ", err)
					return nil
				}
				byPath[path] = ProjectFile{URI: pathToURI(path), Text: string(text)}
			}
			return nil
		})
	}

	for uri, text := range documents {
		byPath[uriToPath(uri)] = ProjectFile{URI: uri, Text: text}
	}

	files := []ProjectFile{}
	for _, f := range byPath {
		files = append(files, f)
	}
	return files
}
