import { Container } from '@/components/Container'

const rows = [
  { label: 'CPU', min: '2 cores (x86-64 or ARM64)', recommended: '4+ cores' },
  { label: 'RAM', min: '4 GB', recommended: '8 GB+ (16 GB if running local LLMs)' },
  { label: 'Disk', min: '20 GB SSD', recommended: '100 GB+ for embeddings + objects' },
  { label: 'OS', min: 'Linux with Docker 24+', recommended: 'Ubuntu 24.04 LTS or Debian 13' },
  { label: 'Network', min: 'Outbound to your model provider', recommended: 'Same; or fully air-gapped with Ollama' },
]

export function SystemRequirements() {
  return (
    <section className="bg-white py-20 sm:py-28">
      <Container>
        <div className="mx-auto max-w-3xl">
          <h2 className="font-display text-3xl tracking-tight text-[var(--color-ink)]">
            System requirements
          </h2>
          <p className="mt-4 text-lg text-[var(--color-body)]">
            Raven runs comfortably on a modest VPS or a Raspberry Pi 5. The numbers
            below are guidance — the actual footprint depends on the embedding model
            and corpus size.
          </p>
          <table className="mt-10 w-full text-left text-sm">
            <thead className="border-b border-[var(--color-border)] text-[var(--color-ink)]">
              <tr>
                <th className="pb-3 font-display font-medium">Resource</th>
                <th className="pb-3 font-display font-medium">Minimum</th>
                <th className="pb-3 font-display font-medium">Recommended</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-[var(--color-border)]">
              {rows.map((r) => (
                <tr key={r.label}>
                  <td className="py-3 font-display text-[var(--color-ink)]">{r.label}</td>
                  <td className="py-3 text-[var(--color-body)]">{r.min}</td>
                  <td className="py-3 text-[var(--color-body)]">{r.recommended}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Container>
    </section>
  )
}
