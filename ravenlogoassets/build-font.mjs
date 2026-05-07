import svgtofont from 'svgtofont';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Map Asset N.svg -> letter glyph (these spell RAVEN at U+0052/41/56/45/4E)
const ASSET_TO_LETTER = {
  'Asset 2.svg': 'N',
  'Asset 3.svg': 'E',
  'Asset 4.svg': 'V',
  'Asset 5.svg': 'A',
  'Asset 6.svg': 'R',
};

const stageDir = path.join(__dirname, '.svgs-staged');
const distDir = path.join(__dirname, 'font');

fs.rmSync(stageDir, { recursive: true, force: true });
fs.mkdirSync(stageDir);

for (const [asset, letter] of Object.entries(ASSET_TO_LETTER)) {
  const src = path.join(__dirname, asset);
  if (!fs.existsSync(src)) throw new Error(`Missing source SVG: ${asset}`);
  fs.copyFileSync(src, path.join(stageDir, `${letter}.svg`));
}

await svgtofont({
  src: stageDir,
  dist: distDir,
  fontName: 'ravenicons',
  css: { hasTimestamp: false },
  emptyDist: true,
  outSVGReact: true,
  outSVGReactNative: false,
  generateInfoData: true,
  // Map each filename (R, A, V, E, N) to its real ASCII codepoint so that
  // typing "RAVEN" in font-family: ravenicons renders the logo letters.
  getIconUnicode: (name) => {
    const code = name.toUpperCase().charCodeAt(0);
    return [String.fromCharCode(code), code + 1];
  },
});

fs.rmSync(stageDir, { recursive: true, force: true });
console.log('Done.');
