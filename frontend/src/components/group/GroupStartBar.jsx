import { Button } from '../ui/button'

export default function GroupStartBar({ cta, busy, onStart }) {
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
