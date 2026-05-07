import { ImageResponse } from 'next/og'
import { readFileSync } from 'node:fs'
import path from 'node:path'

export const alt = 'Raven — Self-hostable AI knowledge platform for teams'
export const size = { width: 1200, height: 630 }
export const contentType = 'image/png'
export const dynamic = 'force-static'

export default async function OG() {
  // satori (next/og) does not accept WOFF2; use the TTF for server-side rendering.
  const ravenicons = readFileSync(
    path.join(process.cwd(), 'public/fonts/ravenicons.ttf'),
  )

  return new ImageResponse(
    (
      <div
        style={{
          height: '100%',
          width: '100%',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#0a0a0a',
          color: 'white',
        }}
      >
        <div
          style={{
            fontFamily: 'ravenicons',
            fontSize: 220,
            letterSpacing: 18,
            lineHeight: 1,
          }}
        >
          RAVEN
        </div>
        <div
          style={{
            marginTop: 40,
            fontSize: 36,
            color: '#06b6d4',
            maxWidth: 980,
            textAlign: 'center',
            fontFamily: 'sans-serif',
          }}
        >
          Your team&apos;s knowledge, on your infrastructure.
        </div>
      </div>
    ),
    {
      ...size,
      fonts: [{ name: 'ravenicons', data: ravenicons, weight: 400, style: 'normal' }],
    },
  )
}
