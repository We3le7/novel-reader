package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var errSourceNotImplemented = errors.New("source is reserved but not implemented yet")

type novelItem struct {
	ID     string
	Title  string
	Path   string
	Source string
}

type novelSource interface {
	Name() string
	Status() string
	List(ctx context.Context, rootDir string) ([]novelItem, error)
	Resolve(ctx context.Context, rootDir, query string) (novelItem, error)
}

type localTXTSource struct{}

func (s localTXTSource) Name() string   { return "local" }
func (s localTXTSource) Status() string { return "active" }

func (s localTXTSource) List(_ context.Context, rootDir string) ([]novelItem, error) {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return nil, err
	}
	items := make([]novelItem, 0, 64)
	err = filepath.WalkDir(rootAbs, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(filepath.Ext(d.Name()), ".txt") {
			rel, err := filepath.Rel(rootAbs, path)
			if err != nil {
				return err
			}
			id := filepath.ToSlash(rel)
			items = append(items, novelItem{
				ID:     id,
				Title:  strings.TrimSuffix(d.Name(), filepath.Ext(d.Name())),
				Path:   path,
				Source: s.Name(),
			})
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].ID < items[j].ID
	})
	return items, nil
}

func (s localTXTSource) Resolve(ctx context.Context, rootDir, query string) (novelItem, error) {
	rootAbs, err := filepath.Abs(rootDir)
	if err != nil {
		return novelItem{}, err
	}

	tryPath := query
	if !filepath.IsAbs(tryPath) {
		tryPath = filepath.Join(rootAbs, query)
	}
	tryAbs, err := filepath.Abs(tryPath)
	if err == nil {
		if fileInfo, statErr := os.Stat(tryAbs); statErr == nil && !fileInfo.IsDir() && strings.EqualFold(filepath.Ext(fileInfo.Name()), ".txt") {
			if !isWithinRoot(rootAbs, tryAbs) {
				return novelItem{}, fmt.Errorf("file is outside project root: %s", tryAbs)
			}
			rel, relErr := filepath.Rel(rootAbs, tryAbs)
			if relErr != nil {
				return novelItem{}, relErr
			}
			id := filepath.ToSlash(rel)
			return novelItem{ID: id, Title: strings.TrimSuffix(filepath.Base(tryAbs), ".txt"), Path: tryAbs, Source: s.Name()}, nil
		}
	}

	items, err := s.List(ctx, rootAbs)
	if err != nil {
		return novelItem{}, err
	}
	normalized := filepath.ToSlash(strings.TrimPrefix(query, "./"))
	for _, it := range items {
		if it.ID == normalized || strings.EqualFold(it.Title, query) {
			return it, nil
		}
	}

	return novelItem{}, fmt.Errorf("txt file not found in project source: %s", query)
}

type placeholderSource struct {
	name string
}

func (s placeholderSource) Name() string   { return s.name }
func (s placeholderSource) Status() string { return "reserved" }

func (s placeholderSource) List(context.Context, string) ([]novelItem, error) {
	return nil, errSourceNotImplemented
}

func (s placeholderSource) Resolve(context.Context, string, string) (novelItem, error) {
	return novelItem{}, errSourceNotImplemented
}

type sourceRegistry struct {
	sources map[string]novelSource
}

func newSourceRegistry() sourceRegistry {
	return sourceRegistry{sources: map[string]novelSource{
		"local":     localTXTSource{},
		"gutenberg": placeholderSource{name: "gutenberg"},
		"custom":    placeholderSource{name: "custom"},
	}}
}

func (r sourceRegistry) Get(name string) (novelSource, error) {
	src, ok := r.sources[name]
	if !ok {
		return nil, fmt.Errorf("unknown source: %s", name)
	}
	return src, nil
}

func (r sourceRegistry) All() []novelSource {
	keys := make([]string, 0, len(r.sources))
	for k := range r.sources {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	list := make([]novelSource, 0, len(keys))
	for _, k := range keys {
		list = append(list, r.sources[k])
	}
	return list
}

func isWithinRoot(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	rel = filepath.ToSlash(rel)
	return rel != ".." && !strings.HasPrefix(rel, "../")
}
