# Privacy Notice

> **STATUS:** DRAFT — pending review by qualified counsel. Do not rely on this
> document as legal advice. Until the DRAFT banner is removed by counsel, this
> notice must not be linked from public-facing surfaces or relied upon to
> satisfy any statutory disclosure obligation.

**Effective date:** 2026-05-08 *(placeholder — to be confirmed by counsel)*

This Privacy Notice explains how the Ravencloak organisation ("Ravencloak",
"we", "us") processes personal data in connection with the cloud-hosted
variant of the Raven platform. It is written to satisfy the transparency
obligations of the EU General Data Protection Regulation (GDPR) Articles 13
and 14, the UK GDPR, the Swiss Federal Act on Data Protection (FADP), and
India's Digital Personal Data Protection Act, 2023 ("DPDP Act").

## 1. Scope and Deployment Model

Raven is offered in two deployment modes, and the controller relationship
differs between them:

- **Self-hosted.** When a customer deploys Raven on their own infrastructure,
  that customer is the **data controller** (or "data fiduciary" under DPDP)
  for end-user personal data processed within their instance. Ravencloak does
  not have access to that data and is not a processor in respect of it. This
  notice does not govern self-hosted deployments; the operating customer must
  publish their own privacy notice.
- **Cloud-hosted (Ravencloak SaaS).** Where Ravencloak operates the platform
  on behalf of a customer, Ravencloak acts as **processor** (or "data
  processor" under DPDP) on the customer's behalf, save for a narrow set of
  controller activities (account administration, billing, security
  monitoring) covered by this notice.

## 2. Identity of the Controller

| Field                  | Value                                            |
| ---------------------- | ------------------------------------------------ |
| Controller             | Jobin Lawrance, trading as Ravencloak Org        |
| Place of establishment | India                                            |
| Postal address         | *(to be added by counsel)*                       |
| General contact        | <jobinlawrance@gmail.com>                        |

A formal entity registration and registered office will be added prior to
removal of the DRAFT banner.

## 3. Data Protection Officer / Grievance Officer

Pursuant to GDPR Article 37 (where applicable) and DPDP Act Sections 10 and
13, the following individual is designated as the contact for data protection
queries and grievances:

| Field   | Value                          |
| ------- | ------------------------------ |
| Name    | Jobin Lawrance                 |
| Role    | Data Protection / Grievance Officer |
| Email   | <jobinlawrance@gmail.com>      |

The DPO/Grievance Officer is the single point of contact for data subjects
and supervisory authorities. A dedicated alias (`privacy@ravencloak.org`)
will replace the personal address once provisioned.

## 4. Categories of Personal Data Processed

In the cloud-hosted variant, Ravencloak processes the following categories
of personal data:

- **Account and authentication identifiers** — email address, hashed
  credentials, SuperTokens session tokens, IP address, user agent, MFA
  enrolment metadata.
- **Conversation content** — text prompts, model responses, attached
  documents, retrieval-augmented generation (RAG) embeddings derived from
  customer content.
- **Voice audio** — short-form audio captured by the LiveKit voice agent
  feature, plus its derived transcripts and embeddings.
- **Telemetry and product analytics** — page views, feature usage events,
  error traces, and similar product analytics emitted by the
  `internal/posthog` package and stored in ClickHouse and PostHog.
- **Operational telemetry** — logs, traces, and metrics shipped to
  OpenObserve for incident response and performance management.
- **Billing data** — name, billing address, tax identifiers, payment-method
  metadata (full card numbers are not stored by Ravencloak).
- **Support correspondence** — anything a user voluntarily includes when
  contacting the team.

## 5. Lawful Bases for Processing (GDPR Art. 6)

| Processing activity                          | Lawful basis                                       |
| -------------------------------------------- | -------------------------------------------------- |
| Provision of the contracted service          | Performance of a contract — Art. 6(1)(b)           |
| Account security, fraud and abuse prevention | Legitimate interests — Art. 6(1)(f)                |
| Service telemetry and product improvement    | Legitimate interests — Art. 6(1)(f)                |
| Marketing communications                     | Consent — Art. 6(1)(a)                             |
| Voice recording beyond the default retention | Consent — Art. 6(1)(a)                             |
| Compliance with legal obligations            | Legal obligation — Art. 6(1)(c)                    |

For users in India, the corresponding lawful grounds under DPDP Act Sections
4 and 7 (consent, legitimate uses) are relied upon.

## 6. Special Categories of Personal Data (GDPR Art. 9)

Ravencloak does not solicit special-category data. Voice transcripts may
incidentally capture such data if a user volunteers it; the platform does
not infer health, biometric identification, sexual orientation, or political
opinions from voice signals. Where a customer chooses to process special
categories of data through the platform, the customer is responsible for
identifying an appropriate Article 9(2) condition.

## 7. Sub-Processors

The cloud-hosted variant relies on the following sub-processors. Each
sub-processor's privacy notice should be consulted for the data they process
on our behalf. Where the customer brings their own LLM key (BYOK), the
provider acts as a sub-processor of the customer, not of Ravencloak.

| Sub-processor    | Role                                           | Privacy policy           |
| ---------------- | ---------------------------------------------- | ------------------------ |
| AWS (incl. SES)  | Transactional email delivery (US transfer)     | *(URL placeholder)*      |
| PostgreSQL host  | Primary relational + pgvector store            | *(URL placeholder)*      |
| ClickHouse host  | Analytics and audit-log store                  | *(URL placeholder)*      |
| Valkey host      | Cache and asynchronous job queue (Asynq)       | *(URL placeholder)*      |
| SeaweedFS host   | Object storage (attachments, voice audio)      | *(URL placeholder)*      |
| SuperTokens      | Authentication and session management          | *(URL placeholder)*      |
| LiveKit          | Real-time voice transport for the voice agent  | *(URL placeholder)*      |
| PostHog          | Product analytics                              | *(URL placeholder)*      |
| Anthropic *(BYOK)* | LLM inference where the customer configures it | *(URL placeholder)*    |
| OpenAI *(BYOK)*    | LLM inference where the customer configures it | *(URL placeholder)*    |
| Cohere *(BYOK)*    | Embeddings and reranking where configured      | *(URL placeholder)*    |

The current authoritative list of sub-processors is mirrored in Annex III of
the Data Processing Addendum (`DPA.md`).

## 8. International Data Transfers

Personal data may be transferred outside the data subject's country of
residence — most notably to the United States in connection with AWS SES,
and potentially to other jurisdictions where BYOK LLM providers operate.
Where such transfers occur, Ravencloak relies on:

- The European Commission's **Standard Contractual Clauses** (Decision
  2021/914) for transfers from the EU/EEA;
