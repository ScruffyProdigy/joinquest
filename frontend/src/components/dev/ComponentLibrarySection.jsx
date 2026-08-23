import { useState } from 'react'
import { PRIMITIVE_GROUPS, COMPOSITE_PATTERNS, statusStyle } from '../../lib/componentLibrary'
import { accentColorFor } from '../../lib/gameAccent'

function StatusBadge({ status }) {
  const { label, badgeClass, dotClass } = statusStyle(status)
  return (
    <span
      className={`absolute top-3 right-3 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-2xs font-semibold ${badgeClass}`}
    >
      <span className={`w-1.5 h-1.5 rounded-full ${dotClass}`} />
      {label}
    </span>
  )
}

function UsageLines({ prototypeUse, productionUse }) {
  return (
    <div className="flex flex-col gap-1 mt-1.5">
      <div className="flex gap-1.5 items-baseline">
        <span className="font-mono-display text-2xs uppercase tracking-wide text-muted-foreground w-10 shrink-0">
          Proto
        </span>
        <span className="text-2xs text-muted-foreground">{prototypeUse}</span>
      </div>
      <div className="flex gap-1.5 items-baseline">
        <span className="font-mono-display text-2xs uppercase tracking-wide text-muted-foreground w-10 shrink-0">
          Live
        </span>
        <span className="text-2xs text-muted-foreground">{productionUse}</span>
      </div>
    </div>
  )
}

function PrimitivePreview({ previewKind, item }) {
  switch (previewKind) {
    case 'pill': {
      const pillClass =
        item.pillStyle === 'solid'
          ? 'bg-primary text-primary-foreground'
          : item.pillStyle === 'segment'
            ? 'bg-primary/10 text-primary border border-primary/30'
            : 'border border-border text-foreground'
      return <span className={`px-4 py-2 rounded-full text-xs font-semibold ${pillClass}`}>{item.name}</span>
    }
    case 'circle':
      return (
        <div className="w-10 h-10 rounded-full" style={{ background: accentColorFor('component-preview-avatar').badge }} />
      )
    case 'field':
      return (
        <div className="w-full h-9 rounded-lg bg-background border flex items-center px-2.5 gap-1.5">
          <svg
            viewBox="0 0 24 24"
            className="w-3.5 h-3.5 stroke-muted-foreground fill-none shrink-0"
            strokeWidth="1.8"
            strokeLinecap="round"
          >
            <circle cx="11" cy="11" r="7" />
            <line x1="21" y1="21" x2="16.65" y2="16.65" />
          </svg>
          <div className="flex-1 h-2 rounded bg-foreground/10" />
        </div>
      )
    case 'toggle':
      return item.toggleVariant === 'switch' ? (
        <div className="w-9 h-5 rounded-full bg-primary relative">
          <div className="absolute top-0.5 right-0.5 w-4 h-4 rounded-full bg-primary-foreground" />
        </div>
      ) : (
        <div className="w-4 h-4 rounded bg-primary" />
      )
    case 'panel':
      return (
        <div className="w-full rounded-lg bg-background border overflow-hidden">
          <div className="h-3.5 bg-foreground/5" />
          <div className="p-2 flex flex-col gap-1.5">
            <div className="h-1.5 rounded bg-foreground/10" />
            <div className="h-1.5 rounded bg-foreground/10 w-1/2" />
          </div>
        </div>
      )
    case 'tabsRow':
      return (
        <div className="flex gap-0.5 bg-background rounded-full p-1 w-full">
          {['Popular', 'Newest', 'A–Z'].map((label, index) => (
            <span
              key={label}
              className={`flex-1 text-center text-[9px] py-1 rounded-full ${
                index === 0 ? 'bg-card text-primary' : 'text-muted-foreground'
              }`}
            >
              {label}
            </span>
          ))}
        </div>
      )
    case 'barLine':
      return item.barVariant === 'progress' ? (
        <div className="w-full h-2 rounded-full bg-background overflow-hidden">
          <div className="h-full w-[62%] bg-primary" />
        </div>
      ) : (
        <div className="w-full h-px bg-border" />
      )
    default:
      return null
  }
}

function PrimitiveCard({ group, item }) {
  return (
    <div className="relative bg-card border rounded-2xl p-4 flex flex-col gap-3">
      <StatusBadge status={item.status} />
      <div className="h-[76px] rounded-xl bg-muted flex items-center justify-center p-3">
        <PrimitivePreview previewKind={group.previewKind} item={item} />
      </div>
      <div>
        <span className="font-sans font-semibold text-sm">{item.name}</span>
        <UsageLines prototypeUse={item.prototypeUse} productionUse={item.productionUse} />
      </div>
    </div>
  )
}

