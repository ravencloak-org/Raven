import Link from 'next/link'

export function NavLink({
  href,
  children,
}: {
  href: string
  children: React.ReactNode
}) {
  return (
    <Link
      href={href}
      className="inline-block rounded-lg px-2 py-1 text-sm text-[var(--color-body)] hover:bg-[var(--color-ink)]/5 hover:text-[var(--color-ink)]"
    >
      {children}
    </Link>
  )
}
