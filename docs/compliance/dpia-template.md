# Data Protection Impact Assessment (DPIA) — Template

> **STATUS:** DRAFT — pending review by qualified counsel. This template is
> intended as a working artefact for internal use; it must not be presented
> to a supervisory authority as a completed DPIA without legal review.

This document provides a template for conducting a Data Protection Impact
Assessment (DPIA) under GDPR Article 35, aligned with guidance from the UK
Information Commissioner's Office (ICO), the French CNIL, and the European
Data Protection Board's WP248 rev.01. Where the processing also falls
within the DPDP Act, 2023, this template should be supplemented with any
"Data Protection Impact Assessment" or "Significant Data Fiduciary"
obligations that the Indian rules introduce.

A completed DPIA should be filed in the DPIA register at
`docs/compliance/dpia-register.md`. The register file is intentionally not
created in this PR — it will be added in the next compliance round.

---

## When a DPIA Is Required

A DPIA is mandatory when processing is likely to result in a **high risk to
the rights and freedoms** of natural persons (GDPR Art. 35(1)). It is
**always** required in the cases listed in Article 35(3), including:

- Systematic and extensive evaluation based on automated processing,
  including profiling, that produces legal or similarly significant effects;
- Large-scale processing of special-category or criminal-conviction data;
- Systematic monitoring of a publicly accessible area on a large scale.

In addition, the **ICO's mandatory list** and **CNIL's "list of processing
operations subject to a DPIA"** include indicators such as: use of innovative
technology, biometric or genetic data, location tracking, processing of
data concerning vulnerable people, and combining datasets from different
sources.

If two or more WP248 criteria apply, a DPIA is presumptively required.
Document the determination either way.

---

## Step 1 — Identify the Need

| Field                                           | Value |
| ----------------------------------------------- | ----- |
| Processing activity                             |       |
| Owner (business)                                |       |
| Owner (engineering)                             |       |
| Date of assessment                              |       |
| DPIA reference                                  |       |
| Linked GitHub issue / RFC                       |       |
| Trigger (Art. 35(3) / WP248 / ICO / CNIL list)  |       |

Briefly explain what the processing involves and why a DPIA was triggered.

## Step 2 — Describe the Processing

Cover scope, context, and purpose:

- **Nature of processing:** what is collected, who has access, how long it
  is retained, where it is stored, how it flows. Include a **data-flow
  diagram** (placeholder — embed a diagram from the architecture
  documentation).
- **Scope:** volume and variety of data, geography, duration, frequency,
  number of data subjects.
- **Context:** relationship with data subjects, their reasonable
  expectations, prior public concerns, sectoral codes of practice.
- **Purpose:** the legitimate aim, expected benefits, and lawful basis.

## Step 3 — Consultation Process

| Stakeholder                | Consulted? | Date | Notes |
| -------------------------- | ---------- | ---- | ----- |
| DPO / Grievance Officer    |            |      |       |
| Engineering lead           |            |      |       |
| Security lead              |            |      |       |
| Product / business sponsor |            |      |       |
| External processor(s)      |            |      |       |
| Data subjects (sample)     |            |      |       |
| Supervisory authority      |            |      |       |

Where the views of data subjects cannot be sought, document the reason.

## Step 4 — Necessity and Proportionality

- Is the processing necessary to achieve the stated purpose? Could the
  purpose be achieved with less data or less intrusive means?
- What is the lawful basis under GDPR Art. 6, and any Art. 9 condition for
  special-category data?
- What is the lawful ground under DPDP Act §§ 4–7 for users in India?
- How is the data minimisation principle satisfied?
- What is the retention period and how is it enforced?
- How are data-subject rights operationalised?
- Are international transfers in scope, and which transfer mechanism is
  used (SCCs, UK IDTA, Swiss FADP addendum)?

## Step 5 — Identify and Assess Risks

For each identified risk, assess **likelihood** and **severity** on the
scales below, then plot in the matrix.

- **Likelihood:** Remote / Possible / Probable
- **Severity:** Minimal / Significant / Severe

| #   | Risk to data subjects                                 | Likelihood | Severity | Inherent rating |
| --- | ----------------------------------------------------- | ---------- | -------- | --------------- |
| R1  |                                                       |            |          |                 |
| R2  |                                                       |            |          |                 |
| R3  |                                                       |            |          |                 |

Risks to consider include: illegitimate access, unauthorised modification,
loss of data, re-identification, function creep, discrimination, loss of
control, chilling effects on free expression, and incidental capture of
special-category data.

### Likelihood × Severity Matrix

|              | Minimal | Significant | Severe |
| ------------ | ------- | ----------- | ------ |
| Remote       | Low     | Low         | Medium |
| Possible     | Low     | Medium      | High   |
| Probable     | Medium  | High        | High   |

## Step 6 — Identify Measures to Mitigate

For each risk in Step 5, identify mitigations and the resulting **residual
rating**.

| #   | Mitigation                                            | Effect on likelihood | Effect on severity | Residual rating | Owner |
| --- | ----------------------------------------------------- | -------------------- | ------------------ | --------------- | ----- |
| R1  |                                                       |                      |                    |                 |       |
| R2  |                                                       |                      |                    |                 |       |
| R3  |                                                       |                      |                    |                 |       |

If any residual risk remains **High**, **prior consultation with the
supervisory authority** under GDPR Article 36 is required before processing
begins.

## Step 7 — Sign-Off and Record Outcomes

| Role                          | Name | Date | Decision |
| ----------------------------- | ---- | ---- | -------- |
| DPO / Grievance Officer       |      |      |          |
| Engineering lead              |      |      |          |
| Product sponsor               |      |      |          |

Document any prior consultation submitted to a supervisory authority and
the response received. Record the planned review date — DPIAs are living
documents and must be revisited when the processing changes materially.

---

## Worked Example Reference — Voice Agent Feature

The Raven voice agent (LiveKit-backed) is the canonical worked example for
this template, because:

- **Voice is biometric-adjacent.** Even where the platform does not perform
  voice-print identification, the audio payload itself can support
  identification and is treated as high-risk.
- **AI inference on personal data.** Speech is transcribed, embedded, and
  fed into LLM inference, which is a form of automated processing producing
  effects on data subjects.
- **Incidental special-category data.** Callers may volunteer health,
  political, or other Article 9 data; the system must not treat that as
  expected input.
- **Cross-border transfers.** BYOK LLM providers may sit in the US.

A DPIA on this feature should explicitly address: retention defaults
(30 days, opt-in extension), erasure flows for both raw audio and derived
embeddings, the boundary between Ravencloak's processor role and
Customer's controller role, and prior consultation triggers if any
residual risk remains High.
