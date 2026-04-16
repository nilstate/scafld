export default {
  name: "scafld",
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
            pages: ["cli-reference", "spec-schema", "configuration"],
          },
          {
            group: "Advanced",
            pages: ["validation", "scope-auditing", "invariants", "workspaces"],
          },
        ],
      },
    ],
  },
  navbar: {
    links: [
      { type: "github", href: "https://github.com/nilstate/scafld" },
    ],
  },
};
