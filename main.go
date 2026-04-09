package main

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/net/html"
)

type Link struct {
	Href string
}

type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	Xmlns   string   `xml:"xmlns,attr"`
	URLs    []URL    `xml:"url"`
}

type URL struct {
	Loc string `xml:"loc"`
}

var ErrNonHTML = errors.New("non-html content")

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     30 * time.Second,
	},
}

// Styling
var (
	style_header   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00D7FF"))
	style_visiting = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFF00")) // Yellow
	style_success  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")) // Green
	style_skip     = lipgloss.NewStyle().Foreground(lipgloss.Color("#808080")) // Gray
	style_error    = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red
	style_summary  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00FF00"))
	style_stats    = lipgloss.NewStyle().Foreground(lipgloss.Color("#87CEEB")) // Sky blue
)

func main() {
	// Get inputs
	baseURL := flag.String("url", "", "The URL to build the sitemap for")
	output := flag.String("output", "sitemap.xml", "The name of the output sitemap file")
	workers := flag.Int("workers", runtime.NumCPU(), "The number of concurrent workers to use")
	flag.Parse()

	fmt.Println()
	fmt.Printf("%d workers will be used for crawling.", *workers)

	if *baseURL == "" {
		fmt.Println(style_error.Render("❌ Please provide a URL with -url flag"))
		os.Exit(1)
	}

	normalizedStartURL, err := normalizeStartURL(*baseURL)
	if err != nil {
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Invalid URL: %v", err)))
		os.Exit(1)
	}

	// Print header
	fmt.Println()
	fmt.Println(style_header.Render("🕷️  Sitemap Builder"))
	fmt.Println(style_stats.Render("Crawling: " + normalizedStartURL))
	fmt.Println()

	startTime := time.Now()

	c, err := buildSitemap(normalizedStartURL, *workers)
	if err != nil {
		fmt.Println()
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Error: %v", err)))
		os.Exit(1)
	}

	err = c.writeXML(*output)
	if err != nil {
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Error writing XML: %v", err)))
		os.Exit(1)
	}

	duration := time.Since(startTime)
	printSummary(duration, *output, c.visited)
}

func printSummary(duration time.Duration, filename string, visited map[string]bool) {
	fmt.Println()
	fmt.Println(style_summary.Render("✅ Sitemap generated successfully!"))
	fmt.Println()
	fmt.Println(style_stats.Render(fmt.Sprintf("  📊 Total Pages: %d", len(visited))))
	fmt.Println(style_stats.Render(fmt.Sprintf("  ⏱️  Time Taken: %v", duration.Round(time.Millisecond))))
	fmt.Println(style_stats.Render(fmt.Sprintf("  📁 Output File: %s", filename)))
	fmt.Println()
}

type crawler struct {
	ctx       context.Context
	cancel    context.CancelFunc
	sem       chan struct{}
	wg        sync.WaitGroup
	errOnce   sync.Once
	firstErr  error
	mu        sync.Mutex
	scheduled map[string]bool
	visited   map[string]bool
	baseURL   url.URL
}

func newCrawler(workers int, baseURL url.URL) *crawler {
	ctx, cancel := context.WithCancel(context.Background())
	return &crawler{
		ctx:       ctx,
		cancel:    cancel,
		sem:       make(chan struct{}, workers),
		scheduled: make(map[string]bool),
		visited:   make(map[string]bool),
		baseURL:   baseURL,
	}
}

func (c *crawler) setErr(err error) {
	c.errOnce.Do(func() {
		c.firstErr = err
		c.cancel()
	})
}

func (c *crawler) isDone() bool {
	select {
	case <-c.ctx.Done():
		return true
	default:
		return false
	}
}

func (c *crawler) enqueue(rawURL string) {
	c.mu.Lock()
	if c.scheduled[rawURL] {
		c.mu.Unlock()
		return
	}
	c.scheduled[rawURL] = true
	c.mu.Unlock()

	c.wg.Add(1)
	go c.process(rawURL)
}

func (c *crawler) process(target string) {
	defer c.wg.Done()

	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-c.ctx.Done():
		return
	}

	fmt.Println(style_visiting.Render("🔗 Visiting: " + target))

	r, err := getHTML(target)
	if err != nil {
		if errors.Is(err, ErrNonHTML) {
			fmt.Println(style_skip.Render("⏭️  Skipping non-HTML: " + target))
			return
		}
		c.setErr(fmt.Errorf("Error getting HTML for %s: %v", target, err))
		return
	}

	links, err := c.extractLinks(r)
	if err != nil {
		c.setErr(fmt.Errorf("Error extracting links for %s: %v", target, err))
		return
	}

	c.mu.Lock()
	c.visited[target] = true
	c.mu.Unlock()

	fmt.Println(style_success.Render(fmt.Sprintf("✓ Found %d links on: %s", len(links), target)))

	if c.isDone() {
		return
	}

	for _, link := range links {
		if c.isDone() {
			return
		}
		c.enqueue(link.Href)
	}
}

