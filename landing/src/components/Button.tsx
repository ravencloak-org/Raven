import Link from 'next/link'
import clsx from 'clsx'

const baseStyles = {
  solid:
    'inline-flex items-center justify-center rounded-full px-5 py-2.5 text-sm font-semibold tracking-tight transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-[var(--color-accent)]',
  outline:
    'inline-flex items-center justify-center rounded-full border px-5 py-2.5 text-sm font-semibold tracking-tight transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:ring-[var(--color-accent)]',
}

const variantStyles = {
  solid: {
    accent:
      'bg-[var(--color-accent)] text-white hover:bg-[var(--color-accent-hover)] active:bg-[var(--color-accent-hover)]',
    ink:
      'bg-[var(--color-ink)] text-white hover:bg-[var(--color-body)] active:bg-[var(--color-body)]',
  },
  outline: {
    accent:
      'border-[var(--color-accent)] text-[var(--color-accent)] hover:bg-[var(--color-accent)]/5',
    ink:
      'border-[var(--color-border)] text-[var(--color-ink)] hover:border-[var(--color-ink)] hover:bg-[var(--color-ink)]/5',
  },
}

type ButtonProps = (
  | { variant?: 'solid'; color?: keyof (typeof variantStyles)['solid'] }
  | { variant: 'outline'; color?: keyof (typeof variantStyles)['outline'] }
) &
  (
    | (Omit<React.ComponentPropsWithoutRef<typeof Link>, 'color'> & { href: string })
    | (Omit<React.ComponentPropsWithoutRef<'button'>, 'color'> & { href?: undefined })
  )

export function Button({ className, ...props }: ButtonProps) {
  const variant = props.variant ?? 'solid'
  const color = props.color ?? 'accent'
  className = clsx(
    baseStyles[variant],
    variant === 'solid'
      ? variantStyles.solid[color as keyof typeof variantStyles.solid]
      : variantStyles.outline[color as keyof typeof variantStyles.outline],
    className,
  )
  return typeof props.href === 'undefined' ? (
    <button className={className} {...(props as React.ComponentPropsWithoutRef<'button'>)} />
  ) : (
    <Link className={className} {...(props as React.ComponentPropsWithoutRef<typeof Link>)} />
  )
}
