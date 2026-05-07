import { Container } from '@/components/Container'

export function UpgradeNotes() {
  return (
    <section className="bg-white py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            Upgrades and rollbacks
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Every release is tagged with semver and ships its own migration plan.
            Upgrade in place with <code className="font-mono text-[var(--color-ink)]">docker compose pull &amp;&amp; docker compose up -d</code>.
            Roll back by pinning the previous tag in your Compose file —
            migrations are forward-only but always backward-compatible within a minor version.
          </p>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Subscribe to release notifications by watching the GitHub repo, or follow{' '}
            <a className="text-[var(--color-accent)] underline" href="https://github.com/ravencloak-org/Raven/releases">
              the release feed
            </a>.
          </p>
        </div>
      </Container>
    </section>
  )
}