function CompositePreview({ previewKind }) {
  switch (previewKind) {
    case 'catalogCard':
      return (
        <div
          className="w-full h-full rounded-2xl p-3 flex flex-col justify-end gap-1"
          style={{ background: 'linear-gradient(160deg, #241b0f, #191305)', border: '1px solid rgba(245,158,11,0.25)' }}
        >
          <span className="self-start bg-black/40 text-[9px] px-2 py-1 rounded-full mb-auto">Trivia · Party</span>
          <span className="font-semibold text-sm">Trivia Blitz</span>
          <span className="text-2xs text-muted-foreground">112 playing</span>
        </div>
      )
    case 'hero':
      return (
        <div className="w-full h-full rounded-2xl flex items-end p-3" style={{ background: 'linear-gradient(135deg, #1e293b, #0f172a)' }}>
          <span className="font-heading font-bold text-base">Iron Fist Arena</span>
        </div>
      )
    case 'sheet':
      return (
        <div className="w-full h-full flex items-end">
          <div className="w-full bg-card border-t border-t-border rounded-t-2xl p-3 flex flex-col gap-1.5">
            <div className="w-8 h-1 rounded-full bg-border self-center" />
            <div className="h-2 rounded bg-foreground/10 w-2/5" />
            <div className="h-1.5 rounded bg-foreground/10 w-1/3" />
          </div>
        </div>
      )
    case 'avatarRow':
      return (
        <div className="flex items-center justify-center gap-2 w-full h-full">
          {['#f59e0b,#b45309', '#22c55e,#15803d', '#8b5cf6,#6d28d9', '#06b6d4,#0e7490'].map((stops, index) => (
            <div
              key={stops}
              className="w-8 h-8 rounded-full"
              style={{
                background: `linear-gradient(135deg, ${stops})`,
                boxShadow: index === 2 ? '0 0 0 2px var(--primary)' : undefined,
              }}
            />
          ))}
        </div>
      )
    case 'filterList':
      return (
        <div className="flex flex-col gap-2 justify-center w-full h-full px-3">
          <div className="flex items-center gap-2">
            <div className="w-3.5 h-3.5 rounded bg-primary" />
            <div className="h-1.5 rounded bg-foreground/10 flex-1" />
          </div>
          <div className="flex items-center gap-2">
            <div className="w-3.5 h-3.5 rounded border border-border" />
            <div className="h-1.5 rounded bg-foreground/10 w-3/4" />
          </div>
          <div className="flex items-center gap-2">
            <div className="w-3.5 h-3.5 rounded bg-primary" />
            <div className="h-1.5 rounded bg-foreground/10 w-4/5" />
          </div>
        </div>
      )
    default:
      return null
  }
}

function CompositeCard({ pattern }) {
  return (
    <div className="relative bg-card border rounded-2xl p-4 flex flex-col gap-3.5">
      <StatusBadge status={pattern.status} />
      <div className="h-[140px] rounded-xl bg-muted overflow-hidden">
        <CompositePreview previewKind={pattern.previewKind} />
      </div>
      <div>
        <span className="font-sans font-semibold text-sm">{pattern.name}</span>
        <UsageLines prototypeUse={pattern.prototypeUse} productionUse={pattern.productionUse} />
      </div>
    </div>
  )
}

export default function ComponentLibrarySection() {
  const [tab, setTab] = useState('primitives')

  return (
    <section className="flex flex-col gap-4">
      <h2 className="font-heading text-xl font-semibold">Component library</h2>
      <p className="font-sans text-sm text-muted-foreground">
        Every primitive and recurring pattern from the Figma prototype, tracked against what actually exists in
        production.
      </p>

      <div className="flex gap-1 bg-muted rounded-full p-1 w-fit">
        <button
          type="button"
          className={`px-4 py-2 rounded-full text-sm font-semibold ${
            tab === 'primitives' ? 'bg-background text-primary' : 'bg-transparent text-muted-foreground'
          }`}
          onClick={() => setTab('primitives')}
        >
          Primitives
        </button>
        <button
          type="button"
          className={`px-4 py-2 rounded-full text-sm font-semibold ${
            tab === 'composite' ? 'bg-background text-primary' : 'bg-transparent text-muted-foreground'
          }`}
          onClick={() => setTab('composite')}
        >
          Composite patterns
        </button>
      </div>

      {tab === 'primitives' ? (
        <div className="flex flex-col gap-7">
          {PRIMITIVE_GROUPS.map((group) => (
            <div key={group.title} className="flex flex-col gap-3">
              <span className="font-mono-display text-2xs uppercase tracking-wide text-muted-foreground">
                {group.title}
              </span>
              <div className="grid grid-cols-[repeat(auto-fill,minmax(212px,1fr))] gap-3.5">
                {group.items.map((item) => (
                  <PrimitiveCard key={item.name} group={group} item={item} />
                ))}
              </div>
            </div>
          ))}
        </div>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(280px,1fr))] gap-4">
          {COMPOSITE_PATTERNS.map((pattern) => (
            <CompositeCard key={pattern.name} pattern={pattern} />
          ))}
        </div>
      )}
    </section>
  )
}
