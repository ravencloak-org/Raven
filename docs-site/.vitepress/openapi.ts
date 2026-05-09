import { useSidebar } from "vitepress-openapi";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import yaml from "js-yaml";

const specPath = resolve(import.meta.dirname, "..", "..", "contracts", "openapi.yaml");
const spec = yaml.load(readFileSync(specPath, "utf8")) as Record<string, unknown>;

const sidebarBuilder = useSidebar({
  spec,
  prefix: "/api/",
  // One sidebar group per OpenAPI tag.
  collapsible: true,
});

export const apiSidebar = sidebarBuilder.generateSidebarGroups();
export { spec };
