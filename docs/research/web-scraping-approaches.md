# Web Scraping/Crawling Approaches for Raven Knowledge Base

**Date:** 2026-03-27
**Status:** Draft / Research
**Author:** Research Phase

---

## Context

Raven needs to crawl user-provided website URLs, extract meaningful content, and feed it into a document processing pipeline for vector embedding. Key requirements:

- User provides a root URL (e.g., `https://www.mit.edu`)
- Crawl linked pages with configurable depth
- Extract clean text content from HTML
- Handle JavaScript-rendered content (SPAs)
- Respect `robots.txt`
- Integrate with a Node.js or Python backend

---

## Tool Comparison Matrix

| Feature | Firecrawl | Crawl4AI | Playwright + Custom | Jina Reader API | Scrapy + Splash | Stagehand |
|---|---|---|---|---|---|---|
| Self-hostable | Yes | Yes | Yes | No (SaaS) | Yes | Yes |
| JS Rendering | Yes (Playwright) | Yes (Playwright) | Yes (native) | Yes (server-side) | Yes (Splash) | Yes (Playwright) |
| Output Format | Markdown, HTML, structured | Markdown, HTML, JSON | Raw HTML (you parse) | Markdown | Raw HTML (you parse) | Text, structured |
| robots.txt | Yes | Yes | Manual | Yes | Yes (built-in) | Manual |
| Rate Limiting | Built-in | Built-in | Manual | API rate limits | Built-in | Manual |
| Crawl Depth | Configurable | Configurable | Manual | Single page | Configurable | Single page |
| Language | TypeScript/Python SDK | Python | Node.js / Python | REST API | Python | TypeScript |
| License | AGPL-3.0 | Apache 2.0 | Apache 2.0 | Proprietary SaaS | BSD-3 | MIT |

---

## Approach 1: Firecrawl (Recommended Primary)

**Repository:** https://github.com/mendableai/firecrawl

### Overview

Firecrawl is purpose-built for LLM-oriented web scraping. It crawls websites, handles JS rendering via Playwright, and outputs clean markdown -- exactly what a vector embedding pipeline needs. It was designed specifically for the RAG (Retrieval-Augmented Generation) use case.

### Key Features

- **Crawl mode:** Provide a root URL, Firecrawl discovers and crawls all linked pages. Configurable `maxDepth`, `limit` (max pages), and URL inclusion/exclusion patterns.
- **Scrape mode:** Single-page extraction with clean markdown output.
- **Map mode:** Discovers all URLs on a site without scraping content (useful for sitemap generation).
- **JS Rendering:** Uses Playwright under the hood. Can wait for dynamic content, execute custom JS actions (click, scroll, type).
- **Output formats:** Markdown (default), HTML, raw text, screenshots, structured data (via LLM extraction).
- **robots.txt:** Respected by default.
- **Anti-bot handling:** Proxy support, stealth mode, configurable headers/cookies.
- **Webhooks:** Async crawl with webhook notifications on completion.

### Architecture

```
User Request (root URL + config)
        |
        v
  Firecrawl API Server (self-hosted)
        |
        v
  Playwright Browser Pool
        |
        v
  Content Extraction (Readability + custom)
        |
        v
  Markdown/Structured Output --> Raven Pipeline
```

### Self-Hosting

Firecrawl can be self-hosted via Docker:
```bash
# Clone and configure
git clone https://github.com/mendableai/firecrawl.git
cd firecrawl

# Docker compose (includes Redis, Playwright workers)
docker compose up -d
```

The self-hosted version runs:
- API server (Node.js/Express)
- Worker processes (Playwright-based scrapers)
- Redis (job queue)
- Optional: Bull dashboard for monitoring

### Integration with Raven

```python
# Python SDK
from firecrawl import FirecrawlApp

app = FirecrawlApp(api_url="http://localhost:3002")

# Crawl a site
result = app.crawl_url(
    url="https://www.mit.edu",
    params={
        "maxDepth": 3,
        "limit": 100,
        "scrapeOptions": {
            "formats": ["markdown"],
        },
        "includePaths": ["/research/*", "/about/*"],
        "excludePaths": ["/login/*", "/admin/*"],
    },
    poll_interval=5,
)

# Each page in result has: url, markdown, metadata
for page in result.data:
    # Feed into Raven's document pipeline
    process_document(
        content=page.markdown,
        source_url=page.url,
        metadata=page.metadata,  # title, description, language, etc.
    )
```

