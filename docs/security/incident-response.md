# Incident Response Runbook

> **STATUS:** DRAFT — pending review by qualified counsel. The notification
> templates, statutory timers, and regulatory addressees in this runbook
> must be confirmed by counsel before this document is relied upon during
> a live incident.

This runbook governs Ravencloak's response to security incidents affecting
the Raven platform, including personal-data breaches subject to GDPR
Article 33 and DPDP Act Section 8. It complements `SECURITY.md` (the public
disclosure path), `PRIVACY.md` (the controller-facing notice), `DPA.md`
(the contractual processor-to-controller flow), and `GOVERNANCE.md` (the
escalation chain).

## Scope

This runbook applies to:

- **Confidentiality, integrity, or availability incidents** affecting the
  cloud-hosted Raven platform or its supporting infrastructure;
- **Personal-data breaches** as defined in GDPR Article 4(12) and DPDP Act
  Section 2(1)(p);
- **Supply-chain compromises** affecting Raven's build, signing, or
  distribution pipeline;
- **Suspected insider misuse** of production systems or production data.

It does **not** govern self-hosted deployments operated by customers; in
those cases, the customer is the controller and runs their own runbook.

## Severity Classification

| Severity | Definition | Examples |
| -------- | ---------- | -------- |
| **SEV-1** | Confirmed personal-data breach with risk to rights/freedoms; major production outage; supply-chain or key compromise | Confirmed database exfiltration; signing key compromise; ransomware on production |
| **SEV-2** | Probable personal-data breach pending forensics; degraded (not down) service; credential misuse with possible production impact | Suspicious storage egress; suspected insider misuse; high-volume auth anomaly |
| **SEV-3** | Security event with no confirmed data exposure; isolated customer-impacting issue under investigation | Scorecard regression; unexploited deployed CVE; contained tenant misconfiguration |

The **on-call Incident Commander** sets the initial severity at T+0 and
revises it as evidence develops. Severity is recorded in the incident
ticket and never silently downgraded.

## Roles

| Role                     | Responsibility                                                      | Default holder                |
| ------------------------ | ------------------------------------------------------------------- | ----------------------------- |
| Incident Commander (IC)  | Overall coordination, severity calls, decisions of record           | On-call engineer              |
| Comms Lead               | Customer-facing communications, status page, regulator letters      | Founder / delegated comms     |
| Tech Lead                | Containment, forensic capture, remediation, post-incident review    | On-call engineer              |
| Legal / Privacy Lead     | Statutory notifications, supervisory-authority engagement, records  | Data Protection Officer       |

A single individual may temporarily hold more than one role at the early
stages of a small organisation; role separation is mandatory before any
external notification leaves the building.

## Detection Sources

- **OpenObserve alerts** — application traces, logs, security rules.
- **Beszel host metrics** — host-level anomalies on AWS hosts.
- **OSSF Scorecard regressions** — CI-driven supply-chain signal.
- **CodeQL / Semgrep / Gitleaks** — pre-merge and scheduled scans.
- **Third-party reports** — disclosures via the GitHub Security Advisory
  channel and the `security@ravencloak.org` alias defined in `SECURITY.md`.
- **Customer reports** — support correspondence escalated to the IC.
- **Sub-processor notifications** — breach notices received from any
  sub-processor listed in Annex III of `DPA.md`.

## Tools Reference

- **Beszel** — host monitoring/metrics system used for AWS host telemetry.
  Product documentation and dashboard context: https://beszel.dev

## Response Timeline

The clock starts at **T+0**, defined as the first moment a member of the
on-call rotation has actual or constructive awareness that an incident has
occurred. "Awareness" is interpreted in line with WP29 Guidelines on
personal-data breach notification (WP250 rev.01).

### T+0 → T+1 hour: Triage, Contain, Classify

1. Open an incident ticket and a private war-room channel.
2. Assign IC, Tech Lead, Comms Lead, and Privacy Lead.
3. Establish initial severity.
4. Take **immediate containment actions** that do not destroy evidence:
   rotate credentials, revoke sessions, isolate hosts, block egress.
5. Snapshot affected hosts and database state; preserve logs in
   write-locked storage.
6. Begin a single source-of-truth incident timeline; every action logged
   with timestamp and actor.

### T+1h → T+24h: Investigate, Preserve, Scope

1. Conduct evidence-led investigation: who, what, when, how, blast radius.
2. Determine whether **personal data** is implicated and, if so, the
   categories, volume, and affected jurisdictions.
3. Determine whether the incident is a "personal-data breach" requiring
   notification under GDPR Article 33, the DPDP Act, or any sectoral law.
4. Communicate ongoing status to internal stakeholders at least every four
   hours during a SEV-1.

### T+24h → T+48h: Processor → Controller Notice and Customer Communications

1. Where Ravencloak processes data on behalf of customers, deliver the
   **48-hour processor-to-controller notice** required by Section 7 of
   `DPA.md`. The notice covers, to the extent then known, the elements
   required by GDPR Article 33(3).
2. Send appropriate customer communications via the status page, in-product
   banner, and direct email where indicated.
3. Coordinate with the Privacy Lead on the wording of any external
   statement so that it is consistent with the regulator filing planned
   for the next phase.

