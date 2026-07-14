import { defineConfig, godoc, markdown } from "sourcey";

export default defineConfig({
  name: "scafld",
  prettyUrls: "strip",
  repo: "https://github.com/nilstate/scafld",
  editBranch: "main",
  editBasePath: "docs",
  theme: {
    colors: { primary: "#10b981" },
  },
  navigation: {
    tabs: [
      {
        tab: "Documentation",
        slug: "",
        source: markdown({
          groups: [
            {
              group: "Getting Started",
              pages: ["introduction", "quickstart", "installation"],
            },
            {
              group: "Workflow",
              pages: ["lifecycle", "planning", "execution", "review"],
            },
            {
              group: "Reference",
              pages: [
                "cli-reference",
                "spec-schema",
                "configuration",
                "artifacts",
              ],
            },
            {
              group: "Advanced",
              pages: [
                "validation",
                "scope-auditing",
                "invariants",
                "workspaces",
                "sourcey",
              ],
            },
          ],
        }),
      },
      {
        tab: "Go API",
        source: godoc({
          module: "..",
          packages: [
            "./cmd/scafld",
            "./internal/core/...",
            "./internal/app/...",
            "./internal/adapters/...",
            "./internal/platform/...",
          ],
          sourceBasePath: "",
          mode: "live",
          includeTests: true,
          includeUnexported: false,
          hideUndocumented: false,
        }),
      },
    ],
  },
  navbar: {
    links: [{ type: "github", href: "https://github.com/nilstate/scafld" }],
  },
});
