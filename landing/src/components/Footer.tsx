import Link from 'next/link'

import { Container } from '@/components/Container'
import { Logo } from '@/components/Logo'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

export function Footer() {
  return (
    <footer className="bg-[var(--color-ink)] text-[var(--color-bg)]">
      <Container>
        <div className="py-16">
          <Logo variant="mark" inverted markClassName="h-10 w-auto" className="mx-auto" />
          <nav className="mt-10 text-sm" aria-label="Footer">
            <ul className="-my-1 flex flex-wrap justify-center gap-x-6 gap-y-1">
              <li><Link href="/#features" className="hover:text-white">Features</Link></li>
              <li><Link href="/pricing" className="hover:text-white">Pricing</Link></li>
              <li><Link href="/self-host" className="hover:text-white">Self-host</Link></li>
              <li><Link href="/about" className="hover:text-white">About</Link></li>
              <li><Link href={REPO_URL} className="hover:text-white">GitHub</Link></li>
            </ul>
          </nav>
        </div>
        <div className="flex flex-col items-center border-t border-white/10 py-10 sm:flex-row-reverse sm:justify-between">
          <div className="flex gap-x-6">
            <Link href={REPO_URL} className="text-sm text-white/60 hover:text-white" aria-label="Raven on GitHub">
              GitHub
            </Link>
          </div>
          <p className="mt-6 text-sm text-white/60 sm:mt-0">
            © {new Date().getFullYear()} Ravencloak. Source-available under the Raven licence.
          </p>
        </div>
      </Container>
    </footer>
  )
}
