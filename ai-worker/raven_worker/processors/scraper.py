"""Web scraping using httpx and BeautifulSoup4.

Extracts text content from URLs, handles redirects, and respects robots.txt.
"""

from __future__ import annotations

import urllib.robotparser
from dataclasses import dataclass, field
from urllib.parse import urlparse

import httpx
import structlog
from bs4 import BeautifulSoup

logger = structlog.get_logger(__name__)

_DEFAULT_USER_AGENT = "RavenBot/1.0 (+https://github.com/AumniPrime/raven)"
_DEFAULT_TIMEOUT = 30.0


@dataclass
class ScrapeResult:
    """Structured output from web scraping."""

    url: str
    text: str
    title: str = ""
    metadata: dict[str, str] = field(default_factory=dict)


class ScraperError(Exception):
    """Raised when scraping fails."""


class RobotsTxtDisallowedError(ScraperError):
    """Raised when robots.txt disallows crawling the URL."""


class WebScraper:
    """Scrape web pages using httpx + BeautifulSoup4.

    Features:
    - Follows redirects (httpx default behaviour)
    - Respects robots.txt
    - Extracts clean text from HTML
    - Captures page title and meta description
    """

    def __init__(
        self,
        user_agent: str = _DEFAULT_USER_AGENT,
        timeout: float = _DEFAULT_TIMEOUT,
        respect_robots: bool = True,
    ) -> None:
        self._user_agent = user_agent
        self._timeout = timeout
        self._respect_robots = respect_robots

    async def scrape(self, url: str) -> str:
        """Scrape a web page and return its text content.

        Args:
            url: The URL to scrape.

        Returns:
            Extracted text content from the page.

        Raises:
            RobotsTxtDisallowedError: If robots.txt forbids the URL.
            ScraperError: On network or parsing errors.
        """
        result = await self.scrape_structured(url)
        return result.text

    async def scrape_structured(self, url: str) -> ScrapeResult:
        """Scrape a web page and return structured output.

        Args:
            url: The URL to scrape.

        Returns:
            A ScrapeResult with text, title, and metadata.
        """
        logger.info("scrape_url", url=url)

        if self._respect_robots:
            await self._check_robots_txt(url)

        html = await self._fetch(url)
        return self._extract(url, html)

    async def _check_robots_txt(self, url: str) -> None:
        """Check robots.txt to see if we are allowed to fetch the URL.

        Raises:
            RobotsTxtDisallowedError: If the URL is disallowed.
        """
        parsed = urlparse(url)
        robots_url = f"{parsed.scheme}://{parsed.netloc}/robots.txt"

        rp = urllib.robotparser.RobotFileParser()

        try:
            async with httpx.AsyncClient(
                timeout=self._timeout,
                follow_redirects=True,
                headers={"User-Agent": self._user_agent},
            ) as client:
                resp = await client.get(robots_url)

                if resp.status_code == 200:
                    rp.parse(resp.text.splitlines())
                else:
                    # No robots.txt or error fetching it -- assume allowed
                    logger.debug(
                        "robots_txt_unavailable",
                        url=robots_url,
                        status=resp.status_code,
                    )
                    return
        except httpx.HTTPError:
            # Network error fetching robots.txt -- assume allowed
            logger.debug("robots_txt_fetch_error", url=robots_url, exc_info=True)
            return

        if not rp.can_fetch(self._user_agent, url):
            raise RobotsTxtDisallowedError(
                f"robots.txt disallows fetching {url} for user-agent {self._user_agent}"
            )

    async def _fetch(self, url: str) -> str:
        """Fetch the raw HTML from a URL.

        Args:
            url: The URL to fetch.

        Returns:
            HTML content as a string.

        Raises:
            ScraperError: On network errors or non-200 responses.
        """
        try:
            async with httpx.AsyncClient(
                timeout=self._timeout,
                follow_redirects=True,
                headers={"User-Agent": self._user_agent},
            ) as client:
                response = await client.get(url)
                response.raise_for_status()
                return response.text
        except httpx.HTTPStatusError as exc:
            raise ScraperError(f"HTTP {exc.response.status_code} fetching {url}") from exc
        except httpx.HTTPError as exc:
            raise ScraperError(f"Network error fetching {url}: {exc}") from exc

    @staticmethod
    def _extract(url: str, html: str) -> ScrapeResult:
        """Extract text content from HTML using BeautifulSoup.

        Removes script/style/nav elements and extracts clean text.
        """
        soup = BeautifulSoup(html, "html.parser")

        # Extract title
        title = ""
        title_tag = soup.find("title")
        if title_tag:
            title = title_tag.get_text(strip=True)

        # Extract meta description
        meta_desc = ""
        meta_tag = soup.find("meta", attrs={"name": "description"})
        if meta_tag and meta_tag.get("content"):
            meta_desc = str(meta_tag["content"])

        # Remove non-content elements
        for tag in soup(["script", "style", "nav", "footer", "header", "aside", "noscript"]):
            tag.decompose()

        # Extract text with newline separators between block elements
        text = soup.get_text(separator="\n", strip=True)

        # Collapse multiple blank lines
        lines = text.splitlines()
        cleaned_lines: list[str] = []
        prev_blank = False
        for line in lines:
            stripped = line.strip()
            if not stripped:
                if not prev_blank:
                    cleaned_lines.append("")
                prev_blank = True
            else:
                cleaned_lines.append(stripped)
                prev_blank = False

        text = "\n".join(cleaned_lines).strip()

        metadata: dict[str, str] = {"url": url}
        if meta_desc:
            metadata["description"] = meta_desc

        logger.info("scrape_complete", url=url, text_length=len(text), title=title)

        return ScrapeResult(url=url, text=text, title=title, metadata=metadata)
