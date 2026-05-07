import { Button } from '@/components/Button'
import { Container } from '@/components/Container'

const REPO_URL = 'https://github.com/ravencloak-org/Raven'

export function Hero() {
  return (
    <Container className="pt-20 pb-16 text-center lg:pt-32">
      <h1 className="mx-auto max-w-4xl font-display text-5xl font-medium tracking-tight text-[var(--color-ink)] sm:text-7xl">
        Your team&apos;s knowledge,{' '}
        <span className="relative whitespace-nowrap text-[var(--color-accent)]">
          <span className="relative">on your infrastructure.</span>
        </span>
      </h1>
      <p className="mx-auto mt-6 max-w-2xl text-lg tracking-tight text-[var(--color-body)]">
        A self-hostable, multi-tenant RAG platform with built-in voice, chat, and
        edge deployment. GDPR-ready out of the box. Bring your own models.
      </p>
      <div className="mt-10 flex justify-center gap-x-6">
        <Button href="/self-host">Self-host in 5 min</Button>
        <Button href={REPO_URL} variant="outline" color="ink">
          <svg
            aria-hidden="true"
            className="-mr-1 h-5 w-5 flex-none"
            viewBox="0 0 24 24"
            fill="currentColor"
          >
            <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.111.82-.261.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.4 3-.405 1.02.005 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12" />
          </svg>
          Star on GitHub
        </Button>
      </div>
    </Container>
  )
}
