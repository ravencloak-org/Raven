import { type Metadata } from 'next'

import { Footer } from '@/components/Footer'
import { Header } from '@/components/Header'

export const metadata: Metadata = {
  title: 'Privacy Policy',
  description:
    'How Raven collects, uses, and protects personal data on the public demo at demo.raven.ravencloak.org.',
  alternates: { canonical: 'https://raven.ravencloak.org/legal/privacy/' },
}

export default function PrivacyPage() {
  return (
    <>
      <Header />
      <main className="mx-auto max-w-3xl px-6 py-16 prose prose-invert">
        <h1>Privacy Policy</h1>
        <p>
          <em>Last updated: 13 May 2026.</em>
        </p>

        <p>
          This Privacy Policy explains how Raven (&quot;we&quot;, &quot;us&quot;)
          collects, uses, and shares personal data when you use the public
          demo at <strong>demo.raven.ravencloak.org</strong>. The demo is
          operated by Jobin Lawrance as an individual, based in India.
        </p>

        <h2>Data we collect</h2>
        <ul>
          <li>
            <strong>Account profile.</strong> When you sign in with Google we
            receive your email address, name, profile photo, and Google
            account ID. We do not request additional Google scopes.
          </li>
          <li>
            <strong>Application data.</strong> Workspaces, conversations,
            messages, and any content you choose to create inside the demo.
          </li>
          <li>
            <strong>Operational telemetry.</strong> IP address, browser
            user-agent, timestamps, request paths. Used for security,
            abuse-prevention, and debugging. Held in our log aggregator
            (OpenObserve) for 30 days then deleted.
          </li>
          <li>
            <strong>Cookies.</strong> Essential session cookies only (SuperTokens
            authentication and CSRF protection). No advertising or analytics
            cookies are set on this demo. Cloudflare may set its own
            anti-abuse cookies on the edge.
          </li>
        </ul>

        <h2>Legal basis (GDPR / DPDP)</h2>
        <ul>
          <li>
            <strong>Legitimate interest</strong> (GDPR Art. 6(1)(f)) — for
            operating the demo, preventing abuse, and securing user accounts.
          </li>
          <li>
            <strong>Consent</strong> — for any optional cookies; none are
            currently set on the demo.
          </li>
          <li>
            <strong>Contract</strong> (GDPR Art. 6(1)(b)) — to deliver the
            requested service when you sign in.
          </li>
        </ul>

        <h2>How long we keep your data</h2>
        <p>
          Inactive accounts are deleted automatically <strong>30 days</strong>{' '}
          after your last sign-in. You receive a warning email and an in-app
          banner 7 days before deletion. Backups are retained for 30 days
          (logical dumps) and 14 days (volume snapshots). You can request
          immediate deletion at any time via the in-app{' '}
          <em>Delete my account</em> control.
        </p>

        <h2>Recipients and processors</h2>
        <table>
          <thead>
            <tr>
              <th>Processor</th>
              <th>Purpose</th>
              <th>Region</th>
            </tr>
          </thead>
          <tbody>
            <tr>
              <td>Amazon Web Services (EC2, S3, SSM)</td>
              <td>Hosting and backups</td>
              <td>ap-south-1 (Mumbai)</td>
            </tr>
            <tr>
              <td>Cloudflare (Tunnel, Access, Turnstile, DNS)</td>
              <td>Edge proxy, anti-bot, login gate</td>
              <td>Global anycast</td>
            </tr>
            <tr>
              <td>Google (OAuth)</td>
              <td>Federated sign-in</td>
              <td>Global</td>
            </tr>
            <tr>
              <td>Resend</td>
              <td>Transactional email (retention notices, DSAR confirmations)</td>
              <td>EU / US</td>
            </tr>
            <tr>
              <td>LLM provider (Anthropic, OpenAI, or self-hosted)</td>
              <td>Generating AI responses</td>
              <td>US</td>
            </tr>
            <tr>
              <td>Razorpay / Hyperswitch (sandbox only on demo)</td>
              <td>Payment UI rehearsal</td>
              <td>India / Global</td>
            </tr>
          </tbody>
        </table>

        <h2>Your rights</h2>
        <ul>
          <li>
            <strong>Access / export.</strong> Use{' '}
            <em>Account settings → Export my data</em> to download a JSON
            archive of your data.
          </li>
          <li>
            <strong>Erasure.</strong> Use <em>Account settings → Delete my
            account</em>. We confirm via email and irreversibly delete within
            24 hours.
          </li>
          <li>
            <strong>Rectification.</strong> Edit your profile and content
            in-app.
          </li>
          <li>
            <strong>Complain.</strong> If you believe your data has been
            mishandled, email{' '}
            <a href="mailto:privacy@ravencloak.org">privacy@ravencloak.org</a>{' '}
            or contact your local data-protection authority.
          </li>
        </ul>

        <h2>Security</h2>
        <p>
          Data is encrypted at rest (AWS-managed AES-256 on EBS and S3) and in
          transit (TLS via Cloudflare). Access to the host is restricted to
          AWS Systems Manager — no inbound SSH ports are open.
        </p>

        <h2>Changes</h2>
        <p>
          We update this policy as the demo evolves. Material changes are
          announced via an in-app banner at next sign-in.
        </p>

        <h2>Contact</h2>
        <p>
          Questions? Email{' '}
          <a href="mailto:privacy@ravencloak.org">privacy@ravencloak.org</a>.
        </p>
      </main>
      <Footer />
    </>
  )
}