- The **UK International Data Transfer Addendum** for transfers from the
  United Kingdom; and
- The **Swiss FADP addendum** for transfers from Switzerland.

A copy of the executed clauses is available on request to the DPO.

## 9. Retention

| Category                          | Default retention                                                  |
| --------------------------------- | ------------------------------------------------------------------ |
| Account and authentication data   | Lifetime of the account, plus 30 days after deletion request       |
| Conversation content              | Configurable per workspace; default retention set by the customer  |
| Voice audio                       | 30 days, unless the caller opts in to extended retention           |
| Voice transcripts and embeddings  | Tied to the parent conversation's retention setting                |
| Operational telemetry (logs)      | 90 days                                                            |
| Product analytics events          | 13 months                                                          |
| Billing records                   | 7 years (statutory accounting retention)                           |

Customers can shorten any of the above for their workspace via configuration.

## 10. Your Rights

Subject to applicable law, data subjects have the rights set out in GDPR
Articles 15–22 and DPDP Act Chapter III, namely:

- **Access** their personal data (Art. 15 / DPDP §11);
- **Rectify** inaccurate data (Art. 16 / DPDP §12);
- **Erase** their data (Art. 17 / DPDP §12);
- **Restrict** processing (Art. 18);
- **Portability** of data they provided (Art. 20);
- **Object** to processing based on legitimate interests (Art. 21);
- **Withdraw consent** at any time, without affecting prior lawful processing
  (Art. 7(3) / DPDP §6(4));
- **Lodge a complaint** with the competent supervisory authority — for EU
  residents, their national DPA (e.g. CNIL, ICO); for Indian residents, the
  Data Protection Board of India.

To exercise any right, open a private GitHub issue against
`ravencloak-org/Raven` or email the DPO at <jobinlawrance@gmail.com>.
Ravencloak responds to verifiable requests within **30 days**, extendable
once where strictly necessary under Article 12(3).

## 11. Children

The Raven platform is not directed at individuals under 18 years of age.
Ravencloak does not knowingly collect personal data from minors. Where a
parent or guardian believes their child has provided personal data, they
should contact the DPO so the data can be erased.

## 12. Security

Technical and organisational measures are described in `SECURITY.md` and
mirrored in Annex II of the Data Processing Addendum (`DPA.md`). Ravencloak
maintains SLSA Level 3 build provenance, OSPS Level 2 process compliance,
encryption at rest and in transit, role-based access control with row-level
security, and centralised audit logging.

## 13. Changes to This Notice

Material changes will be announced at least 30 days in advance via in-product
notice and via the project changelog. The "Effective date" at the top of
this notice will reflect the date of the most recent material revision.

## 14. Contact

| Purpose                                | Contact                                |
| -------------------------------------- | -------------------------------------- |
| Privacy queries / DPO / Grievance      | <jobinlawrance@gmail.com>              |
| Security vulnerability reports         | <security@ravencloak.org> (preferred), <jobinlawrance@gmail.com> (fallback) |
| Supervisory authority complaints (EU)  | Your national Data Protection Authority |
| Data Protection Board (India)          | <https://www.dpdp.gov.in/> *(placeholder)* |
