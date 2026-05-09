import { promises as fs } from "node:fs";
import path from "node:path";
import yaml from "js-yaml";

const REPO_ROOT = path.resolve(import.meta.dirname, "..", "..");
const DIST_API = path.resolve(import.meta.dirname, "..", ".vitepress", "dist", "api");
const SPEC_YAML = path.join(REPO_ROOT, "contracts", "openapi.yaml");

async function main() {
  await fs.mkdir(DIST_API, { recursive: true });

  const yamlContent = await fs.readFile(SPEC_YAML, "utf8");
  await fs.writeFile(path.join(DIST_API, "openapi.yaml"), yamlContent);

  const json = JSON.stringify(yaml.load(yamlContent), null, 2);
  await fs.writeFile(path.join(DIST_API, "openapi.json"), json);

  console.log(`copy-spec: emitted openapi.yaml + openapi.json into ${path.relative(REPO_ROOT, DIST_API)}/`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
