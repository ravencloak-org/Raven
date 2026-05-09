import type { DefaultTheme } from "vitepress";

export const mainSidebar: DefaultTheme.SidebarMulti = {
  "/get-started/": [
    {
      text: "Get Started",
      items: [
        { text: "Installation", link: "/get-started/installation" },
        { text: "First Knowledge Base", link: "/get-started/first-knowledge-base" },
        { text: "Embed the Chat Widget", link: "/get-started/embed-the-chat-widget" },
        { text: "Try the Voice Agent", link: "/get-started/try-the-voice-agent" },
      ],
    },
  ],

  "/guides/": [
    {
      text: "Operating Raven",
      items: [
        { text: "Workspaces & Tenancy", link: "/guides/workspaces-and-tenancy" },
        { text: "Ingestion", link: "/guides/ingestion" },
        { text: "Retrieval", link: "/guides/retrieval" },
        { text: "LLM Providers", link: "/guides/llm-providers" },
        { text: "Voice", link: "/guides/voice" },
        { text: "Webhooks & Events", link: "/guides/webhooks-and-events" },
        { text: "Billing", link: "/guides/billing" },
      ],
    },
    {
      text: "Self-Hosting",
      items: [
        { text: "Docker Compose", link: "/guides/self-hosting/docker-compose" },
        { text: "Edge & Raspberry Pi", link: "/guides/self-hosting/edge-and-raspberry-pi" },
        { text: "Traefik & TLS", link: "/guides/self-hosting/traefik-and-tls" },
        { text: "Observability", link: "/guides/self-hosting/observability" },
        { text: "Backups", link: "/guides/self-hosting/backups" },
        { text: "Upgrades", link: "/guides/self-hosting/upgrades" },
        { text: "Hardening", link: "/guides/self-hosting/hardening" },
      ],
    },
  ],

  "/concepts/": [
    {
      text: "Concepts",
      items: [
        { text: "Architecture", link: "/concepts/architecture" },
        { text: "System Overview", link: "/concepts/system-overview" },
        { text: "Data Model", link: "/concepts/data-model" },
        { text: "Multi-Tenancy", link: "/concepts/multi-tenancy" },
        { text: "Hybrid Retrieval", link: "/concepts/hybrid-retrieval" },
        { text: "Deployment Models", link: "/concepts/deployment-models" },
        { text: "AI Worker Design", link: "/concepts/ai-worker-design" },
      ],
    },
  ],

  "/reference/": [
    {
      text: "Reference",
      items: [
        { text: "Configuration", link: "/reference/configuration" },
        { text: "CLI", link: "/reference/cli" },
        { text: "Security Policy", link: "/reference/security-policy" },
        { text: "Changelog", link: "/reference/changelog" },
      ],
    },
  ],

  "/contributing/": [
    {
      text: "Contributing",
      items: [
        { text: "Overview", link: "/contributing/overview" },
        { text: "Dev Setup", link: "/contributing/dev-setup" },
        { text: "Architecture Decisions", link: "/contributing/architecture-decisions" },
        { text: "Dependency Policy", link: "/contributing/dependency-policy" },
        { text: "Release Process", link: "/contributing/release-process" },
      ],
    },
  ],

  "/community/": [
    {
      text: "Community",
      items: [
        { text: "Code of Conduct", link: "/community/code-of-conduct" },
        { text: "Governance", link: "/community/governance" },
        { text: "Maintainers", link: "/community/maintainers" },
        { text: "Support", link: "/community/support" },
      ],
    },
  ],

  "/trust/": [
    {
      text: "Trust",
      items: [
        { text: "OpenSSF Baseline", link: "/trust/openssf-baseline" },
        { text: "OpenSSF Best Practices", link: "/trust/openssf-best-practices" },
        { text: "SLSA Level 3", link: "/trust/slsa-level-3" },
        { text: "Security", link: "/trust/security" },
      ],
    },
  ],
};
