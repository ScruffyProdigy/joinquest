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
          The king's manual Start stays available beside it for a table that is playable
          but not full — offered second, because a group on this screen is far more often
          waiting for a game than choosing to play short-handed.
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
          Any seated member may stop it, not only whoever started it: nobody on this
          screen can see who pressed the button, so a Stop that worked for one player and
          not another would be unexplainable from the outside.
        */}
        <Button type="button" variant="secondary" size="sm" disabled={busy} onClick={onCancelFindMatch}>
          {cta.cancelLabel}
        </Button>
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