```typescript
// Node.js SDK
import FirecrawlApp from "@mendable/firecrawl-js";

const app = new FirecrawlApp({ apiUrl: "http://localhost:3002" });

const crawlResult = await app.crawlUrl("https://www.mit.edu", {
  maxDepth: 3,
  limit: 100,
  scrapeOptions: {
    formats: ["markdown"],
  },
});

for (const page of crawlResult.data) {
  await processDocument({
    content: page.markdown,
    sourceUrl: page.url,
    metadata: page.metadata,
  });
}
```

### Pros

- Purpose-built for LLM/RAG use cases -- markdown output is exactly what embedding pipelines need
- Full crawling capability with depth control, URL patterns, and page limits
- Both Python and Node.js SDKs
- Active development and community (18k+ GitHub stars)
- Handles JS rendering, anti-bot measures, and complex sites out of the box
- Async crawl with webhook support for large sites
- Structured data extraction via LLM (can extract specific schemas)
- Built-in rate limiting and concurrency control

### Cons

- **AGPL-3.0 license** -- copyleft; if Raven modifies Firecrawl itself, those changes must be open-sourced. Using it as a service (API calls) is generally fine, but legal review recommended.
- Resource-heavy when self-hosted (Playwright browsers consume significant RAM)
- The self-hosted version may lag behind the cloud version in features
- Redis dependency adds operational complexity
- Some advanced features (e.g., LLM extraction) may require API keys for external services

### Cost Considerations

- **Self-hosted:** Free (infrastructure costs only). Expect ~2-4GB RAM for moderate workloads.
- **Cloud API:** Free tier (500 credits/month), paid plans starting at $16/month for 3,000 credits.

---

## Approach 2: Crawl4AI (Recommended Alternative)

**Repository:** https://github.com/unclecode/crawl4ai

### Overview

Crawl4AI is an open-source Python library specifically designed for crawling websites and extracting content optimized for AI/LLM consumption. It is lightweight, fast, and produces high-quality markdown output with built-in chunking strategies.

### Key Features

- **Async-first:** Built on `asyncio` with `aiohttp` and async Playwright for high throughput.
- **Smart extraction:** Multiple extraction strategies -- LLM-based, CSS-selector-based, cosine similarity-based, and JSON-CSS hybrid.
- **Markdown output:** Clean markdown with configurable content filters (remove navbars, footers, ads).
- **Chunking:** Built-in chunking strategies (by topic, regex, sentence, fixed-length) -- very useful for embedding pipelines.
- **Link analysis:** Categorizes internal vs external links, tracks link context.
- **Media extraction:** Extracts images, videos, and audio with metadata.
- **Session management:** Maintains browser sessions across multiple crawls (useful for authenticated content).
- **Caching:** Built-in caching to avoid re-crawling unchanged pages.

### Architecture

```
User Request (root URL + config)
        |
        v
  Crawl4AI AsyncWebCrawler
        |
        v
  Playwright (async) or HTTP client
        |
        v
  Content Extraction Strategy
  (LLM / CSS / Cosine / JsonCSS)
        |
        v
  Markdown + Chunks --> Raven Pipeline
```

### Self-Hosting

Crawl4AI runs as a Python library or Docker-based API server:
```bash
# Install as library
pip install crawl4ai
crawl4ai-setup  # Downloads Playwright browsers

# Or run as Docker API server
docker pull unclecode/crawl4ai
docker run -p 11235:11235 unclecode/crawl4ai
```

### Integration with Raven

```python
from crawl4ai import AsyncWebCrawler, CrawlerRunConfig, BrowserConfig
from crawl4ai.deep_crawling import BFSDeepCrawlStrategy
from crawl4ai.content_filter import PruningContentFilter

async def crawl_for_raven(root_url: str, max_depth: int = 3, max_pages: int = 100):
    browser_config = BrowserConfig(
        headless=True,
        java_script_enabled=True,
    )

    # Deep crawl strategy with BFS
    deep_crawl = BFSDeepCrawlStrategy(
        max_depth=max_depth,
        max_pages=max_pages,
        include_external=False,
    )

    # Content filter to remove boilerplate
    content_filter = PruningContentFilter(
        threshold=0.5,
        threshold_type="fixed",
    )

    crawl_config = CrawlerRunConfig(
        deep_crawl_strategy=deep_crawl,
        content_filter=content_filter,
        markdown_generator=DefaultMarkdownGenerator(
            content_filter=content_filter,
        ),
    )

    async with AsyncWebCrawler(config=browser_config) as crawler:
        results = await crawler.arun(url=root_url, config=crawl_config)

        # results is a list of CrawlResult objects
        for result in results:
            if result.success:
                await process_document(
                    content=result.markdown.fit_markdown,  # Filtered markdown
                    source_url=result.url,
                    metadata={
                        "title": result.metadata.get("title"),
                        "description": result.metadata.get("description"),
                        "links": result.links,
                    },
                )
```

