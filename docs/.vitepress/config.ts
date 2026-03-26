import { createPhenotypeConfig } from "@phenotype/docs/config";

export default createPhenotypeConfig({
  title: "CLIProxyAPI++",
  description: "CLIProxyAPI++ documentation",
  base: process.env.GITHUB_ACTIONS ? "/cliproxyapi-plusplus/" : "/",
  srcDir: ".",
  githubOrg: "KooshaPari",
  githubRepo: "cliproxyapi-plusplus",
  nav: [
    { text: "Home", link: "/" },
    { text: "Wiki", link: "/wiki/" },
    { text: "Development Guide", link: "/development/" },
    { text: "Document Index", link: "/index/" },
    { text: "API", link: "/api/" },
    { text: "Roadmap", link: "/roadmap/" },
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
        { text: "DevOps and CI/CD", link: "/operations/devops-cicd" },
        { text: "Provider Operations", link: "/provider-operations" },
        { text: "Troubleshooting", link: "/troubleshooting" },
        { text: "Planning Boards", link: "/planning/" },
      ],
    },
    {
      text: "Reference",
      items: [
        { text: "Routing and Models", link: "/routing-reference" },
        { text: "Feature Guides", link: "/features/" },
        { text: "Docsets", link: "/docsets/" },
      ],
    },
    {
      text: "API",
      items: [
        { text: "API Index", link: "/api/" },
        { text: "OpenAI-Compatible API", link: "/api/openai-compatible" },
        { text: "Management API", link: "/api/management" },
        { text: "Operations API", link: "/api/operations" },
      ],
    },
  ],
});
