<template>
  <div class="min-h-screen bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-900">
    <div class="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8">
      <div class="rounded-2xl bg-white p-8 shadow-2xl sm:p-12">
        <header class="mb-10 border-b border-gray-200 pb-8">
          <h1 class="text-3xl font-bold tracking-tight text-gray-900">
            Terms of Service
          </h1>
          <p class="mt-2 text-sm text-gray-500">
            Last updated: March 2026
          </p>
          <nav class="mt-4">
            <RouterLink
              to="/privacy"
              class="text-sm font-medium text-indigo-600 transition-colors hover:text-indigo-800"
            >
              View Privacy Policy &rarr;
            </RouterLink>
          </nav>
        </header>

        <article class="prose prose-indigo max-w-none prose-headings:text-gray-900 prose-p:text-gray-600 prose-a:text-indigo-600 prose-li:text-gray-600">
          <h2>1. Acceptance of Terms</h2>
          <p>
            By accessing or using the Raven platform ("Service"), operated by
            Raven Technologies ("we", "our", or "us"), you agree to be bound by
            these Terms of Service ("Terms"). If you are using the Service on
            behalf of an organisation, you represent and warrant that you have
            authority to bind that organisation to these Terms.
          </p>
          <p>
            If you do not agree to these Terms, you must not access or use the
            Service. We reserve the right to modify these Terms at any time. Your
            continued use after changes constitutes acceptance of the modified
            Terms.
          </p>

          <h2>2. Account Registration and Security</h2>
          <p>
            To use the Service, you must create an account through our Keycloak
            identity provider. You agree to:
          </p>
          <ul>
            <li>
              Provide accurate, current, and complete registration information.
            </li>
            <li>
              Maintain the security of your account credentials and not share
              them with third parties.
            </li>
            <li>
              Notify us immediately of any unauthorised access to or use of your
              account.
            </li>
            <li>
              Accept responsibility for all activities that occur under your
              account.
            </li>
          </ul>
          <p>
            We reserve the right to suspend or terminate accounts that violate
            these Terms or that we reasonably believe are being used
            fraudulently.
          </p>

          <h2>3. Permitted Use</h2>
          <p>
            The Service is a knowledge management platform designed for
            organisations to capture, organise, and retrieve information. You
            agree to use the Service only for lawful purposes and in accordance
            with these Terms. Specifically, you shall not:
          </p>
          <ul>
            <li>
              Use the Service to store, transmit, or distribute any content that
              is unlawful, harmful, threatening, abusive, defamatory, or
              otherwise objectionable.
            </li>
            <li>
              Attempt to gain unauthorised access to any part of the Service,
              other accounts, or systems connected to the Service.
            </li>
            <li>
              Reverse engineer, decompile, or disassemble any portion of the
              Service.
            </li>
            <li>
              Use automated means (bots, scrapers, crawlers) to access the
              Service except through our published APIs.
            </li>
            <li>
              Interfere with or disrupt the integrity or performance of the
              Service or its underlying infrastructure.
            </li>
            <li>
              Resell, sublicence, or redistribute the Service without our prior
              written consent.
            </li>
          </ul>

          <h2>4. Content and Intellectual Property</h2>
          <h3>4.1 Your Content</h3>
          <p>
            You retain all ownership rights in the content you upload, create, or
            store within the Service ("Your Content"). By using the Service, you
            grant us a limited, non-exclusive licence to process, store, index,
            and display Your Content solely for the purpose of providing and
            improving the Service, including generating embeddings and semantic
            search indexes.
          </p>
          <h3>4.2 Our Intellectual Property</h3>
          <p>
            The Service, including its software, design, logos, documentation,
            and all associated intellectual property, is owned by Raven
            Technologies and protected by copyright, trademark, and other
            intellectual property laws. These Terms do not grant you any right to
            use our trademarks, logos, or brand features without prior written
            permission.
          </p>
          <h3>4.3 Feedback</h3>
          <p>
            If you provide feedback, suggestions, or ideas about the Service, we
            may use them without restriction or obligation to you.
          </p>

          <h2>5. Service Availability and Modifications</h2>
          <p>
            We strive to maintain high availability of the Service but do not
            guarantee uninterrupted or error-free operation. We reserve the right
            to:
          </p>
          <ul>
            <li>
              Modify, suspend, or discontinue any part of the Service at any
              time, with or without notice.
            </li>
            <li>
              Perform scheduled maintenance, during which the Service may be
              temporarily unavailable.
            </li>
            <li>
              Update features, interfaces, and APIs, which may require you to
              update your integrations.
            </li>
          </ul>

          <h2>6. Limitation of Liability</h2>
          <p>
            To the maximum extent permitted by applicable law:
          </p>
          <ul>
            <li>
              The Service is provided "as is" and "as available" without
              warranties of any kind, either express or implied, including but
              not limited to implied warranties of merchantability, fitness for a
              particular purpose, and non-infringement.
            </li>
            <li>
              We shall not be liable for any indirect, incidental, special,
              consequential, or punitive damages, including loss of profits,
              data, or business opportunities, arising out of or relating to your
              use of the Service.
            </li>
            <li>
              Our total aggregate liability for any claims arising from or
              relating to the Service shall not exceed the amounts paid by you to
              us in the twelve (12) months preceding the claim.
            </li>
          </ul>

          <h2>7. Indemnification</h2>
          <p>
            You agree to indemnify, defend, and hold harmless Raven Technologies
            and its officers, directors, employees, and agents from and against
            any claims, liabilities, damages, losses, and expenses (including
            reasonable legal fees) arising from:
          </p>
          <ul>
            <li>Your use of the Service.</li>
            <li>Your violation of these Terms.</li>
            <li>
              Your Content or any content you make available through the Service.
            </li>
            <li>Your violation of any rights of a third party.</li>
          </ul>

          <h2>8. Termination</h2>
          <p>
            Either party may terminate the use of the Service at any time:
          </p>
          <ul>
            <li>
              <strong>By you:</strong> you may close your account at any time
              through your account settings or by contacting support.
            </li>
            <li>
              <strong>By us:</strong> we may suspend or terminate your access
              immediately if you breach these Terms, if required by law, or if we
              discontinue the Service.
            </li>
          </ul>
          <p>
            Upon termination, your right to use the Service ceases immediately.
            We will make your data available for export for 30 days following
            termination, after which it may be permanently deleted.
          </p>

          <h2>9. Governing Law and Dispute Resolution</h2>
          <p>
            These Terms shall be governed by and construed in accordance with the
            laws of the jurisdiction in which Raven Technologies is
            incorporated, without regard to its conflict of law provisions.
          </p>
          <p>
            Any disputes arising from these Terms or the Service shall be
            resolved through binding arbitration in accordance with the rules of
            the applicable arbitration body in our jurisdiction. Before
            initiating arbitration, you agree to attempt to resolve the dispute
            informally by contacting us.
          </p>

          <h2>10. General Provisions</h2>
          <ul>
            <li>
              <strong>Severability:</strong> if any provision of these Terms is
              found to be unenforceable, the remaining provisions shall continue
              in full force and effect.
            </li>
            <li>
              <strong>Waiver:</strong> failure to enforce any provision of these
              Terms shall not constitute a waiver of that provision.
            </li>
            <li>
              <strong>Entire agreement:</strong> these Terms, together with our
              Privacy Policy, constitute the entire agreement between you and
              Raven Technologies regarding the Service.
            </li>
            <li>
              <strong>Assignment:</strong> you may not assign or transfer these
              Terms without our prior written consent. We may assign these Terms
              freely.
            </li>
          </ul>

          <h2>11. Contact Us</h2>
          <p>
            If you have questions about these Terms of Service, please contact
            us:
          </p>
          <ul>
            <li>
              <strong>Email:</strong> legal@ravenapp.dev
            </li>
            <li>
              <strong>Address:</strong> Raven Technologies, Legal Department
            </li>
          </ul>
        </article>

        <footer class="mt-10 border-t border-gray-200 pt-6">
          <div class="flex flex-wrap items-center justify-between gap-4">
            <RouterLink
              to="/privacy"
              class="text-sm font-medium text-indigo-600 transition-colors hover:text-indigo-800"
            >
              Privacy Policy &rarr;
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
