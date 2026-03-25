<<<<<<< HEAD
import { defineConfig } from "vitepress";
import { contentTabsPlugin } from "./plugins/content-tabs";

const repo = process.env.GITHUB_REPOSITORY?.split("/")[1] ?? "cliproxyapi-plusplus";
const isCI = process.env.GITHUB_ACTIONS === "true";
const docsBase = isCI ? `/${repo}/` : "/";
const faviconHref = `${docsBase}favicon.ico`;

export default defineConfig({
  title: "cliproxy++",
  description: "cliproxyapi-plusplus documentation",
  base: docsBase,
  head: [
    ["link", { rel: "icon", href: faviconHref }]
  ],
  cleanUrls: true,
  ignoreDeadLinks: true,
  lastUpdated: true,
  themeConfig: {
    nav: [
      { text: "Home", link: "/" },
      { text: "Start Here", link: "/start-here" },
      { text: "Tutorials", link: "/tutorials/" },
      { text: "How-to", link: "/how-to/" },
      { text: "Explanation", link: "/explanation/" },
      { text: "Getting Started", link: "/getting-started" },
      { text: "Providers", link: "/provider-usage" },
      { text: "Provider Catalog", link: "/provider-catalog" },
      { text: "Operations", link: "/operations/" },
      { text: "Reference", link: "/routing-reference" },
      { text: "API", link: "/api/" },
      { text: "Docsets", link: "/docsets/" }
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Overview", link: "/" },
          { text: "Getting Started", link: "/getting-started" },
          { text: "Install", link: "/install" },
          { text: "Provider Usage", link: "/provider-usage" },
          { text: "Provider Catalog", link: "/provider-catalog" },
          { text: "Provider Operations", link: "/provider-operations" },
          { text: "Troubleshooting", link: "/troubleshooting" },
          { text: "Planning Boards", link: "/planning/" }
        ]
      },
      {
        text: "Reference",
        items: [
          { text: "Routing and Models", link: "/routing-reference" },
          { text: "Feature Guides", link: "/features/" },
          { text: "Docsets", link: "/docsets/" }
        ]
      },
      {
        text: "API",
        items: [
          { text: "API Index", link: "/api/" },
          { text: "OpenAI-Compatible API", link: "/api/openai-compatible" },
          { text: "Management API", link: "/api/management" },
          { text: "Operations API", link: "/api/operations" }
        ]
      }
    ],
    search: {
      provider: "local"
    },
    footer: {
      message: "MIT Licensed",
      copyright: "Copyright © KooshaPari"
    },
    editLink: {
      pattern:
        "https://github.com/kooshapari/cliproxyapi-plusplus/edit/main/docs/:path",
      text: "Edit this page on GitHub"
    },
    outline: {
      level: [2, 3]
    },
    socialLinks: [
      { icon: "github", link: "https://github.com/kooshapari/cliproxyapi-plusplus" }
    ]
  },

  markdown: {
    config: (md) => {
      md.use(contentTabsPlugin)
    }
  }
});
=======
import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'CLIProxyAPI++',
  description: 'CLIProxyAPI++ documentation',
  srcDir: '.',
  lastUpdated: true,
  cleanUrls: true,
  ignoreDeadLinks: true,
  themeConfig: {
    nav: [
      { text: 'Home', link: '/' },
      { text: 'Wiki', link: '/wiki/' },
      { text: 'Development Guide', link: '/development/' },
      { text: 'Document Index', link: '/index/' },
      { text: 'API', link: '/api/' },
      { text: 'Roadmap', link: '/roadmap/' }
    ],
    sidebar: {
      '/wiki/': [
        { text: 'Wiki (User Guides)', items: [
          { text: 'Overview', link: '/wiki/' }
        ]}
      ],
      '/development/': [
        { text: 'Development Guide', items: [
          { text: 'Overview', link: '/development/' }
        ]}
      ],
      '/index/': [
        { text: 'Document Index', items: [
          { text: 'Overview', link: '/index/' },
          { text: 'Raw/All', link: '/index/raw-all' },
          { text: 'Planning', link: '/index/planning' },
          { text: 'Specs', link: '/index/specs' },
          { text: 'Research', link: '/index/research' },
          { text: 'Worklogs', link: '/index/worklogs' },
          { text: 'Other', link: '/index/other' }
        ]}
      ],
      '/api/': [
        { text: 'API', items: [
          { text: 'Overview', link: '/api/' }
        ]}
      ],
      '/roadmap/': [
        { text: 'Roadmap', items: [
          { text: 'Overview', link: '/roadmap/' }
        ]}
      ],
      '/': [
        { text: 'Quick Links', items: [
          { text: 'Wiki', link: '/wiki/' },
          { text: 'Development Guide', link: '/development/' },
          { text: 'Document Index', link: '/index/' },
          { text: 'API', link: '/api/' },
          { text: 'Roadmap', link: '/roadmap/' }
        ]}
      ]
    },
    search: { provider: 'local' },
    socialLinks: [{ icon: 'github', link: 'https://github.com/KooshaPari/cliproxyapi-plusplus' }]
  }
})
>>>>>>> origin/main
