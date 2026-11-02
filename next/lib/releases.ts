import fs from "node:fs";
import path from "node:path";
export const repo = "https://github.com/wrestle-R/spotify-tui-rdp";
const root = path.resolve(process.cwd(), "..");
export function versions() {
  return fs
    .readdirSync(path.join(root, "docs/releases"))
    .filter((name) => /^v\d+\.\d+\.\d+\.md$/.test(name))
    .map((name) => name.slice(0, -3))
    .sort((a, b) =>
      b.slice(1).localeCompare(a.slice(1), undefined, { numeric: true }),
    );
}
export function latest() {
  return versions()[0];
}
export function changelog() {
  return fs.readFileSync(path.join(root, "CHANGELOG.md"), "utf8");
}
export function release(version: string) {
  if (!versions().includes(version)) return null;
  return fs.readFileSync(
    path.join(root, "docs/releases", `${version}.md`),
    "utf8",
  );
}
export function download(target: "linux_amd64" | "darwin_arm64" = "linux_amd64") {
  const v = latest();
  return `${repo}/releases/download/${v}/spotify-tui_${v.slice(1)}_${target}.tar.gz`;
}
