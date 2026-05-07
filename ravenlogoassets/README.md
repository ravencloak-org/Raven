# Raven logo assets

Source SVGs for the Raven wordmark + bird, plus a generated icon font (`ravenicons`)
that renders the wordmark by typing the literal characters `RAVEN`.

## Layout

```
ravenlogoassets/
├── Asset 2.svg … Asset 6.svg   # source glyphs for R A V E N (see mapping below)
├── logo/                       # bird logo SVGs (Asset 7, 8, 11, 12)
├── build-font.mjs              # regenerates font/ from the Asset *.svg files
├── package.json                # pins svgtofont
└── font/                       # generated — commit alongside source
    ├── ravenicons.{woff2,woff,ttf,eot,svg}
    ├── ravenicons.symbol.svg   # SVG sprite for <use xlink:href>
    ├── ravenicons.{css,scss,less,module.less,styl}
    ├── react/                  # React components (one per glyph)
    ├── info.json               # codepoint manifest
    └── preview.html            # open in a browser to verify rendering
```

## Glyph → codepoint map

Each source SVG is wired to its real ASCII codepoint, so any element using
`font-family: ravenicons` will render the wordmark when the text is `RAVEN`:

| Letter | Codepoint | Source        |
| ------ | --------- | ------------- |
| R      | `U+0052`  | `Asset 6.svg` |
| A      | `U+0041`  | `Asset 5.svg` |
| V      | `U+0056`  | `Asset 4.svg` |
| E      | `U+0045`  | `Asset 3.svg` |
| N      | `U+004E`  | `Asset 2.svg` |

## Using the font

### Plain HTML / CSS

```html
<link rel="stylesheet" href="/path/to/font/ravenicons.css" />
<span style="font-family: 'ravenicons'; font-size: 8rem;">RAVEN</span>
```

The `ravenicons.css` file declares the `@font-face` and per-glyph helper
classes (`.ravenicons-R`, `.ravenicons-A`, `.ravenicons-V`, `.ravenicons-E`,
`.ravenicons-N`) that emit the glyph via `::before`.

### Vue 3 (frontend/)

Import the CSS once (e.g. in `src/main.ts` or your global stylesheet):

```ts
import 'ravenlogoassets/font/ravenicons.css';
```

Then anywhere in a template:

```vue
<span class="raven-logo">RAVEN</span>

<style scoped>
.raven-logo {
  font-family: 'ravenicons';
  font-size: 6rem;
  letter-spacing: 0.05em;
}
</style>
```

### React

Either import the generated component:

```tsx
import { R, A, V, E, N } from 'ravenlogoassets/font/react';
```

…or use the same CSS approach with the `font-family` set to `ravenicons`.

### SVG sprite (no font load)

```html
<svg><use xlink:href="font/ravenicons.symbol.svg#ravenicons-R" /></svg>
```

## Regenerating the font

```bash
cd ravenlogoassets
npm install            # one-time, installs svgtofont
node build-font.mjs    # regenerates font/ from Asset *.svg
```

The build script:

1. Stages `Asset 2.svg … Asset 6.svg` into a temp dir under their letter names.
2. Runs `svgtofont` with `getIconUnicode` set so each glyph maps to its real
   ASCII codepoint (not the default Private Use Area).
3. Cleans up the staging dir.

Output is deterministic — `hasTimestamp: false` keeps CSS diffs stable across
rebuilds.

## Adding or replacing a glyph

1. Drop the new SVG into this directory as `Asset N.svg`.
2. Add an entry to `ASSET_TO_LETTER` in `build-font.mjs` mapping it to a letter.
3. Run `node build-font.mjs`.
4. Commit the source SVG and the regenerated `font/` together.

## Visual smoke-test

Open `font/preview.html` in any browser — you should see the wordmark
rendered at 12rem, then individual letters via the helper classes.
