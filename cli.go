package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func runCLI(args []string) error {
	registry := newSourceRegistry()
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	ctx := context.Background()

	if len(args) == 0 {
		return runResume(root)
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp()
		return nil
	case "sources":
		return runSources(registry)
	case "resume":
		return runResume(root)
	case "open":
		return runOpen(ctx, registry, root, args[1:])
	case "library":
		return runLibrary(ctx, registry, root, args[1:])
	default:
		if strings.HasPrefix(args[0], "-") {
			return fmt.Errorf("unknown flag: %s", args[0])
		}
		// Backward compatible mode: treat first arg as a file path.
		return runOpen(ctx, registry, root, []string{args[0]})
	}
}

func runSources(reg sourceRegistry) error {
	fmt.Println("available sources:")
	for _, src := range reg.All() {
		fmt.Printf("  - %s (%s)\n", src.Name(), src.Status())
	}
	return nil
}

func runResume(projectRoot string) error {
	last, err := loadLastFilePath()
	if err != nil {
		return errors.New("no resume snapshot found, use: novel-reader open <txt-path>")
	}
	if !filepath.IsAbs(last) {
		last = filepath.Join(projectRoot, last)
	}
	if _, statErr := os.Stat(last); statErr != nil {
		return fmt.Errorf("resume file missing: %s", last)
	}
	return runReader(last)
}

func runOpen(ctx context.Context, reg sourceRegistry, projectRoot string, args []string) error {
	if len(args) == 0 {
		return errors.New("missing target, usage: novel-reader open <txt-path-or-id> [--source local]")
	}

	sourceName := "local"
	target := ""

	for i := 0; i < len(args); i++ {
		if args[i] == "--source" {
			if i+1 >= len(args) {
				return errors.New("missing source name after --source")
			}
			sourceName = args[i+1]
			i++
			continue
		}
		if target == "" {
			target = args[i]
		}
	}

	if target == "" {
		return errors.New("missing target, usage: novel-reader open <txt-path-or-id> [--source local]")
	}

	src, err := reg.Get(sourceName)
	if err != nil {
		return err
	}
	if src.Status() != "active" {
		return fmt.Errorf("source %s is reserved only", sourceName)
	}

	if idx, convErr := strconv.Atoi(target); convErr == nil {
		if idx <= 0 {
			return fmt.Errorf("invalid index: %d", idx)
		}
		cache, cacheErr := loadLastLibraryScan()
		if cacheErr != nil {
			return errors.New("no scan cache found, run: novel-reader library scan")
		}
		if idx > len(cache.Items) {
			return fmt.Errorf("index out of range: %d (max %d)", idx, len(cache.Items))
		}
		selected := cache.Items[idx-1]
		return runReader(selected.Path)
	}

	item, err := src.Resolve(ctx, projectRoot, target)
	if err != nil {
		return err
	}
	return runReader(item.Path)
}

func runLibrary(ctx context.Context, reg sourceRegistry, projectRoot string, args []string) error {
	if len(args) == 0 || args[0] == "scan" {
		dir := projectRoot
		if len(args) > 1 {
			dir = args[1]
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(projectRoot, dir)
			}
		}
		src, err := reg.Get("local")
		if err != nil {
			return err
		}
		items, err := src.List(ctx, dir)
		if err != nil {
			return err
		}
		if len(items) == 0 {
			fmt.Printf("no txt files found under %s\n", dir)
			return nil
		}
		fmt.Printf("found %d txt files under %s\n", len(items), dir)
		for i, it := range items {
			fmt.Printf("%3d. %s\n", i+1, it.ID)
		}
		if saveErr := saveLastLibraryScan(dir, items); saveErr != nil {
			fmt.Printf("warning: failed to save scan cache: %v\n", saveErr)
		}
		fmt.Println("tip: use 'novel-reader open <index>' to open by sequence.")
		return nil
	}
	return errors.New("unknown library command, usage: novel-reader library scan [dir]")
}

func printHelp() {
	fmt.Println("Novel Reader CLI")
	fmt.Println("")
	fmt.Println("Usage:")
	fmt.Println("  novel-reader help")
	fmt.Println("  novel-reader sources")
	fmt.Println("  novel-reader resume")
	fmt.Println("  novel-reader open <txt-path-or-id> [--source local]")
	fmt.Println("  novel-reader library scan [dir]")
	fmt.Println("")
	fmt.Println("Examples:")
	fmt.Println("  novel-reader open examples/demo.txt")
	fmt.Println("  novel-reader library scan")
	fmt.Println("  novel-reader resume")
}
