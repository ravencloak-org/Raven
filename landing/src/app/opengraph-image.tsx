import { ImageResponse } from 'next/og'

export const alt = 'Raven — Self-hostable AI knowledge platform for teams'
export const size = { width: 1200, height: 630 }
export const contentType = 'image/png'
export const dynamic = 'force-static'

export default async function OG() {
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
            fontFamily: 'sans-serif',
            fontSize: 220,
            letterSpacing: 18,
            lineHeight: 1,
            fontWeight: 700,
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
    },
  )
}
