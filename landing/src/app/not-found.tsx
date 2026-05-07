import Link from 'next/link'

import { Container } from '@/components/Container'
import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'

export default function NotFound() {
  return (
    <>
      <Header />
      <main>
        <Container className="flex min-h-[60vh] flex-col items-center justify-center text-center">
          <p className="font-mono text-sm text-[var(--color-body)]">404</p>
          <h1 className="mt-4 font-display text-4xl tracking-tight text-[var(--color-ink)] sm:text-5xl">
            Page not found
          </h1>
          <p className="mt-4 max-w-md text-lg text-[var(--color-body)]">
            That URL doesn&apos;t exist on raven.ravencloak.org. Try the home page or the
            self-host guide.
          </p>
          <div className="mt-10 flex gap-x-4">
            <Link href="/" className="text-[var(--color-accent)] underline">Home</Link>
            <Link href="/self-host" className="text-[var(--color-accent)] underline">Self-host</Link>
          </div>
        </Container>
      </main>
      <Footer />
    </>
  )
}
