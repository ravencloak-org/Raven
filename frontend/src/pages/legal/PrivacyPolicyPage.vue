<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-900">
    <div class="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8">
      <div class="rounded-2xl bg-white p-8 shadow-2xl sm:p-12">
        <header class="mb-10 border-b border-gray-200 pb-8">
          <h1 class="text-3xl font-bold tracking-tight text-gray-900">
            Privacy Policy
          </h1>
          <p class="mt-2 text-sm text-gray-500">
            Last updated: March 2026
          </p>
          <nav class="mt-4">
            <RouterLink
              to="/terms"
              class="text-sm font-medium text-indigo-600 transition-colors hover:text-indigo-800"
            >
              View Terms of Service &rarr;
            </RouterLink>
          </nav>
        </header>

        <article class="prose prose-indigo max-w-none prose-headings:text-gray-900 prose-p:text-gray-600 prose-a:text-indigo-600 prose-li:text-gray-600">
          <h2>1. Introduction</h2>
          <p>
            Raven ("we", "our", or "us") is a SaaS knowledge management platform
            that helps organisations capture, organise, and retrieve information
            through intelligent search and AI-powered assistance. This Privacy
            Policy explains how we collect, use, disclose, and safeguard your
            personal data when you use our platform, website, and related
            services (collectively, the "Service").
          </p>
          <p>
            By accessing or using the Service, you agree to the collection and
            use of information in accordance with this policy. If you do not
            agree, please do not use the Service.
          </p>

          <h2>2. Data We Collect</h2>
          <h3>2.1 Information You Provide</h3>
          <ul>
            <li>
              <strong>Account information:</strong> name, email address,
              organisation name, and authentication credentials managed through
              our Keycloak identity provider.
            </li>
            <li>
              <strong>Content:</strong> documents, knowledge base entries,
              queries, chat messages, and other materials you upload or create
              within the Service.
            </li>
            <li>
              <strong>Communications:</strong> support requests, feedback, and
              correspondence you send to us.
            </li>
            <li>
              <strong>Payment information:</strong> billing details processed
              through our third-party payment provider. We do not store full
              payment card numbers on our servers.
            </li>
          </ul>

          <h3>2.2 Information Collected Automatically</h3>
          <ul>
            <li>
              <strong>Usage data:</strong> pages visited, features used, search
              queries, timestamps, and interaction patterns.
            </li>
            <li>
              <strong>Device and browser information:</strong> IP address,
              browser type and version, operating system, device identifiers, and
              screen resolution.
            </li>
            <li>
              <strong>Cookies and similar technologies:</strong> we use essential
              cookies for authentication and session management, and optional
              analytics cookies to improve the Service. See our cookie consent
              settings for more details.
            </li>
          </ul>

          <h2>3. How We Use Your Data</h2>
          <p>We process your personal data for the following purposes:</p>
          <ul>
            <li>
              Providing, operating, and maintaining the Service, including
              AI-powered search and retrieval features.
            </li>
            <li>
              Authenticating users and managing access controls within
              organisations and workspaces.
            </li>
            <li>
              Generating embeddings and semantic indexes from your content to
              power intelligent search (processed using our vector database
              infrastructure).
            </li>
            <li>
              Analysing usage patterns to improve performance, reliability, and
              user experience.
            </li>
            <li>
              Sending transactional communications such as account
              notifications, security alerts, and service updates.
            </li>
            <li>
              Complying with legal obligations and responding to lawful
              requests.
            </li>
          </ul>

          <h2>4. Data Sharing and Disclosure</h2>
          <p>
            We do not sell your personal data. We may share data in the
            following circumstances:
          </p>
          <ul>
            <li>
              <strong>Service providers:</strong> trusted third parties who
              assist in operating the Service (hosting, analytics, payment
              processing, email delivery). These providers are contractually
              bound to protect your data.
            </li>
            <li>
              <strong>Within your organisation:</strong> content and activity
              within a workspace is visible to other members of that workspace
              and organisation administrators.
            </li>
            <li>
              <strong>Legal requirements:</strong> when required by law, court
              order, or governmental regulation, or to protect the rights,
              safety, or property of Raven, our users, or the public.
            </li>
            <li>
              <strong>Business transfers:</strong> in connection with a merger,
              acquisition, or sale of assets, your data may be transferred to the
              acquiring entity.
            </li>
          </ul>

          <h2>5. Data Retention</h2>
          <p>
            We retain your personal data for as long as your account is active or
            as needed to provide the Service. Specific retention periods are:
          </p>
          <ul>
            <li>
              <strong>Account data:</strong> retained for the duration of your
              account, plus 30 days after deletion to allow recovery.
            </li>
            <li>
              <strong>Content and knowledge bases:</strong> retained until you or
              your organisation administrator deletes them. Deleted content is
              purged from backups within 90 days.
            </li>
            <li>
              <strong>Usage logs:</strong> retained for up to 12 months for
              analytics and security purposes.
            </li>
            <li>
              <strong>Legal hold data:</strong> retained as required by
              applicable law or regulation.
            </li>
          </ul>

          <h2>6. Your Rights</h2>
          <p>
            Depending on your jurisdiction, you may have the following rights
            regarding your personal data:
          </p>
          <ul>
            <li>
              <strong>Access:</strong> request a copy of the personal data we
              hold about you.
            </li>
            <li>
              <strong>Rectification:</strong> request correction of inaccurate
              or incomplete data.
            </li>
            <li>
              <strong>Erasure:</strong> request deletion of your personal data,
              subject to legal retention requirements.
            </li>
            <li>
              <strong>Data portability:</strong> receive your data in a
              structured, machine-readable format.
            </li>
            <li>
              <strong>Restriction:</strong> request that we restrict processing
              of your data in certain circumstances.
            </li>
            <li>
              <strong>Objection:</strong> object to processing based on
              legitimate interests or direct marketing.
            </li>
            <li>
              <strong>Withdraw consent:</strong> where processing is based on
              consent, you may withdraw it at any time.
            </li>
          </ul>
          <p>
            To exercise any of these rights, contact us at the address below. We
            will respond within 30 days.
          </p>

          <h2>7. Security</h2>
          <p>
            We implement industry-standard security measures to protect your
            data, including encryption in transit (TLS 1.3) and at rest
            (AES-256), role-based access controls, regular security audits, and
            SOC 2 compliance practices. However, no method of electronic
            transmission or storage is 100% secure, and we cannot guarantee
            absolute security.
          </p>

          <h2>8. International Transfers</h2>
          <p>
            Your data may be processed in countries other than your country of
            residence. When we transfer data internationally, we rely on
            appropriate safeguards such as Standard Contractual Clauses (SCCs) or
            adequacy decisions to ensure an equivalent level of protection.
          </p>

          <h2>9. Children's Privacy</h2>
          <p>
            The Service is not directed at individuals under the age of 16. We
            do not knowingly collect personal data from children. If you believe
            a child has provided us with personal data, please contact us and we
            will take steps to delete such information.
          </p>

          <h2>10. Changes to This Policy</h2>
          <p>
            We may update this Privacy Policy from time to time. We will notify
            you of material changes by posting the updated policy on this page
            and updating the "Last updated" date. Your continued use of the
            Service after changes constitutes acceptance of the revised policy.
          </p>

          <h2>11. Contact Us</h2>
          <p>
            If you have questions about this Privacy Policy or wish to exercise
            your data rights, please contact us:
          </p>
          <ul>
            <li>
              <strong>Email:</strong> privacy@ravenapp.dev
            </li>
            <li>
              <strong>Address:</strong> Raven Technologies, Data Protection
              Office
            </li>
          </ul>
        </article>

        <footer class="mt-10 border-t border-gray-200 pt-6">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <RouterLink
              to="/terms"
              class="text-sm font-medium text-indigo-600 transition-colors hover:text-indigo-800"
            >
              Terms of Service &rarr;
            </RouterLink>
            <RouterLink
              to="/login"
              class="text-sm text-gray-500 transition-colors hover:text-gray-700"
            >
              &larr; Back to Login
            </RouterLink>
          </div>
        </footer>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { RouterLink } from 'vue-router'
</script>
