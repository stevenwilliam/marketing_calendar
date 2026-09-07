import { brandColor } from '../lib/format'
import type { Company } from '../lib/api'

/**
 * Brand tabs.
 *
 * The same shape the reports and settings screens use, so the product reads as
 * one system rather than three people's ideas of a tab. Two things it does
 * that a plain tab strip does not:
 *
 *  - the active tab carries the BRAND's colour, not the generic accent, so the
 *    page tells you which brand you are in even after you have scrolled past
 *    the tabs;
 *  - the count is per tab and honours the search and filters in force, so
 *    "Ruuma 0" means "no Ruuma plans match what you typed", not "no Ruuma
 *    plans exist". A count that ignores the filters is worse than no count.
 *
 * Tabs are hidden entirely when the user can see only one brand: a tab strip
 * with one tab is furniture.
 */
export function BrandTabs({
  companies, value, onChange, counts, loading,
}: {
  companies: Company[]
  value: string                       // '' means every brand
  onChange: (companyID: string) => void
  counts?: Record<string, number>     // keyed by company_id, '' for all
  loading?: boolean
}) {
  if (companies.length <= 1) return null

  const tabs = [{ id: '', name: 'Semua merek', code: '' }, ...companies.map((c) => ({
    id: c.company_id, name: c.company_name, code: c.company_code,
  }))]

  return (
    <div
      role="tablist"
      aria-label="Saring menurut merek"
      // Scrolls in its own strip rather than pushing the page sideways at
      // 360px, where three brands plus "Semua" do not fit.
      className="flex overflow-x-auto border-b-2 border-divider -mb-0.5"
    >
      {tabs.map((t) => {
        const active = value === t.id
        const colour = t.id === '' ? '#ae1800' : brandColor(t.code)
        const n = counts?.[t.id]
        return (
          <button
            key={t.id || 'all'}
            role="tab"
            aria-selected={active}
            onClick={() => onChange(t.id)}
            className="relative whitespace-nowrap px-4 py-2.5 text-sm font-semibold"
            style={{
              color: active ? colour : 'rgba(32,30,29,0.65)',
              // A 3px underline in the brand's own colour. Colour is never the
              // only signal: the label is also the brand's name, and the
              // active tab is bolder.
              boxShadow: active ? `inset 0 -3px 0 0 ${colour}` : undefined,
            }}
          >
            {t.name}
            {n !== undefined && (
              <span className="ml-1.5 tnum text-xs" style={{ opacity: loading ? 0.4 : 0.7 }}>
                {loading ? '·' : n}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
