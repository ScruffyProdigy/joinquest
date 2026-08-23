export const PRIMITIVE_GROUPS = [
  {
    title: 'Actions & selection',
    previewKind: 'pill',
    items: [
      {
        name: 'Button',
        pillStyle: 'solid',
        status: 'ported',
        prototypeUse: 'Every action — filter chips, quick play, form submits',
        productionUse: 'Ported (JQ-46) — 5 cva variants',
      },
      {
        name: 'Badge',
        pillStyle: 'outline',
        status: 'ported',
        prototypeUse: 'Genre/mode tag pills, player-count badge on catalog cards',
        productionUse: 'Ported (JQ-52) — table card tags',
      },
      {
        name: 'Toggle',
        pillStyle: 'segment',
        status: 'not-started',
        prototypeUse: 'Not directly observed — standard shadcn single-toggle',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Avatars',
    previewKind: 'circle',
    items: [
      {
        name: 'Avatar',
        status: 'not-started',
        prototypeUse: 'Guest name picker, profile header, spirit-picker grid',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Form fields',
    previewKind: 'field',
    items: [
      {
        name: 'Input',
        status: 'not-started',
        prototypeUse: 'Search-games field on Home',
        productionUse: 'Not yet',
      },
      {
        name: 'Select',
        status: 'not-started',
        prototypeUse: 'Not directly observed — standard shadcn field',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Switches & checks',
    previewKind: 'toggle',
    items: [
      {
        name: 'Switch',
        toggleVariant: 'switch',
        status: 'not-started',
        prototypeUse: 'Not directly observed',
        productionUse: 'Not yet — token scaffolded (JQ-46)',
      },
      {
        name: 'Checkbox',
        toggleVariant: 'checkbox',
        status: 'not-started',
        prototypeUse: 'Filter panel: player-count + category checklists',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Surfaces & overlays',
    previewKind: 'panel',
    items: [
      {
        name: 'Card',
        status: 'ported',
        prototypeUse: 'Foundation under every composite pattern below',
        productionUse: 'Ported (JQ-52) — room panel, table card, share toolbar',
      },
      {
        name: 'Dialog',
        status: 'ported',
        prototypeUse: '"Own your handle" account-creation prompt',
        productionUse: 'Ported (JQ-52) — room QR-code modal',
      },
      {
        name: 'Sheet',
        status: 'ported',
        prototypeUse: 'Bottom-sheet: filters, game modes, account panel',
        productionUse: 'Ported (JQ-52) — room drawer, seat picker; filters/account panel still not started',
      },
      {
        name: 'Tooltip',
        status: 'not-started',
        prototypeUse: 'Not directly observed',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Navigation',
    previewKind: 'tabsRow',
    items: [
      {
        name: 'Tabs',
        status: 'not-started',
        prototypeUse: 'Popular / Fastest / Newest / A–Z sort row',
        productionUse: 'Not yet',
      },
    ],
  },
  {
    title: 'Feedback',
    previewKind: 'barLine',
    items: [
      {
        name: 'Progress',
        barVariant: 'progress',
        status: 'not-started',
        prototypeUse: "Locked-mode unlock progress ('Available in 47 days')",
        productionUse: 'Not yet',
      },
      {
        name: 'Separator',
        barVariant: 'separator',
        status: 'not-started',
        prototypeUse: 'Hairline divider between sheet sections',
        productionUse: 'Not yet',
      },
    ],
  },
]

export const COMPOSITE_PATTERNS = [
  {
    name: 'Genre-accent catalog card',
    previewKind: 'catalogCard',
    status: 'ported',
    prototypeUse: 'Home → All games grid, every card',
    productionUse: 'GameCard.jsx — home catalog grid (JQ-48)',
  },
  {
    name: 'Game detail hero',
    previewKind: 'hero',
    status: 'not-started',
    prototypeUse: 'Tap any catalog card → full-bleed hero header',
    productionUse: 'Not started — JQ-49',
  },
  {
    name: 'Bottom-sheet drawer',
    previewKind: 'sheet',
    status: 'not-started',
    prototypeUse: 'Filters, game modes, account panel — same shape, 3 places',
    productionUse: 'Not started',
  },
  {
    name: 'Avatar / spirit picker grid',
    previewKind: 'avatarRow',
    status: 'legacy',
    prototypeUse: 'Guest name selection on the Welcome screen',
    productionUse: 'Legacy — SpiritAnimalFlow.jsx, hand-written CSS',
  },
  {
    name: 'Filter panel',
    previewKind: 'filterList',
    status: 'not-started',
    prototypeUse: 'Home → Filter → player-count & category checklists',
    productionUse: 'Not started',
  },
]

/** @param {string} status @returns {{label: string, badgeClass: string, dotClass: string}} */
export function statusStyle(status) {
  if (status === 'ported') {
    return { label: 'Ported', badgeClass: 'bg-primary/10 text-primary', dotClass: 'bg-primary' }
  }
  if (status === 'legacy') {
    return { label: 'Legacy CSS', badgeClass: 'bg-amber-500/10 text-amber-500', dotClass: 'bg-amber-500' }
  }
  return { label: 'Not started', badgeClass: 'bg-foreground/5 text-muted-foreground', dotClass: 'bg-muted-foreground' }
}
