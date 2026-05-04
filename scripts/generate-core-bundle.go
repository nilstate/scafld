package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	check := flag.Bool("check", false, "verify the source bundle exists")
	source := flag.String("source", ".scafld/core", "core bundle source")
	flag.Parse()
	if !*check {
		fmt.Println("bundle generation is explicit; pass --check to verify")
		return
	}
	info, err := os.Stat(*source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "source bundle missing: %v\n", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		fmt.Fprintf(os.Stderr, "source bundle is not a directory: %s\n", filepath.Clean(*source))
		os.Exit(1)
	}
	fmt.Printf("bundle source ok: %s\n", filepath.Clean(*source))
}