For Node.js backends, Crawl4AI can be used via its Docker API:
```typescript
// Call Crawl4AI Docker API from Node.js
const response = await fetch("http://localhost:11235/crawl", {
  method: "POST",
  headers: { "Content-Type": "application/json" },
  body: JSON.stringify({
    urls: ["https://www.mit.edu"],
    crawler_params: {
      headless: true,
      browser_type: "chromium",
    },
    deep_crawl_strategy: {
      type: "bfs",
      max_depth: 3,
      max_pages: 100,
    },
  }),
});
const result = await response.json();
```

### Pros

- **Apache 2.0 license** -- permissive, no copyleft concerns; ideal for commercial use
- Built-in chunking strategies are a major advantage for embedding pipelines
- Content filtering removes boilerplate (navbars, footers, ads) automatically
- Async-first design enables high throughput
- Memory-efficient compared to Firecrawl (lighter infrastructure footprint)
- Built-in caching reduces redundant crawls
- Very active development (30k+ GitHub stars)
- No Redis dependency -- simpler to operate
- Docker API makes it accessible from Node.js backends

### Cons

- Python-only library (Node.js must go through Docker API)
- Fewer anti-bot features compared to Firecrawl
- Deep crawling is newer and less battle-tested than Firecrawl's crawling
- No built-in webhook support for async crawl notifications
- Documentation can be inconsistent with rapid development pace

### Cost Considerations

- **Self-hosted:** Free. Lighter than Firecrawl (~1-2GB RAM for moderate workloads).
- **Cloud API:** Available but less mature than Firecrawl's cloud offering.

---

## Approach 3: Playwright/Puppeteer + Custom Extraction Pipeline

### Overview

Build a custom crawling and extraction pipeline using Playwright (or Puppeteer) for browser automation, combined with content extraction libraries like Mozilla's Readability, Turndown (HTML-to-Markdown), and a custom crawler for link following and depth management.

### Key Components

| Component | Library Options |
|---|---|
| Browser automation | Playwright (recommended), Puppeteer |
| Content extraction | @mozilla/readability, cheerio, jsdom |
| HTML to Markdown | Turndown, html-to-md |
| Crawl orchestration | Custom BFS/DFS queue, or crawlee |
| robots.txt | robots-parser (npm), robotexclusionrulesparser (pip) |
| Rate limiting | bottleneck (npm), aiohttp rate limiting (pip) |

### Architecture

```
User Request (root URL + depth config)
        |
        v
  Crawl Orchestrator (BFS queue + visited set)
        |
        v
  Playwright Browser Pool (configurable concurrency)
        |
        v
  Page Loaded --> Extract with Readability
        |
        v
  Convert to Markdown (Turndown)
        |
        v
  Clean Markdown --> Raven Pipeline
```

### Implementation Sketch (Node.js with Crawlee)

