import { promises as fs } from "node:fs";
import path from "node:path";
import process from "node:process";

interface Mapping { from: string; to: string }
interface LinkRewrite { match: string; replace: string }

interface ContentMap {
  mappings: Mapping[];
  linkRewrites: LinkRewrite[];
}

const REPO_ROOT = path.resolve(import.meta.dirname, "..", "..");
const DOCS_SITE = path.resolve(import.meta.dirname, "..");
const TMP_DIR = path.join(DOCS_SITE, ".tmp");

// Repo files that are quoted from synced docs but live outside the docs-site
// IA (LICENSE, README, MAINTAINERS, etc.) get rewritten to canonical GitHub
// blob URLs so the build doesn't fail with dead-link errors.
const GITHUB_BLOB_BASE =
  "https://github.com/ravencloak-org/Raven/blob/main";

async function loadMap(): Promise<ContentMap> {
  const raw = await fs.readFile(
    path.join(DOCS_SITE, "content-map.json"),
    "utf8",
  );
  return JSON.parse(raw);
}

function rewriteLinks(
  content: string,
  rewrites: LinkRewrite[],
  fromRepoPath: string,
): string {
  // Source file's directory relative to repo root, used to resolve any
  // relative target paths the markdown might contain.
  const fromDir = path.posix.dirname(fromRepoPath.split(path.sep).join("/"));

  // Match Markdown links of the form [text](path/to/file.md) and rewrite
  // their path component when it matches one of the configured patterns.
  return content.replace(/\]\(([^)]+)\)/g, (whole, target: string) => {
    // External, anchor-only, or absolute-from-site links pass through.
    if (/^(https?:|#|mailto:|\/)/.test(target)) return whole;

    // Strip optional anchor for matching, preserve it for output.
    const [pathPart, anchor = ""] = target.split("#");
    const suffix = anchor ? `#${anchor}` : "";

    // First, try the configured repo-relative linkRewrites. These take a
    // full repo-relative path (e.g. `docs/architecture.md`), so resolve
    // the target against the source file's directory before matching.
    const repoRelative = path.posix
      .normalize(path.posix.join(fromDir, pathPart))
      .replace(/^\.\//, "");
    for (const rule of rewrites) {
      const re = new RegExp(rule.match);
      if (re.test(repoRelative)) {
        // Drop a trailing slash on the replacement so VitePress's
        // cleanUrls dead-link checker doesn't treat the URL as a
        // directory looking for an index.md.
        const newPath = repoRelative.replace(re, rule.replace).replace(/\/$/, "");
        return `](${newPath}${suffix})`;
      }
    }

    // No mapped destination — point the link at the file on GitHub so the
    // VitePress dead-link checker doesn't fail the build for repo-internal
    // references that aren't part of the docs IA (LICENSE, README, etc.).
    // Anchor-bearing links keep their anchor on the GitHub side too.
    return `](${GITHUB_BLOB_BASE}/${repoRelative}${suffix})`;
  });
}

// VitePress runs every Markdown file through the Vue template compiler,
// so any literal `{{ ... }}` token (e.g. a GitHub Actions expression like
// `${{ github.event.* }}` or a Go template `{{ .Manifest.Digest }}` shown
// in a code span) is parsed as a Vue interpolation and breaks the build.
// We escape the opening braces with HTML entities — Markdown renders the
// entities back to `{{` in the output, so the rendered text is unchanged.
function escapeVueInterpolation(content: string): string {
  return content.replace(/\{\{/g, "&#123;&#123;");
}

async function copyFile(from: string, to: string, rewrites: LinkRewrite[]) {
  const src = path.join(REPO_ROOT, from);
  const dest = path.join(TMP_DIR, to);
  await fs.mkdir(path.dirname(dest), { recursive: true });

  let content: string;
  try {
    content = await fs.readFile(src, "utf8");
  } catch (err: unknown) {
    if ((err as NodeJS.ErrnoException).code === "ENOENT") {
      console.warn(`sync-content: missing source ${from} (skipping)`);
      return;
    }
    throw err;
  }
  await fs.writeFile(
    dest,
    escapeVueInterpolation(rewriteLinks(content, rewrites, from)),
  );
}

async function main() {
  const map = await loadMap();
  await fs.rm(TMP_DIR, { recursive: true, force: true });
  await fs.mkdir(TMP_DIR, { recursive: true });

  for (const m of map.mappings) {
    await copyFile(m.from, m.to, map.linkRewrites);
  }

  console.log(
    `sync-content: copied ${map.mappings.length} files to ${path.relative(REPO_ROOT, TMP_DIR)}/`,
  );
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
