package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

var headless = flag.Bool("headless", true, "run browser in headless mode")

const jsAllLinks = `
(function(){
  const links = document.links;
  const urls = new Array(links.length);
  for (let i = 0; i < links.length; i++) {
    urls[i] = new URL(links[i], document.baseURI).href;
  }
  return urls;
})()
`

func main() {
	flag.Parse()

	if flag.NArg() < 1 {
		errExit(`usage:
  site-inspector URL [URL...]
  site-inspector BASE_URL STATIC_SITE_DIR`, 2)
	}

	urls := flag.Args()
	if len(urls) == 2 {
		baseURL, publicDir := flag.Arg(0), flag.Arg(1)
		info, err := os.Stat(publicDir)
		if err == nil && info.IsDir() {
			var err error
			urls, err = generateURLs(baseURL, publicDir)
			if err != nil {
				errExit(err.Error(), 1)
			}
		}
	}

	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", *headless),
	)

	// Start a new browser without a timeout. Subsequent calls to
	// chromedp.NewContext (if any) create new tabs.
	ctx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()
	ctx, cancel = chromedp.NewContext(ctx)
	defer cancel()
	if err := chromedp.Run(ctx); err != nil {
		errExit(err.Error(), 1)
	}

	var all []string
	start := time.Now()
	for i, url := range urls {
		if len(urls) > 100 {
			log.Printf("Loading links from page %d of %d (%v since start)...", i+1, len(urls), time.Since(start).Truncate(time.Millisecond))
		}
		links, err := linksFrom(ctx, url)
		if err != nil {
			// Stop on error
			errExit(err.Error(), 1)
			// Or print a message and continue
			// fmt.Fprintln(os.Stderr, "**************", err.Error(), "**************")
			// continue
		}
		all = append(all, links...)
	}
	all = sortDedup(all)
	log.Println("Found", len(all), "unique links")
	for _, link := range all {
		fmt.Println(link)
	}
}

// errExit prints message to stderr and exits with the given code.
func errExit(message string, code int) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(code)
}

// linksFrom opens the url and extracts all links.
func linksFrom(ctx context.Context, url string) (links []string, err error) {
	resp, err := chromedp.RunResponse(ctx,
		chromedp.Navigate(url),
		chromedp.Evaluate(jsAllLinks, &links),
	)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", url, err)
	}
	fmt.Fprintf(os.Stderr, "GET %s\n%d %s\n", url, resp.Status, resp.StatusText)

	return links, nil
}

// sortDedup sorts and removes duplicates from s mutating the original data
// in-place.
func sortDedup(s []string) []string {
	sort.Strings(s)
	j := 0
	for i := 1; i < len(s); i++ {
		if s[j] == s[i] {
			continue
		}
		j++
		s[j] = s[i]
	}
	return s[:j+1]
}

// generateURLs inspects the structure of publicDir to find index.html files and
// construct URLs relative to baseURL. This is useful for generating a complete
// list of "pretty" URLs of a static website.
func generateURLs(baseURL, publicDir string) (urls []string, err error) {
	const indexhtml = "index.html"
	baseURL = strings.TrimRight(baseURL, "/")
	err = filepath.Walk(publicDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == indexhtml {
			urls = append(urls, baseURL+filepath.ToSlash(strings.TrimSuffix(strings.TrimPrefix(path, publicDir), indexhtml)))
		}
		return nil
	})
	return urls, err
}
