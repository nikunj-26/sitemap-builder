# Sitemap Builder

A fast, lightweight CLI tool to automatically generate XML sitemaps for your website. Built with Go, featuring a vibrant terminal UI, concurrent crawling, and resilient link handling.

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8?style=flat-square&logo=go)
![License](https://img.shields.io/badge/License-MIT-green?style=flat-square)

## Features

✨ **Vibrant Terminal UI**

- Colorful, real-time feedback during crawling
- Visual indicators for each action (visiting, skipping, errors)
- Summary statistics with crawl duration

🕷️ **Smart Web Crawling**

- Automatically discovers all internal links on a domain
- Tracks visited URLs to avoid duplicates
- Handles malformed and external links gracefully

🎯 **Robust Link Handling**

- Normalizes relative and absolute URLs
- Skips non-HTML content (PDFs, images, etc.)
- Sanitizes malformed href attributes
- Silently ignores invalid external URLs

📄 **Standard Sitemap Format**

- Generates valid XML sitemaps compatible with search engines
- Proper sitemap protocol formatting
- Ready to submit to Google Search Console

⚡ **Fast & Efficient**

- Concurrent worker-based crawler
- Minimal memory footprint
- Timing statistics for performance analysis

## Installation

### From Source

```bash
git clone https://github.com/yourusername/sitemap-builder.git
cd sitemap-builder
go build
```

### Requirements

- Go 1.25 or higher

## Usage

### Basic Usage

Generate a sitemap for a website:

```bash
./sitemap-builder -url=https://example.com
```

Or run directly without building:

```bash
go run . -url=https://example.com
```

This creates a `sitemap.xml` file in the current directory.

### Custom Output File

Specify a custom filename for the sitemap:

```bash
./sitemap-builder -url=https://example.com -output=my-sitemap.xml
```

### Custom Worker Count

Control concurrency with worker count:

```bash
./sitemap-builder -url=https://example.com -workers=12
```

### Command-Line Flags

| Flag       | Description                                 | Default            |
| ---------- | ------------------------------------------- | ------------------ |
| `-url`     | The URL to build the sitemap for (required) | —                  |
| `-output`  | Output filename for the sitemap             | `sitemap.xml`      |
| `-workers` | Number of concurrent workers                | `runtime.NumCPU()` |

## Example Output

```
8 workers will be used for crawling.

🕷️  Sitemap Builder
Crawling: https://example.com

🔗 Visiting: https://example.com
✓ Found 12 links on: https://example.com
🔗 Visiting: https://example.com/about
✓ Found 5 links on: https://example.com/about
⏭️  Skipping non-HTML: https://example.com/document.pdf
...

✅ Sitemap generated successfully!

  📊 Total Pages: 145
  ⏱️  Time Taken: 12.543s
  📁 Output File: sitemap.xml
```

## How It Works

1. **Parse Input URL** - Validates and normalizes the base URL
2. **Concurrent Crawling** - Uses a worker-limited crawler to discover links in parallel
3. **Link Extraction** - Parses HTML to find all anchor tags and links
4. **URL Validation** - Checks if links are internal (same domain) and valid
5. **Deduplication** - Skips already-scheduled URLs to prevent duplicate work
6. **Content Filtering** - Ignores non-HTML responses (PDFs, images, etc.)
7. **Sitemap Generation** - Creates and sorts a properly formatted XML sitemap
8. **Summary Report** - Displays crawl statistics and completion status

## Dependencies

- `golang.org/x/net/html` - HTML parsing
- `github.com/charmbracelet/lipgloss` - Terminal styling

All dependencies are managed via `go.mod` and will be automatically installed.

## Limitations

- Only crawls the specified domain (same-host links only)
- Does not execute JavaScript (static HTML only)
- Does not handle protected or authenticated pages
- Does not respect robots.txt (use responsibly)

## FAQ

**Q: How long does it take?**
Depends on site size and server speed. Typical sites (100-1000 pages) take 10-60 seconds.

**Q: Can it crawl subdomains?**
Only links with the same host are crawled. Cross-domain links are skipped.

**Q: Does it handle JavaScript?**
No, only static HTML is parsed. AJAX and dynamically rendered content won't be discovered.

**Q: Is it safe to crawl production sites?**
Yes, it makes normal HTTP requests. Be respectful of server resources.

## License

MIT License - see LICENSE file for details

---

Built with inspiration from [Gophercises](https://gophercises.com/) • Styled with [Charmbracelet](https://github.com/charmbracelet) 🎨