[Crawlee](https://github.com/apify/crawlee) (by Apify) is a mature Node.js web scraping framework that handles the orchestration layer -- queue management, rate limiting, retry logic, browser pool management, and proxy rotation.

```typescript
import { PlaywrightCrawler, Configuration } from "crawlee";
import { Readability } from "@mozilla/readability";
import { JSDOM } from "jsdom";
import TurndownService from "turndown";

const turndown = new TurndownService({
  headingStyle: "atx",
  codeBlockStyle: "fenced",
});

const crawler = new PlaywrightCrawler({
  maxRequestsPerCrawl: 100,
  maxConcurrency: 5,
  navigationTimeoutSecs: 30,

  async requestHandler({ request, page, enqueueLinks, log }) {
    const depth = request.userData.depth || 0;
    const maxDepth = 3;

    // Wait for content to load (important for SPAs)
    await page.waitForLoadState("networkidle");

    // Extract content using Readability
    const html = await page.content();
    const dom = new JSDOM(html, { url: request.url });
    const article = new Readability(dom.window.document).parse();

    if (article) {
      const markdown = turndown.turndown(article.content);

      // Feed into Raven pipeline
      await processDocument({
        content: markdown,
        sourceUrl: request.url,
        metadata: {
          title: article.title,
          excerpt: article.excerpt,
          siteName: article.siteName,
          depth,
        },
      });
    }

    // Enqueue linked pages if within depth
    if (depth < maxDepth) {
      await enqueueLinks({
        strategy: "same-domain",
        userData: { depth: depth + 1 },
      });
    }
  },
});

await crawler.run(["https://www.mit.edu"]);
```

### Implementation Sketch (Python)

```python
import asyncio
from playwright.async_api import async_playwright
from readability import Document  # readability-lxml
import html2text
from urllib.parse import urljoin, urlparse
from collections import deque

class RavenCrawler:
    def __init__(self, max_depth=3, max_pages=100, concurrency=5):
        self.max_depth = max_depth
        self.max_pages = max_pages
        self.concurrency = concurrency
        self.visited = set()
        self.h2t = html2text.HTML2Text()
        self.h2t.ignore_links = False
        self.h2t.ignore_images = True

    async def crawl(self, root_url: str):
        queue = deque([(root_url, 0)])
        results = []

        async with async_playwright() as p:
            browser = await p.chromium.launch(headless=True)

            while queue and len(results) < self.max_pages:
                url, depth = queue.popleft()
                if url in self.visited:
                    continue
                self.visited.add(url)

                page = await browser.new_page()
                try:
                    await page.goto(url, wait_until="networkidle", timeout=30000)
                    html = await page.content()

                    # Extract readable content
                    doc = Document(html)
                    markdown = self.h2t.handle(doc.summary())

                    results.append({
                        "url": url,
                        "title": doc.title(),
                        "markdown": markdown,
                        "depth": depth,
                    })

                    # Extract links for further crawling
                    if depth < self.max_depth:
                        links = await page.eval_on_selector_all(
                            "a[href]",
                            "els => els.map(e => e.href)"
                        )
                        base_domain = urlparse(root_url).netloc
                        for link in links:
                            parsed = urlparse(link)
                            if parsed.netloc == base_domain and link not in self.visited:
                                queue.append((link, depth + 1))
                finally:
                    await page.close()

            await browser.close()
        return results
```

### Pros

- **Full control** over every aspect of crawling and extraction
- No license concerns (MIT/Apache-licensed components)
- Can be optimized specifically for Raven's needs
- No external service dependencies
- Crawlee (if used) provides production-grade crawl orchestration, retries, and monitoring
- Can integrate directly into Raven's existing backend without API boundaries

### Cons

- **Significant development effort** -- 2-4 weeks to build a production-quality crawler
- Must handle edge cases manually: infinite scroll, lazy loading, iframes, redirects, cookie consent banners, CAPTCHAs
- robots.txt handling must be implemented manually
- No built-in content cleaning (Readability helps but isn't perfect for all sites)
- Markdown output quality depends heavily on site structure
- Need to build monitoring, error handling, and retry logic (unless using Crawlee)
- Browser pool management and memory optimization require expertise

### When to Choose This Approach

- When AGPL licensing (Firecrawl) is unacceptable and Crawl4AI's extraction quality is insufficient
- When you need very specific extraction logic for certain site types
- When Raven's backend is Node.js and you want to avoid Python dependencies
- When you need maximum control over browser behavior (custom JS execution, authentication flows)

---

## Other Notable Options Evaluated

### Jina Reader API

**URL:** https://r.jina.ai/
**Type:** SaaS API

- Prefix any URL with `https://r.jina.ai/` to get markdown output
- Excellent markdown quality
- No self-hosting option (SaaS only)
- Rate-limited free tier; paid plans for production use
- **Single page only** -- no crawling capability; would need a separate crawler
- Good as a fallback for individual page extraction but not suitable as a primary solution

**Verdict:** Not recommended as primary approach due to lack of self-hosting and crawling capabilities. Could be used as a supplementary single-page extractor.

### Scrapy + Splash

**Type:** Python framework + JS rendering service

- Scrapy is the most mature Python crawling framework
- Splash provides JS rendering via a headless browser service
- Very scalable (Scrapy's async architecture handles thousands of concurrent requests)
- Splash is less capable than Playwright for modern SPAs
- Steep learning curve (Scrapy's middleware/pipeline architecture)
- No built-in markdown output -- requires custom extraction pipeline
- Better suited for structured data extraction than content-for-LLM extraction

**Verdict:** Overkill for content extraction use case. Best for large-scale structured scraping, not LLM-oriented content extraction.

### Stagehand (by Browserbase)

**Repository:** https://github.com/browserbase/stagehand
**License:** MIT

- AI-powered browser automation -- uses LLM to understand and interact with pages
- Playwright-based with AI-guided actions (click, extract, observe)
- Interesting for complex extraction scenarios
- Single-page focused; no built-in crawling
- Requires LLM API calls for each extraction (adds latency and cost)
- Relatively new, smaller community

**Verdict:** Innovative but not mature enough for production crawling. Worth watching for future complex extraction needs.

### Apify Platform

**URL:** https://apify.com/
**Type:** SaaS + self-hostable actors

- Full-featured web scraping platform
- Crawlee (mentioned above) is Apify's open-source crawling library
- Self-hosted option via Apify SDK
- Extensive actor ecosystem for specific sites
- Can be expensive at scale on cloud

**Verdict:** Crawlee (the library) is the best component from Apify for Raven. The full platform is unnecessary.

---

## Recommended Approach for Raven

### Primary Recommendation: Approach 1 (Firecrawl) with Approach 2 (Crawl4AI) as Fallback

**Rationale:**

1. **Start with Firecrawl** for the fastest time-to-value:
   - Self-host via Docker
   - Use the Python or Node.js SDK
   - Markdown output feeds directly into the embedding pipeline
   - Configurable crawl depth, page limits, and URL patterns
   - Handles JS rendering, robots.txt, and rate limiting out of the box

2. **Evaluate Crawl4AI as alternative** if:
   - AGPL-3.0 license is a concern (Crawl4AI is Apache 2.0)
   - Lower resource footprint is needed
   - Built-in chunking strategies are valuable for the embedding pipeline
   - Python-native integration is preferred

3. **Consider Approach 3 (Custom with Crawlee)** only if:
   - Both Firecrawl and Crawl4AI prove insufficient for specific site types
   - Maximum control over extraction is required
   - The team has bandwidth for 2-4 weeks of custom development

### Suggested Integration Architecture

```
                    Raven Backend (Node.js/Python)
                              |
                    +---------+---------+
                    |                   |
              Crawl Request        Document Processor
                    |                   ^
                    v                   |
            +-------+-------+     Markdown + Metadata
            |               |           |
        Firecrawl      Crawl4AI    (output)
       (Docker API)   (Docker API)     |
            |               |           |
            +-------+-------+           |
                    |                   |
              Playwright Browsers ------+
```

### Implementation Priority

| Phase | Task | Estimated Effort |
|---|---|---|
| Phase 1 | Deploy Firecrawl via Docker, integrate crawl API with Raven backend | 2-3 days |
| Phase 2 | Build crawl job management (queue, status tracking, webhooks) | 3-5 days |
| Phase 3 | Connect markdown output to document chunking + vector embedding pipeline | 2-3 days |
| Phase 4 | Add URL validation, depth config UI, crawl monitoring dashboard | 3-5 days |
| Phase 5 | Evaluate Crawl4AI as alternative/supplement if needed | 2-3 days |

### Key Decisions Needed

1. **License tolerance:** Is AGPL-3.0 acceptable for Raven? If not, go with Crawl4AI (Apache 2.0) or custom approach.
2. **Backend language:** Node.js or Python? Both Firecrawl and Crawl4AI have Python SDKs. Firecrawl also has a Node.js SDK. Crawl4AI requires Docker API for Node.js.
3. **Hosting constraints:** How much RAM/CPU is available for Playwright browsers? Expect 2-4GB for moderate workloads.
4. **Crawl scale:** Expected number of sites and pages? This impacts the choice between single-instance and distributed crawling.
5. **Content quality bar:** Is Readability-level extraction sufficient, or do certain sites need custom extraction rules?

---

## Appendix: License Summary

| Tool | License | Commercial Use | Copyleft | Notes |
|---|---|---|---|---|
| Firecrawl | AGPL-3.0 | Yes, with conditions | Yes | Must open-source modifications to Firecrawl itself. Using as a service (API) is generally safe. |
| Crawl4AI | Apache 2.0 | Yes, unrestricted | No | Most permissive option. |
| Crawlee | Apache 2.0 | Yes, unrestricted | No | |
| Playwright | Apache 2.0 | Yes, unrestricted | No | |
| Readability | Apache 2.0 | Yes, unrestricted | No | |
| Turndown | MIT | Yes, unrestricted | No | |
| Stagehand | MIT | Yes, unrestricted | No | |
