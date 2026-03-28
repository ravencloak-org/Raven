"""Tests for the WebScraper (httpx + BeautifulSoup4)."""

from unittest.mock import AsyncMock, patch

import httpx
import pytest

from raven_worker.processors.scraper import (
    RobotsTxtDisallowedError,
    ScraperError,
    ScrapeResult,
    WebScraper,
)


@pytest.fixture
def scraper():
    """Return a WebScraper with robots.txt checking disabled for most tests."""
    return WebScraper(respect_robots=False)


@pytest.fixture
def scraper_with_robots():
    """Return a WebScraper that respects robots.txt."""
    return WebScraper(respect_robots=True)


def _mock_response(text: str, status_code: int = 200) -> httpx.Response:
    """Create a mock httpx.Response."""
    return httpx.Response(
        status_code=status_code,
        text=text,
        request=httpx.Request("GET", "https://example.com"),
    )


# --- Basic scraping ---


@pytest.mark.asyncio
async def test_scrape_extracts_text(scraper):
    html = "<html><body><h1>Title</h1><p>Hello world.</p></body></html>"
    resp = _mock_response(html)

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=resp):
        result = await scraper.scrape("https://example.com")

    assert "Title" in result
    assert "Hello world" in result


@pytest.mark.asyncio
async def test_scrape_structured_returns_result(scraper):
    html = (
        "<html><head><title>My Page</title>"
        '<meta name="description" content="A test page.">'
        "</head><body><p>Content here.</p></body></html>"
    )
    resp = _mock_response(html)

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=resp):
        result = await scraper.scrape_structured("https://example.com/page")

    assert isinstance(result, ScrapeResult)
    assert result.title == "My Page"
    assert "Content here" in result.text
    assert result.metadata["description"] == "A test page."
    assert result.url == "https://example.com/page"


@pytest.mark.asyncio
async def test_scrape_strips_scripts_and_styles(scraper):
    html = (
        "<html><body>"
        "<script>var x = 1;</script>"
        "<style>.foo { color: red; }</style>"
        "<p>Visible content.</p>"
        "</body></html>"
    )
    resp = _mock_response(html)

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=resp):
        result = await scraper.scrape("https://example.com")

    assert "var x" not in result
    assert "color: red" not in result
    assert "Visible content" in result


@pytest.mark.asyncio
async def test_scrape_strips_nav_footer(scraper):
    html = (
        "<html><body>"
        "<nav><a href='/'>Home</a></nav>"
        "<main><p>Main content.</p></main>"
        "<footer>Copyright 2025</footer>"
        "</body></html>"
    )
    resp = _mock_response(html)

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=resp):
        result = await scraper.scrape("https://example.com")

    assert "Main content" in result
    assert "Home" not in result
    assert "Copyright" not in result


# --- Error handling ---


@pytest.mark.asyncio
async def test_scrape_http_error_raises(scraper):
    resp = _mock_response("Not Found", status_code=404)

    with (
        patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=resp),
        pytest.raises(ScraperError, match="HTTP 404"),
    ):
        await scraper.scrape("https://example.com/missing")


@pytest.mark.asyncio
async def test_scrape_network_error_raises(scraper):
    with (
        patch(
            "httpx.AsyncClient.get",
            new_callable=AsyncMock,
            side_effect=httpx.ConnectError("Connection refused"),
        ),
        pytest.raises(ScraperError, match="Network error"),
    ):
        await scraper.scrape("https://unreachable.test")


# --- robots.txt ---


@pytest.mark.asyncio
async def test_robots_txt_allows(scraper_with_robots):
    """When robots.txt allows the URL, scraping should proceed."""
    robots_txt = "User-agent: *\nAllow: /\n"
    robots_resp = _mock_response(robots_txt)
    page_resp = _mock_response("<html><body><p>Allowed page.</p></body></html>")

    async def mock_get(url, **kwargs):
        if "robots.txt" in str(url):
            return robots_resp
        return page_resp

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, side_effect=mock_get):
        result = await scraper_with_robots.scrape("https://example.com/page")

    assert "Allowed page" in result


@pytest.mark.asyncio
async def test_robots_txt_disallows(scraper_with_robots):
    """When robots.txt disallows the URL, RobotsTxtDisallowedError should be raised."""
    robots_txt = "User-agent: *\nDisallow: /secret\n"
    robots_resp = _mock_response(robots_txt)

    with (
        patch("httpx.AsyncClient.get", new_callable=AsyncMock, return_value=robots_resp),
        pytest.raises(RobotsTxtDisallowedError, match="disallows"),
    ):
        await scraper_with_robots.scrape("https://example.com/secret/page")


@pytest.mark.asyncio
async def test_robots_txt_unavailable_allows(scraper_with_robots):
    """When robots.txt returns 404, scraping should proceed."""
    robots_resp = _mock_response("", status_code=404)
    page_resp = _mock_response("<html><body><p>No robots file.</p></body></html>")

    async def mock_get(url, **kwargs):
        if "robots.txt" in str(url):
            return robots_resp
        return page_resp

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, side_effect=mock_get):
        result = await scraper_with_robots.scrape("https://example.com/page")

    assert "No robots file" in result


@pytest.mark.asyncio
async def test_robots_txt_network_error_allows(scraper_with_robots):
    """When fetching robots.txt fails with a network error, allow scraping."""
    page_resp = _mock_response("<html><body><p>Robots fetch failed.</p></body></html>")

    call_count = 0

    async def mock_get(url, **kwargs):
        nonlocal call_count
        call_count += 1
        if call_count == 1:
            # First call is robots.txt
            raise httpx.ConnectError("Connection refused")
        return page_resp

    with patch("httpx.AsyncClient.get", new_callable=AsyncMock, side_effect=mock_get):
        result = await scraper_with_robots.scrape("https://example.com/page")

    assert "Robots fetch failed" in result
