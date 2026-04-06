package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
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

var visited = make(map[string]bool)
var crawlOrder []string // Track order of discovered URLs

var BaseURL url.URL

var ErrNonHTML = errors.New("non-html content")

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
	flag.Parse()

	if *baseURL == "" {
		fmt.Println(style_error.Render("❌ Please provide a URL with -url flag"))
		os.Exit(1)
	}

	// Setting as global variable for later use
	parsedURL, err := url.Parse(*baseURL)
	if err != nil {
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Error parsing URL: %v", err)))
		os.Exit(1)
	}
	BaseURL = *parsedURL

	// Print header
	fmt.Println()
	fmt.Println(style_header.Render("🕷️  Sitemap Builder"))
	fmt.Println(style_stats.Render("Crawling: " + *baseURL))
	fmt.Println()

	startTime := time.Now()

	err = buildSitemap(*baseURL)
	if err != nil {
		fmt.Println()
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Error: %v", err)))
		os.Exit(1)
	}

	err = writeXML(*output)
	if err != nil {
		fmt.Println(style_error.Render(fmt.Sprintf("❌ Error writing XML: %v", err)))
		os.Exit(1)
	}

	duration := time.Since(startTime)
	printSummary(duration, *output)
}

func printSummary(duration time.Duration, filename string) {
	fmt.Println()
	fmt.Println(style_summary.Render("✅ Sitemap generated successfully!"))
	fmt.Println()
	fmt.Println(style_stats.Render(fmt.Sprintf("  📊 Total Pages: %d", len(visited))))
	fmt.Println(style_stats.Render(fmt.Sprintf("  ⏱️  Time Taken: %v", duration.Round(time.Millisecond))))
	fmt.Println(style_stats.Render(fmt.Sprintf("  📁 Output File: %s", filename)))
	fmt.Println()
}

func buildSitemap(url string) error {
	if visited[url] {
		return nil
	}

	fmt.Println(style_visiting.Render("🔗 Visiting: " + url))

	r, err := getHTML(url)
	if err != nil {
		if errors.Is(err, ErrNonHTML) {
			fmt.Println(style_skip.Render("⏭️  Skipping non-HTML: " + url))
			return nil
		}
		return fmt.Errorf("Error getting HTML for %s: %v", url, err)
	}

	// Mark as visited only after confirming this URL is HTML content.
	visited[url] = true
	crawlOrder = append(crawlOrder, url)

	links, err := extractLinks(r)
	if err != nil {
		return fmt.Errorf("Error extracting links for %s: %v", url, err)
	}
	fmt.Println(style_success.Render(fmt.Sprintf("✓ Found %d links on: %s", len(links), url)))

	for _, link := range links {
		err = buildSitemap(link.Href)
		if err != nil {
			return err
		}
	}

	return nil
}

func getHTML(url string) (io.Reader, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	contentType := strings.ToLower(res.Header.Get("Content-Type"))
	if !strings.Contains(contentType, "text/html") && !strings.Contains(contentType, "application/xhtml+xml") {
		return nil, ErrNonHTML
	}

	body, err := io.ReadAll(res.Body)
	if res.StatusCode > 299 {
		return nil, fmt.Errorf("Response failed with status code: %d", res.StatusCode)
	}
	if err != nil {
		return nil, err
	}
	// fmt.Printf("%s", body)
	return bytes.NewReader(body), nil
}

func extractLinks(r io.Reader) ([]Link, error) {
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

					isInternal, err := isInternalLink(href)
					if err != nil {
						continue
					}
					if isInternal {
						// fmt.Println("Links", a.Val)
						link.Href, err = normalizeURL(href)
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

func isInternalLink(link string) (bool, error) {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return false, fmt.Errorf("Error parsing URL %s: %v", link, err)
	}

	if parsedLink.IsAbs() == false {
		return true, nil
	}

	return parsedLink.Host == BaseURL.Host, nil
}

func normalizeURL(link string) (string, error) {
	parsedLink, err := url.Parse(link)
	if err != nil {
		return "", fmt.Errorf("Error parsing URL %s: %v", link, err)
	}

	if parsedLink.IsAbs() == false {
		return BaseURL.ResolveReference(parsedLink).String(), nil
	}

	return parsedLink.String(), nil
}

func writeXML(filename string) error {
	urlSet := URLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
	}

	for link := range visited {
		urlSet.URLs = append(urlSet.URLs, URL{Loc: link})
	}

	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("Error creating sitemap file: %v", err)
	}
	defer file.Close()

	file.Write([]byte(xml.Header))

	encoder := xml.NewEncoder(file)
	encoder.Indent("", "  ")
	err = encoder.Encode(urlSet)
	if err != nil {
		return fmt.Errorf("Error encoding sitemap: %v", err)
	}

	return nil
}
