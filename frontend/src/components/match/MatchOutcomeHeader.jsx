import { MASCOT_LABEL, MATCH_MOOD, matchHeadline } from '../../lib/playerCopy'

/**
 * How each mood tints the mascot panel, read from the prototype's own `Jp`. It reaches for
 * `--success` and `--warning` where a result went well or nearly did, and falls back to the
 * neutral surface for one that did not — a loss is not an error state, so `--destructive`
 * would be the wrong borrow.
 */
const MOOD_PANEL = {
  [MATCH_MOOD.WIN]: {
    backgroundColor: 'var(--success-surface)',
    borderColor: 'color-mix(in srgb, var(--success) 20%, transparent)',
  },
  [MATCH_MOOD.LOSE]: {
    backgroundColor: 'var(--muted)',
    borderColor: 'var(--border)',
  },
  [MATCH_MOOD.CLOSE]: {
    backgroundColor: 'var(--warning-surface)',
    borderColor: 'color-mix(in srgb, var(--warning) 22%, transparent)',
  },
}

/**
 * The mascot slot. The prototype ships this as a placeholder and so does this — a dashed
 * panel carrying the label of the illustration that belongs in it. Deliberate: the art does
 * not exist yet, and a screen that quietly omits the slot loses the prototype's whole
 * vertical rhythm and hides the fact that something is missing (JQ-277).
 */
function MascotPanel({ mood }) {
  const panel = MOOD_PANEL[mood] ?? MOOD_PANEL[MATCH_MOOD.CLOSE]
  return (
    <div
      className="flex w-full items-center justify-center rounded-3xl border-2 border-dashed"
      style={{ height: 164, ...panel }}
      data-testid="mascot-panel"
      data-mood={mood}
    >
      <p className="text-2xs font-bold tracking-wide text-muted-foreground/40">
        {MASCOT_LABEL[mood] ?? MASCOT_LABEL[MATCH_MOOD.CLOSE]}
      </p>
    </div>
  )
}

/**
 * The top of the return screen: how it went, said large, over a mood panel.
 *
 * This is the screen's answer to "how did I do?", which is what the player came back for.
 * Production used to answer it in a small card in one state and not at all in the other; the
 * prototype answers it first, in both, and keys the whole page's tone off it.
 *
 * Renders nothing when the viewer is not on the roster — an unauthenticated or non-
 * participant read has no outcome to report, and an empty hero is worse than none.
 */
export default function MatchOutcomeHeader({ result, viewerId }) {
  const participants = result?.participants ?? []
  const viewer = participants.find((participant) => participant.user?.id === viewerId)
  if (!viewer) {
    return null
  }

  const { headline, sub, mood } = matchHeadline({
    reason: viewer.reason,
    complete: Boolean(result?.complete),
    placement: viewer.placement,
    playerCount: participants.length,
  })

  return (
    <div className="flex flex-col gap-5">
      <MascotPanel mood={mood} />
      <div>
        {/* The prototype ships 36px stepping to 44px. These are the two nearest rungs of our
            own scale (34px, 40px) — the same trade `buttonVariants` makes, so a restyle stays
            a change in tailwind.css rather than a sweep through components. */}
        <h1 className="font-heading text-2xl leading-tight font-bold text-foreground sm:text-3xl">
          {headline}
        </h1>
        <p className="mt-1 text-base text-muted-foreground">{sub}</p>
      </div>
    </div>
  )
}
