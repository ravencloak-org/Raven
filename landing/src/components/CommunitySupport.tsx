import { Container } from '@/components/Container'

export function CommunitySupport() {
  return (
    <section className="bg-[var(--color-bg)] py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Community support
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Self-hosted Raven is fully supported by the open community. Open an issue or
            a discussion on GitHub — the maintainers and other operators are usually
            responsive within a day.
          </p>
          <ul className="mt-6 list-disc space-y-2 pl-6 text-[var(--color-body)]">
            <li>
              Bugs / feature requests:{' '}
              <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/issues">
                GitHub Issues
              </a>
            </li>
            <li>
              Operational questions, pattern advice:{' '}
              <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/discussions">
                GitHub Discussions
              </a>
            </li>
            <li>Need a paid SLA? <a className="text-[var(--color-accent)] underline" href="/pricing/">See Cloud Pro</a>.</li>
          </ul>
        </div>
      </Container>
    </section>
  )
}
