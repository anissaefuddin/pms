// Gagal bila ada warna hex literal di komponen dan route. Pakai token (index.css) sebagai gantinya.
// Lewati baris yang memuat "allow-hex" (nilai data atau warna tetap yang disengaja).
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const webRoot = resolve(fileURLToPath(new URL("..", import.meta.url)));
const roots = ["src/components", "src/routes"].map((dir) => join(webRoot, dir));
const pattern = /#[0-9A-Fa-f]{3,8}\b/;
const extensions = /\.(tsx|ts|jsx|js|css)$/;
const hits = [];

function walk(dir) {
  for (const name of readdirSync(dir)) {
    const path = join(dir, name);
    if (statSync(path).isDirectory()) walk(path);
    else if (extensions.test(name)) scan(path);
  }
}

function scan(path) {
  readFileSync(path, "utf8").split("\n").forEach((line, index) => {
    if (line.includes("allow-hex")) return;
    const match = line.match(pattern);
    if (match) hits.push(`${relative(webRoot, path)}:${index + 1}: ${match[0]}`);
  });
}

for (const root of roots) walk(root);

if (hits.length > 0) {
  console.error(`Ditemukan ${hits.length} warna hex. Ganti dengan token atau tambahkan "allow-hex" di baris itu bila disengaja:`);
  for (const hit of hits) console.error(`  ${hit}`);
  process.exit(1);
}
console.log("OK: tidak ada warna hex di src/components dan src/routes");
