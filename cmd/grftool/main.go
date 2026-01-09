// grftool is a CLI utility for working with Ragnarok Online GRF archives.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Faultbox/midgard-ro/pkg/grf"
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]
	args := os.Args[2:]

	switch command {
	case "info":
		cmdInfo(args)
	case "list", "ls":
		cmdList(args)
	case "extract", "x":
		cmdExtract(args)
	case "search", "find":
		cmdSearch(args)
	case "help", "-h", "--help":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`grftool - Ragnarok Online GRF archive utility

Usage:
  grftool <command> [options]

Commands:
  info <file.grf>                    Show archive information
  list <file.grf> [pattern]          List files (optional glob pattern)
  extract <file.grf> <path> [output] Extract file(s) to directory
  search <file.grf> <pattern>        Search files by name pattern

Examples:
  grftool info data.grf
  grftool list data.grf "*.spr"
  grftool extract data.grf data/sprite/npc/npc.spr ./output
  grftool search data.grf "prontera"`)
}

func cmdInfo(args []string) {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "Usage: grftool info <file.grf>")
		os.Exit(1)
	}

	archive, err := grf.Open(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	files := archive.List()

	// Count by extension
	extCount := make(map[string]int)
	var totalSize uint64
	for _, f := range files {
		ext := strings.ToLower(filepath.Ext(f))
		if ext == "" {
			ext = "(no ext)"
		}
		extCount[ext]++
	}

	fmt.Printf("Archive: %s\n", args[0])
	fmt.Printf("Files:   %d\n", len(files))
	fmt.Printf("Size:    %.2f MB\n", float64(totalSize)/(1024*1024))
	fmt.Println()
	fmt.Println("Files by type:")

	// Sort by count
	type extStat struct {
		ext   string
		count int
	}
	var stats []extStat
	for ext, count := range extCount {
		stats = append(stats, extStat{ext, count})
	}
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].count > stats[j].count
	})

	for _, s := range stats {
		if s.count >= 10 {
			fmt.Printf("  %-10s %d\n", s.ext, s.count)
		}
	}
}

func cmdList(args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	limit := fs.Int("n", 0, "Limit output to N files (0 = all)")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "Usage: grftool list <file.grf> [pattern]")
		os.Exit(1)
	}

	archive, err := grf.Open(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	files := archive.List()
	sort.Strings(files)

	pattern := ""
	if fs.NArg() > 1 {
		pattern = strings.ToLower(fs.Arg(1))
	}

	count := 0
	for _, f := range files {
		if pattern != "" {
			matched, _ := filepath.Match(pattern, strings.ToLower(filepath.Base(f)))
			if !matched && !strings.Contains(strings.ToLower(f), pattern) {
				continue
			}
		}
		fmt.Println(f)
		count++
		if *limit > 0 && count >= *limit {
			break
		}
	}

	if pattern != "" {
		fmt.Fprintf(os.Stderr, "\n(%d files matched)\n", count)
	}
}

func cmdExtract(args []string) {
	fs := flag.NewFlagSet("extract", flag.ExitOnError)
	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Usage: grftool extract <file.grf> <path> [output_dir]")
		os.Exit(1)
	}

	grfPath := fs.Arg(0)
	filePath := fs.Arg(1)
	outputDir := "."
	if fs.NArg() > 2 {
		outputDir = fs.Arg(2)
	}

	archive, err := grf.Open(grfPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	// Check if it's a pattern
	if strings.Contains(filePath, "*") {
		extractPattern(archive, filePath, outputDir)
		return
	}

	// Single file extraction
	if !archive.Contains(filePath) {
		fmt.Fprintf(os.Stderr, "File not found: %s\n", filePath)
		os.Exit(1)
	}

	data, err := archive.Read(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading file: %v\n", err)
		os.Exit(1)
	}

	// Create output path
	outputPath := filepath.Join(outputDir, filepath.Base(filePath))
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(outputPath, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Extracted: %s (%d bytes)\n", outputPath, len(data))
}

func extractPattern(archive *grf.Archive, pattern, outputDir string) {
	files := archive.List()
	pattern = strings.ToLower(pattern)

	extracted := 0
	for _, f := range files {
		matched, _ := filepath.Match(pattern, strings.ToLower(filepath.Base(f)))
		if !matched {
			continue
		}

		data, err := archive.Read(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", f, err)
			continue
		}

		// Preserve directory structure
		outputPath := filepath.Join(outputDir, f)
		if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
			fmt.Fprintf(os.Stderr, "Error creating directory: %v\n", err)
			continue
		}

		if err := os.WriteFile(outputPath, data, 0644); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", outputPath, err)
			continue
		}

		fmt.Printf("Extracted: %s\n", outputPath)
		extracted++
	}

	fmt.Fprintf(os.Stderr, "\nExtracted %d files\n", extracted)
}

func cmdSearch(args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	limit := fs.Int("n", 50, "Limit results (0 = all)")
	fs.Parse(args)

	if fs.NArg() < 2 {
		fmt.Fprintln(os.Stderr, "Usage: grftool search <file.grf> <pattern>")
		os.Exit(1)
	}

	archive, err := grf.Open(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	defer archive.Close()

	files := archive.List()
	pattern := strings.ToLower(fs.Arg(1))

	count := 0
	for _, f := range files {
		if strings.Contains(strings.ToLower(f), pattern) {
			fmt.Println(f)
			count++
			if *limit > 0 && count >= *limit {
				fmt.Fprintf(os.Stderr, "\n(showing first %d matches, use -n 0 for all)\n", *limit)
				break
			}
		}
	}

	if count == 0 {
		fmt.Fprintln(os.Stderr, "No files found")
	} else if *limit == 0 || count < *limit {
		fmt.Fprintf(os.Stderr, "\n(%d files found)\n", count)
	}
}
