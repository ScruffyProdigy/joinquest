import { Button } from '../ui/button'

/**
 * The sticky bottom control, driven entirely by `groupCtaState`. Every branch below is
 * one of its kinds — the bar decides nothing for itself, so what a player may do lives
 * in one testable place rather than in JSX.
 */
export default function GroupStartBar({ cta, busy, onStart, onFindMatch, onCancelFindMatch }) {
  if (cta.kind === 'find') {
    return (
      <div className="sticky bottom-0 mt-4 flex flex-col gap-2 border-t bg-background/95 px-4 py-4 text-center backdrop-blur">
        <Button type="button" disabled={busy} onClick={onFindMatch}>
          {cta.label}
        </Button>
        <p className="text-xs text-muted-foreground">{cta.hint}</p>
        {/*
          Playing short now, offered second: a group on this screen is far more often
          waiting for a full game than choosing to go without one.
        */}
        {cta.startLabel ? (
          <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={onStart}>
            {cta.startLabel}
          </Button>
        ) : null}
      </div>
    )
  }

  if (cta.kind === 'filling') {
    return (
      <div className="sticky bottom-0 mt-4 flex flex-col gap-2 border-t bg-background/95 px-4 py-4 text-center backdrop-blur">
        <p className="text-sm font-medium" role="status">
          {cta.label}
          {cta.detail ? <span className="font-normal text-muted-foreground"> · {cta.detail}</span> : null}
        </p>
        <p className="text-xs text-muted-foreground">{cta.hint}</p>
        {/*
          Absent rather than disabled for everyone else. A greyed-out Stop invites a
          player to wonder what they did wrong; nothing there says, correctly, that this
          is not theirs to do.
        */}
        {cta.cancelLabel ? (
          <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={onCancelFindMatch}>
            {cta.cancelLabel}
          </Button>
        ) : null}
      </div>
    )
  }

  return (
    <div className="sticky bottom-0 mt-4 border-t bg-background/95 px-4 py-4 text-center backdrop-blur">
      {cta.kind === 'start' ? (
        <Button type="button" disabled={busy} onClick={onStart}>
          {cta.label}
        </Button>
      ) : (
        <>
          <p className="text-sm font-medium" role="status">
            {cta.label}
          </p>
          {cta.hint ? <p className="text-xs text-muted-foreground">{cta.hint}</p> : null}
        </>
      )}
    </div>
  )
}
