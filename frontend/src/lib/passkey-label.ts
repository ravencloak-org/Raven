/**
 * Build a sensible default passkey label from a User-Agent string.
 *
 * Examples:
 *   "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) ... Chrome/142.0.0.0 Safari/537.36"
 *   -> "MacBook | Chrome 142"
 *
 *   "Mozilla/5.0 (iPhone; CPU iPhone OS 17_4 like Mac OS X) ... Version/17.4 Mobile/15E148 Safari/604.1"
 *   -> "iPhone | Safari 17"
 *
 * Falls back to "This device" if nothing matches.
 */
export function defaultPasskeyLabel(ua: string): string {
  const device = detectDevice(ua)
  const browser = detectBrowser(ua)
  if (device && browser) return device + ' · ' + browser
  if (device) return device
  if (browser) return browser
  return 'This device'
}

function detectDevice(ua: string): string | null {
  if (/iPhone/i.test(ua)) return 'iPhone'
  if (/iPad/i.test(ua)) return 'iPad'
  if (/Android/i.test(ua)) {
    // Crude tablet vs phone split: Android tablets usually omit "Mobile".
    return /Mobile/i.test(ua) ? 'Android phone' : 'Android tablet'
  }
  if (/Macintosh|Mac OS X/i.test(ua)) return 'MacBook'
  if (/Windows NT/i.test(ua)) return 'Windows PC'
  if (/Linux/i.test(ua)) return 'Linux PC'
  if (/CrOS/i.test(ua)) return 'Chromebook'
  return null
}

function detectBrowser(ua: string): string | null {
  // Order matters: Edge ships "Chrome" in its UA, Brave & Opera too. Check the
  // more specific tokens before the generic ones.
  const edge = /Edg\/(\d+)/.exec(ua)
  if (edge) return 'Edge ' + edge[1]

  const opera = /OPR\/(\d+)/.exec(ua)
  if (opera) return 'Opera ' + opera[1]

  const firefox = /Firefox\/(\d+)/.exec(ua)
  if (firefox) return 'Firefox ' + firefox[1]

  // Safari (desktop or iOS WebKit) before Chrome.
  if (/Safari\//.test(ua) && !/Chrome|Chromium|CriOS/.test(ua)) {
    const ver = /Version\/(\d+)/.exec(ua)
    return ver ? 'Safari ' + ver[1] : 'Safari'
  }

  const chrome = /(?:Chrome|CriOS|Chromium)\/(\d+)/.exec(ua)
  if (chrome) return 'Chrome ' + chrome[1]

  return null
}

/**
 * Render an ISO-8601 timestamp as a human-friendly relative time.
 * Returns "just now" for under a minute. Localised via Intl.RelativeTimeFormat.
 */
export function formatRelativeTime(iso: string | null | undefined): string {
  if (!iso) return 'never'
  const then = new Date(iso).getTime()
  if (Number.isNaN(then)) return 'unknown'
  const diffMs = then - Date.now()
  const absMs = Math.abs(diffMs)

  const minute = 60_000
  const hour = 60 * minute
  const day = 24 * hour
  const week = 7 * day
  const month = 30 * day
  const year = 365 * day

  const rtf =
    typeof Intl !== 'undefined' && 'RelativeTimeFormat' in Intl
      ? new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' })
      : null

  let value: number
  let unit: Intl.RelativeTimeFormatUnit
  if (absMs < minute) return 'just now'
  if (absMs < hour) {
    value = Math.round(diffMs / minute)
    unit = 'minute'
  } else if (absMs < day) {
    value = Math.round(diffMs / hour)
    unit = 'hour'
  } else if (absMs < week) {
    value = Math.round(diffMs / day)
    unit = 'day'
  } else if (absMs < month) {
    value = Math.round(diffMs / week)
    unit = 'week'
  } else if (absMs < year) {
    value = Math.round(diffMs / month)
    unit = 'month'
  } else {
    value = Math.round(diffMs / year)
    unit = 'year'
  }
  return rtf ? rtf.format(value, unit) : Math.abs(value) + ' ' + unit + (Math.abs(value) === 1 ? '' : 's') + ' ago'
}