func buildSitemap(startURL string, workers int) (*crawler, error) {
	if workers < 1 {
		workers = 1
	}

	parsedURL, err := url.Parse(startURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing URL: %v", err)
	}

	c := newCrawler(workers, *parsedURL)
	defer c.cancel()

	c.enqueue(startURL)
	c.wg.Wait()

	return c, c.firstErr
}

func normalizeStartURL(rawURL string) (string, error) {
	parsedURL, err := url.ParseRequestURI(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("error parsing URL: %v", err)
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return "", fmt.Errorf("unsupported URL scheme: %s", parsedURL.Scheme)
	}

	if parsedURL.Hostname() == "" {
		return "", fmt.Errorf("missing URL host")
	}

	parsedURL.Path = "/"
	parsedURL.RawPath = ""
	parsedURL.RawQuery = ""
	parsedURL.Fragment = ""

	if parsedURL.Scheme == "http" {
		res, err := httpClient.Get(parsedURL.String())
		if err == nil {
			res.Body.Close()
			if res.Request != nil && res.Request.URL != nil {
				finalURL := res.Request.URL
				if finalURL.Scheme == "https" && finalURL.Hostname() == parsedURL.Hostname() {
					parsedURL.Scheme = "https"
					parsedURL.Host = finalURL.Host
				}
			}
		}
	}

	return parsedURL.String(), nil
}

func getHTML(url string) (io.Reader, error) {
	res, err := httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode > 299 {
		return nil, fmt.Errorf("Response failed with status code: %d", res.StatusCode)
	}

	contentType := strings.ToLower(res.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, ErrNonHTML
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(body), nil
}

func (c *crawler) extractLinks(r io.Reader) ([]Link, error) {
	var links []Link

	doc, err := html.Parse(r)
	if err != nil {
		return nil, fmt.Errorf("Error parsing html %v", err)
	}

	for n := range doc.Descendants() {
		if n.Type == html.ElementNode && n.Data == "a" {
			link := Link{}

			for _, a := range n.Attr {
				if a.Key == "href" {
					href := sanitizeHref(a.Val)
					if href == "" {
						continue
					}

					isInternal, err := c.isInternalLink(href)
					if err != nil {
						continue
					}
					if isInternal {
						// fmt.Println("Links", a.Val)
						link.Href, err = c.normalizeURL(href)
						if err != nil {
							continue
						}
						links = append(links, link)
					}
				}
			}
		}
	}

	return links, nil
}

func sanitizeHref(link string) string {
	link = strings.TrimSpace(link)
	for len(link) > 1 && strings.HasPrefix(link, "(") && strings.HasSuffix(link, ")") {
		link = strings.TrimPrefix(link, "(")
		link = strings.TrimSuffix(link, ")")
		link = strings.TrimSpace(link)
	}

	return link
}

func (c *crawler) isInternalLink(link string) (bool, error) {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false, fmt.Errorf("Error parsing URL %s: %v", link, err)
	}

	if parsedLink.Scheme != "" && parsedLink.Scheme != "http" && parsedLink.Scheme != "https" {
		return false, nil
	}

	if parsedLink.Host == "" {
		return true, nil
	}

	return parsedLink.Hostname() == c.baseURL.Hostname(), nil
}

func (c *crawler) normalizeURL(link string) (string, error) {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL %s: %v", link, err)
	}

	if parsedLink.IsAbs() == false {
		return c.baseURL.ResolveReference(parsedLink).String(), nil
	}

	return parsedLink.String(), nil
}

func (c *crawler) writeXML(filename string) error {
	urlSet := URLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	for link := range c.visited {
		urlSet.URLs = append(urlSet.URLs, URL{Loc: link})
	}

	sort.Slice(urlSet.URLs, func(i, j int) bool {
		return urlSet.URLs[i].Loc < urlSet.URLs[j].Loc
	})

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("error creating sitemap file: %v", err)
	}
	defer file.Close()

	file.Write([]byte(xml.Header))

	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	err = encoder.Encode(urlSet)
	if err != nil {
		return fmt.Errorf("error encoding sitemap: %v", err)
	}

	return nil
}
