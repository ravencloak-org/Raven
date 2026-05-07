import { Container } from '@/components/Container'

export function Maintainers() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Who&apos;s behind it
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven is maintained by Jobin Lawrance — previously building Keycloak
            Service Provider Interfaces, now full-time on auth and platform
            infrastructure. Based in Bengaluru, India.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            The codebase is open on{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven">
              GitHub
            </a>
            . Contributions, issues, and pattern discussions welcome.
          </p>
          <p className="mt-8 text-sm text-[var(--color-body)]">
            Source-available under the Raven licence; see{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/blob/main/LICENSE">
              LICENSE
            </a>{' '}
            and{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/blob/main/ee-LICENSE">
              ee-LICENSE
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
