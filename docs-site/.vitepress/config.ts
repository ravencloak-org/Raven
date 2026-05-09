import { defineConfig } from "vitepress";

export default defineConfig({
  title: "Raven Docs",
  description:
    "Open-source multi-tenant knowledge base platform with AI chat, voice, and WhatsApp.",
  lang: "en-US",
  cleanUrls: true,
  lastUpdated: true,

  // Read content from .tmp/, populated by scripts/sync-content.ts at
  // build time. T2 ships this empty; T4 wires the sync.
  srcDir: ".tmp",

  head: [
    ["link", { rel: "icon", href: "/favicon.svg", type: "image/svg+xml" }],
    ["meta", { name: "theme-color", content: "#e11d48" }],
    ["meta", { property: "og:type", content: "website" }],
    ["meta", { property: "og:site_name", content: "Raven Docs" }],
  ],

  themeConfig: {
    nav: [
      { text: "Get Started", link: "/get-started/installation" },
      { text: "Guides", link: "/guides/workspaces-and-tenancy" },
      { text: "Concepts", link: "/concepts/architecture" },
      { text: "API", link: "/api/overview" },
      { text: "Contributing", link: "/contributing/overview" },
    ],

    // Sidebar contributions are merged in by T5 (main IA) and T6 (API).
    sidebar: {},

    socialLinks: [
      { icon: "github", link: "https://github.com/ravencloak-org/Raven" },
    ],

    editLink: {
      // Edit-on-GitHub points at the SOURCE markdown in docs/ or the
      // repo root, not the synced .tmp/ copy. T4 wires the path
      // rewrite when sync-content runs.
      pattern:
        "https://github.com/ravencloak-org/Raven/edit/main/docs/:path",
      text: "Edit this page on GitHub",
    },

    footer: {
      // T7 fills the message + copyright text.
      message: "",
      copyright: "",
    },

    search: {
      provider: "local",
    },
  },
});
