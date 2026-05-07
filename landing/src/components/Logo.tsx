import Image from 'next/image'
import clsx from 'clsx'

import logoMark from '@/images/logo-mark.svg'

type LogoProps = {
  variant?: 'full' | 'mark'
  inverted?: boolean
  className?: string
  /** Tailwind height class for the bird mark. Defaults to `h-8`. */
  markClassName?: string
}

export function Logo({
  variant = 'full',
  inverted = false,
  className,
  markClassName = 'h-8 w-auto',
}: LogoProps) {
  const mark = (
    <Image
      src={logoMark}
      alt=""
      aria-hidden="true"
      className={clsx(markClassName, inverted && 'invert')}
      priority
    />
  )

  if (variant === 'mark') {
    return (
      <span aria-label="Raven" className={clsx('inline-flex items-center', className)}>
        {mark}
      </span>
    )
  }

  return (
    <span
      aria-label="Raven"
      className={clsx('inline-flex items-center gap-2', className)}
    >
      {mark}
      <span
        className={clsx(
          'raven-wordmark text-2xl leading-none',
          inverted ? 'text-white' : 'text-[var(--color-ink)]',
        )}
      >
        RAVEN
      </span>
    </span>
  )
}
