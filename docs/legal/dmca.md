# DMCA Policy

**Designated agent:** [dmca@ravencloak.org](mailto:dmca@ravencloak.org)

## Plain-language summary

If you own the copyright to material that has been published on the
Raven Marketplace without your permission, email
**dmca@ravencloak.org** with a description of the work and where it
appears on Raven. We will review the notice, take down the offending
material within a reasonable time, and notify the publisher. The
publisher has 14 days to file a counter-notice; if they do, you'll be
notified and have the opportunity to seek a court order. Filing a
false notice may make you liable for damages.

This page is the public-facing summary. The operational procedure for
Raven admins lives in [`docs/runbooks/marketplace-takedown.md`](../runbooks/marketplace-takedown.md).

## Submitting a takedown notice (17 U.S.C. § 512(c)(3))

Send a written notice to **dmca@ravencloak.org** that includes **all
six** of the following — incomplete notices cannot be acted on:

1. A physical or electronic signature of the copyright owner (or an
   agent authorised to act on their behalf).
2. Identification of the copyrighted work claimed to have been
   infringed (or, for multiple works at a single Raven URL, a
   representative list).
3. Identification of the material claimed to be infringing — provide
   the Raven Marketplace URL (e.g. `https://raven.ravencloak.org/marketplace/kb/{kb_id}`)
   and enough detail to let us locate it.
4. Contact information: name, address, phone number, and email of the
   notifying party.
5. A statement that the notifying party has a good-faith belief that
   use of the material is not authorised by the copyright owner, its
   agent, or the law.
6. A statement that the information in the notice is accurate and,
   under penalty of perjury, that the notifying party is authorised
   to act on behalf of the copyright owner.

Notices missing any of these elements will receive a single reply
asking for the missing information; we cannot otherwise act.

## What happens after we receive a valid notice

1. **Triage:** within 48 hours, Trust & Safety acknowledges receipt
   and opens an internal record (`dmca_notices` row + `/admin/marketplace/dmca` queue).
2. **Takedown:** the targeted knowledge base transitions to
   `kb_status='dmca_pending'` and is removed from the public catalog.
3. **Publisher notification:** the publisher is emailed with a copy of
   the notice and informed of the 14-day counter-notice window.
4. **Counter-notice window (14 days):**
   - If no counter-notice is received, the KB is permanently set to
     `visibility='private'` and a `marketplace_takedowns` row is
     written with `source='dmca'`. Per ADR-0006, this also increments
     the publisher Org's `takedown_strikes` counter.
   - If a counter-notice is received, see *Counter-notice process* below.
5. **Notifying party update:** when the KB is taken down (or the
   counter-notice window opens), we email the original notifier.

## Counter-notice process

If you are a Raven publisher whose knowledge base has been removed in
response to a DMCA notice and you believe the removal was in error
(e.g. the material is yours, properly licensed, or fair use), you may
file a counter-notice by emailing **dmca@ravencloak.org** within 14
days of receiving our takedown email.

A valid counter-notice must include all of the following per
17 U.S.C. § 512(g)(3):

1. Your physical or electronic signature.
2. Identification of the material that was removed and the location
   from which it was removed (the original Raven Marketplace URL).
3. A statement under penalty of perjury that you have a good-faith
   belief the material was removed as a result of mistake or
   misidentification.
4. Your name, address, phone number, and a statement that you consent
   to the jurisdiction of the federal district court for the judicial
   district in which your address is located (or, if outside the
   United States, the United States District Court for the District
   of Delaware), and that you will accept service of process from the
   notifying party or their agent.

On receipt of a valid counter-notice:

- We forward the counter-notice to the original notifying party.
- We restore the knowledge base no sooner than 10 and no later than
  14 business days from receipt of the counter-notice — **unless** the
  notifying party informs us that they have filed an action seeking a
  court order to restrain the publisher from infringing.

## Repeat-infringer policy

Per our [Marketplace Acceptable Use](https://raven.ravencloak.org/legal/acceptable-use)
and [ADR-0006](../adr/0006-licence-and-moderation.md), an organization
that accrues **three** confirmed takedown strikes (whether from admin
review or DMCA notices) is reviewed for suspension. The operational
detail of how this review is conducted is documented in
[`docs/runbooks/marketplace-repeat-infringer.md`](../runbooks/marketplace-repeat-infringer.md).

A "confirmed strike" means a `marketplace_takedowns` row was written
with `source='admin'` or `source='dmca'` and the resulting KB
transition was not reversed by a successful counter-notice or admin
appeal.

## False notices

Knowingly material misrepresentation in a DMCA notice or counter-notice
is sanctionable under 17 U.S.C. § 512(f). Repeat false notices from the
same notifying party will be ignored after written warning.

## References

- Statute — [17 U.S.C. § 512](https://www.copyright.gov/title17/92chap5.html#512)
- Internal procedure — [`docs/runbooks/marketplace-takedown.md`](../runbooks/marketplace-takedown.md)
- Repeat-infringer suspension — [`docs/runbooks/marketplace-repeat-infringer.md`](../runbooks/marketplace-repeat-infringer.md)
- ADR-0006 — [`docs/adr/0006-licence-and-moderation.md`](../adr/0006-licence-and-moderation.md)
