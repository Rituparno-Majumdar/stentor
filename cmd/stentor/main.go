// Command stentor crossposts one piece of content to many social
// platforms at once. No database, no daemon: it runs, posts, exits.
package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
	"github.com/Rituparno-Majumdar/stentor/internal/platforms"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}
	ctx := context.Background()

	switch os.Args[1] {
	case "auth":
		cmdAuth(ctx, os.Args[2:])
	case "status":
		cmdStatus()
	case "platforms":
		cmdPlatforms()
	case "post":
		cmdPost(ctx, os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Println(`stentor -- one post, everywhere, no database, no daemon.

Usage:
  stentor auth <platform>        interactive credential setup for one platform
  stentor status                 which platforms have stored credentials
  stentor platforms               list every supported platform + its friction tier
  stentor post --platform <names> --text "..." [--image path] [--link url]
                                  names is a comma-separated list, or "all"`)
}

func cmdAuth(ctx context.Context, args []string) {
	if len(args) != 1 {
		fmt.Fprintln(os.Stderr, "usage: stentor auth <platform>")
		os.Exit(1)
	}
	p := platforms.ByName(args[0])
	if p == nil {
		fmt.Fprintf(os.Stderr, "unknown platform %q -- run: stentor platforms\n", args[0])
		os.Exit(1)
	}
	cfg, err := config.Load()
	fatalIf(err)

	in := bufio.NewReader(os.Stdin)
	if err := p.Authenticate(ctx, cfg, in); err != nil {
		fmt.Fprintf(os.Stderr, "auth failed: %v\n", err)
		os.Exit(1)
	}
	fatalIf(cfg.Save())
	fmt.Printf("%s: credentials saved\n", p.Name())
}

func cmdStatus() {
	cfg, err := config.Load()
	fatalIf(err)
	for _, p := range platforms.All() {
		state := "not authenticated"
		if p.IsAuthenticated(cfg) {
			state = "authenticated"
		}
		fmt.Printf("%-10s %s\n", p.Name(), state)
	}
}

func cmdPlatforms() {
	for _, p := range platforms.All() {
		fmt.Printf("%-10s tier=%-24s %s\n", p.Name(), p.Tier(), p.FrictionNote())
	}
}

func cmdPost(ctx context.Context, args []string) {
	fs := flag.NewFlagSet("post", flag.ExitOnError)
	platformFlag := fs.String("platform", "", "comma-separated platform names, or \"all\"")
	text := fs.String("text", "", "post text")
	image := fs.String("image", "", "optional local image path")
	link := fs.String("link", "", "optional URL to attach")
	fatalIf(fs.Parse(args))

	if *platformFlag == "" || *text == "" {
		fmt.Fprintln(os.Stderr, "usage: stentor post --platform <names|all> --text \"...\" [--image path] [--link url]")
		os.Exit(1)
	}

	cfg, err := config.Load()
	fatalIf(err)

	var targets []platforms.Platform
	if *platformFlag == "all" {
		for _, p := range platforms.All() {
			if p.IsAuthenticated(cfg) {
				targets = append(targets, p)
			}
		}
		if len(targets) == 0 {
			fmt.Fprintln(os.Stderr, "no platform has stored credentials yet -- run: stentor auth <platform>")
			os.Exit(1)
		}
	} else {
		for _, name := range strings.Split(*platformFlag, ",") {
			name = strings.TrimSpace(name)
			p := platforms.ByName(name)
			if p == nil {
				fmt.Fprintf(os.Stderr, "unknown platform %q -- run: stentor platforms\n", name)
				os.Exit(1)
			}
			targets = append(targets, p)
		}
	}

	c := content.Content{Text: *text, Image: *image, Link: *link}
	exitCode := 0
	for _, p := range targets {
		if !p.IsAuthenticated(cfg) {
			fmt.Printf("%-10s SKIPPED -- not authenticated, run: stentor auth %s\n", p.Name(), p.Name())
			exitCode = 1
			continue
		}
		res, err := p.Post(ctx, cfg, c)
		if err != nil {
			fmt.Printf("%-10s FAILED  -- %v\n", p.Name(), err)
			exitCode = 1
			continue
		}
		ref := res.URL
		if ref == "" {
			ref = res.ID
		}
		fmt.Printf("%-10s OK      -- %s\n", p.Name(), ref)
	}
	os.Exit(exitCode)
}

func fatalIf(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