### T+48h → T+72h: Statutory Notifications

> **REVIEW WITH COUNSEL — DRAFT.** The DPDP Act filing path, recipient,
> and prescribed notification form below are illustrative pending counsel
> sign-off. The exact route, addressee, and form may vary with the final
> DPDP rules and any sectoral guidance in force at the time of the
> incident. Do not rely on the language below in a live incident before
> counsel has confirmed the current path; treat it as a starting checklist
> only.

1. Where Ravencloak is acting as **controller** for the affected data, file
   a **GDPR Article 33** notification with the lead supervisory authority
   within **72 hours of awareness**, unless the breach is unlikely to result
   in a risk to the rights and freedoms of natural persons. Document the
   decision either way.
2. For data subjects in India, prepare a **DPDP Act § 8(6)** notification
   to the **Data Protection Board of India** and to each affected **data
   principal** — DPDP term equivalent to GDPR “data subject”. The exact
   recipient, prescribed form, and timing must be confirmed by counsel
   against the DPDP rules in force at the time of the incident before the
   notice is dispatched. *(See the REVIEW WITH COUNSEL note above.)*
3. Where the risk to data subjects is high, prepare and dispatch a
   **GDPR Article 34** communication to affected data subjects without
   undue delay.
4. Update sub-processors and downstream parties as required.

### T+72h → T+30d: Post-Incident Review

1. Complete a written **root-cause analysis (RCA)** with a five-whys or
   equivalent technique.
2. File **remediation tickets** with owners and due dates.
3. Conduct a **post-incident review** within ten business days of
   resolution; the review is blameless and produces written corrective
   actions.
4. Update the runbook, detection rules, and tabletop scenarios with
   lessons learned.
5. File the closed incident record in the incident archive.

## Notification Templates

### Regulator letter (GDPR Art. 33)

> Subject: Personal-Data Breach Notification — Ravencloak Org — \[Reference]
>
> Pursuant to Article 33 of Regulation (EU) 2016/679, Ravencloak Org
> notifies the \[supervisory authority] of a personal-data breach that came
> to our attention on \[date/time, UTC].
>
> 1. Nature of the breach: \[description, categories and approximate number
>    of data subjects, categories and approximate number of records].
> 2. Contact point: Data Protection Officer, \[name, email].
> 3. Likely consequences for data subjects: \[assessment].
> 4. Measures taken or proposed: \[containment, mitigation, communication].
>
> Where information is not yet available, it will be provided in phases
> without further undue delay.

### Customer email (processor → controller, per DPA Section 7)

> Subject: \[SEV-?] Personal-Data Breach Notification — \[Reference]
>
> Dear \[Customer],
>
> Ravencloak Org is writing to notify you, in our capacity as processor
> under our Data Processing Addendum, of a personal-data breach affecting
> your tenant of the Raven platform. Awareness occurred at \[time, UTC].
>
> What we know now: \[summary]. What we do not yet know: \[summary].
> Containment actions taken: \[summary]. Next update: \[time].
>
> Please direct any questions to \[DPO contact]. We will continue to update
> you as our investigation progresses.

## Tools and Runbook Commands

> The exact queries below are illustrative. Validate against the live
> schema and adjust before running in production.

- **ClickHouse audit-log triage**

  ```sql
  -- ClickHouse SQL dialect
  -- Set these per incident scope:
  --   {LOOKBACK_HOURS:Int32} (for example: 1, 24, 72, 168)
  --   {ROW_LIMIT:Int32} (for example: 1000, 10000, 50000)
  SELECT timestamp, actor_id, action, resource, source_ip
  FROM audit_events
  WHERE timestamp >= now() - INTERVAL {LOOKBACK_HOURS:Int32} HOUR
    AND (action LIKE '%export%' OR action LIKE '%delete%')
  ORDER BY timestamp DESC
  LIMIT {ROW_LIMIT:Int32};
  ```

- **OpenObserve correlation** — pivot on the `incident_id` tag attached to
  all telemetry generated during a war-room session; export the full trace
  bundle to write-locked storage.

- **Container forensics** — capture the filesystem and process list of any
  container of interest before terminating it; tag the snapshot with the
  incident reference and chain-of-custody metadata.

- **Credential rotation** — rotate database, object-storage, and signing
  credentials per the runbook in `SECURITY.md`. Revoke SuperTokens sessions
  for affected tenants where session-token compromise is suspected.

## Tabletop Exercise Cadence

Tabletop exercises are conducted **quarterly**. Each exercise rehearses one
of: a SEV-1 personal-data breach, a SEV-1 supply-chain compromise, a SEV-2
insider misuse case, or a sub-processor-originated breach. The Privacy Lead
participates in every exercise. Exercise outputs are filed alongside live
incidents in the archive and are an input to the annual review of this
runbook.

## Related Documents

- `SECURITY.md` — public disclosure path and supported versions.
- `GOVERNANCE.md` — escalation, decision rights, and project governance.
- `PRIVACY.md` — the public privacy notice referenced in customer comms.
- `DPA.md` — the processor-to-controller breach notification clause and
  sub-processor list.
- `docs/compliance/dpia-template.md` — the DPIA template used to evaluate
  changes before they reach production.
