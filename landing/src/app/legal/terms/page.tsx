import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'

export const metadata: Metadata = {
  title: 'Terms of Service',
  description:
    'Acceptable-use terms for the public Raven demo at demo.raven.ravencloak.org.',
  alternates: { canonical: 'https://raven.ravencloak.org/legal/terms/' },
}

export default function TermsPage() {
  return (
    <>
      <Header />
      <main className="mx-auto max-w-3xl px-6 py-16 prose prose-invert">
        <h1>Terms of Service</h1>
        <p>
          <em>Last updated: 13 May 2026.</em>
        </p>

        <p>
          By signing in to the public Raven demo at{' '}
          <strong>demo.raven.ravencloak.org</strong>, you agree to these
          terms. The demo is provided as a technology preview by Jobin
          Lawrance (the &quot;Operator&quot;), based in India.
        </p>

        <h2>The demo is provided as-is</h2>
        <p>
          The demo is an evaluation environment. There is no service-level
          agreement, no warranty of fitness for any purpose, no warranty of
          availability, and no warranty of data preservation beyond the
          retention policy described in our{' '}
          <a href="/legal/privacy/">Privacy Policy</a>. Use the demo for
          evaluation, not for storing data you cannot afford to lose.
        </p>

        <h2>Acceptable use</h2>
        <p>You agree not to:</p>
        <ul>
          <li>
            Attempt to gain unauthorized access to systems, accounts, or data
            other than your own.
          </li>
          <li>
            Probe, scan, or load-test the demo. The demo has limited capacity
            and abuse harms other evaluators.
          </li>
          <li>
            Submit content that is unlawful, infringing, defamatory, or that
            violates the rights of others.
          </li>
          <li>
            Use the demo to generate content prohibited by the underlying LLM
            provider&apos;s usage policies (Anthropic, OpenAI, or whichever
            provider is configured at the time).
          </li>
          <li>
            Use the demo as a production backend. The demo is rate-limited
            and may be reset, paused, or shut down with no notice.
          </li>
          <li>
            Resell or repackage the demo&apos;s output as a service.
          </li>
        </ul>

        <h2>Account suspension</h2>
        <p>
          We may suspend or terminate any account at any time, with or without
          notice, if we suspect abuse, misuse, or that the account
          jeopardises the demo for other users.
        </p>

        <h2>Demo limits</h2>
        <p>
          The demo enforces per-account quotas and a global daily LLM spend
          ceiling. Once the ceiling is reached, AI features return a &quot;demo
          limit reached&quot; error until the next UTC day.
        </p>

        <h2>Paid features</h2>
        <p>
          The demo runs payment-provider sandboxes only. No real charges
          occur. Paid plans are marked &quot;Coming soon&quot; and lead to a
          waitlist form. We do not bill the demo.
        </p>

        <h2>Open source</h2>
        <p>
          Raven&apos;s source code is published at{' '}
          <a href="https://github.com/ravencloak-org/Raven">
            github.com/ravencloak-org/Raven
          </a>{' '}
          under the Apache 2.0 license. Self-hosting the software is governed
          by that license, not these terms.
        </p>

        <h2>Indemnity and limitation of liability</h2>
        <p>
          To the maximum extent permitted by law, the Operator is not liable
          for any indirect, incidental, or consequential damages arising from
          your use of the demo. You agree to indemnify and hold the Operator
          harmless against claims arising from your misuse of the demo.
        </p>

        <h2>Governing law</h2>
        <p>
          These terms are governed by the laws of India. Any dispute is
          subject to the exclusive jurisdiction of the courts of Karnataka.
        </p>

        <h2>Changes</h2>
        <p>
          We may change these terms. Continued use after changes are posted
          constitutes acceptance.
        </p>

        <h2>Contact</h2>
        <p>
          Questions? Email{' '}
          <a href="mailto:hello@ravencloak.org">hello@ravencloak.org</a>. For
          privacy-specific matters, see our{' '}
          <a href="/legal/privacy/">Privacy Policy</a>.
        </p>
      </main>
      <Footer />
    </>
  )
}
