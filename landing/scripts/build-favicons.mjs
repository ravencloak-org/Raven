// One-shot favicon generator. Run manually after logo updates.
// Usage: node scripts/build-favicons.mjs
import sharp from 'sharp'
import { promises as fs } from 'node:fs'
import path from 'node:path'

const SRC = path.resolve('src/images/logo-mark.svg')
const PUB = path.resolve('public')
const APP = path.resolve('src/app')

const sizes = {
  'favicon-16x16.png': 16,
  'favicon-32x32.png': 32,
  'apple-touch-icon.png': 180,
  'android-chrome-192x192.png': 192,
  'android-chrome-512x512.png': 512,
}

async function main() {
  const svg = await fs.readFile(SRC)
  for (const [name, size] of Object.entries(sizes)) {
    const out = path.join(PUB, name)
    await sharp(svg, { density: 384 })
      .resize(size, size, { fit: 'contain', background: { r: 0, g: 0, b: 0, alpha: 0 } })
      .png()
      .toFile(out)
    console.log('wrote', out)
  }
  // App-router convention: src/app/favicon.ico (binary 32×32 PNG renamed; Next handles it).
  await sharp(svg, { density: 384 })
    .resize(32, 32, { fit: 'contain' })
    .toFormat('png')
    .toFile(path.join(APP, 'favicon.ico'))
  console.log('wrote', path.join(APP, 'favicon.ico'))

  await fs.writeFile(
    path.join(PUB, 'site.webmanifest'),
    JSON.stringify(
      {
        name: 'Raven',
        short_name: 'Raven',
        icons: [
          { src: '/android-chrome-192x192.png', sizes: '192x192', type: 'image/png' },
          { src: '/android-chrome-512x512.png', sizes: '512x512', type: 'image/png' },
        ],
        theme_color: '#0a0a0a',
        background_color: '#fafaf9',
        display: 'standalone',
      },
      null,
      2,
    ),
  )
  console.log('wrote site.webmanifest')
}

main().catch((err) => {
  console.error(err)
  process.exit(1)
})
