import { useEffect, useId, useState } from 'react'
import QRCode from 'qrcode'
import { ChevronDownIcon, Share2Icon } from 'lucide-react'
import { accentBaseFor } from '../../lib/gameAccent'
import { roomShareText } from '../../lib/rooms'
import { Button } from '../ui/button'
import { Card, CardContent } from '../ui/card'
import { cn } from '../../lib/utils'

/** 112 CSS px at 3x, so the code stays crisp on a phone. */
const QR_PIXELS = 336

/**
 * The group view shares by QR and link. The invite code still exists on the room and
 * still works everywhere else — it is simply not the thing a player needs here.
 *
 * The QR is on the card, not behind a button (JQ-251). A friend standing next to you
 * scans it; a friend somewhere else gets the link. Two ways to invite, two controls —
 * Copy, QR, Share and Text were four ways to send the same URL.
 *
 * Which of the two jobs this screen leads with depends on how the player got here, so the
 * section collapses (JQ-305). Creating a room is asking for people to invite, and the
 * creator gets the QR open. Following a QR code or a room link is arriving, and an
 * arrival's first task is claiming a seat — so for them this stays a single row and the
 * Players card leads. `defaultOpen` is the whole of that distinction; it does not decide
 * whether the section exists, only how it starts.
 */
export default function GroupInviteCard({ room, game, defaultOpen = false }) {
  const accent = accentBaseFor(game?.slug, game?.accentColor)
  const joinUrl = room?.joinUrl
  const [qrDataUrl, setQrDataUrl] = useState('')
  const [shareStatus, setShareStatus] = useState('')
  /*
    null means "whatever the arrival says". The room loads a render or two after the page,
    so `defaultOpen` starts false and flips once the host resolves — reading through an
    override picks that up with no effect and no second source of truth, and a player who
    has already opened or closed the section outranks a default that arrives after them.
  */
  const [override, setOverride] = useState(null)
  const open = override ?? defaultOpen
  const contentId = useId()

  useEffect(() => {
    // Nothing to encode, or nobody looking: an arriving player pays for no QR at all.
    if (!joinUrl || !open) {
      setQrDataUrl('')
      return undefined
    }
    let cancelled = false
    QRCode.toDataURL(joinUrl, {
      width: QR_PIXELS,
      margin: 0,
      // Transparent behind the modules so the tile's own accent tint shows through.
      color: { dark: accent, light: '#00000000' },
    })
      .then((url) => {
        if (!cancelled) {
          setQrDataUrl(url)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setQrDataUrl('')
        }
      })
    return () => {
      cancelled = true
    }
  }, [joinUrl, accent, open])

  async function shareLink() {
    const text = roomShareText(joinUrl)
    if (navigator.share) {
      try {
        await navigator.share({ title: 'JoinQuest room', text, url: joinUrl })
      } catch {
        // user cancelled
      }
      return
    }
    try {
      await navigator.clipboard.writeText(text)
      setShareStatus('Link copied!')
    } catch {
      setShareStatus('Could not copy')
    }
    window.setTimeout(() => setShareStatus(''), 2000)
  }

  return (
    <section className="px-4 pt-4" aria-label="Invite friends">
      <Card className="gap-4 py-4">
        <CardContent className="flex flex-col gap-4 px-4">
          {/*
            The label is the control. Collapsed, the card is this row and nothing else, so
            the section keeps its place on the page whichever state it is in — opening it
            never pushes the seats out from under a thumb already reaching for them.
          */}
          <Button
            type="button"
            variant="ghost"
            size="sm"
            // Flush to the card's own padding and unfilled on hover, so it still reads as
            // the section's label rather than as a second control competing with Share.
            className="w-full justify-between px-0 text-xs font-semibold uppercase tracking-wide text-muted-foreground hover:bg-transparent hover:text-foreground"
            aria-expanded={open}
            aria-controls={open ? contentId : undefined}
            onClick={() => setOverride(!open)}
          >
            Invite friends
            <ChevronDownIcon
              aria-hidden="true"
              className={cn('transition-transform', open && 'rotate-180')}
            />
          </Button>
          {open ? (
            <div id={contentId} className="flex flex-col gap-4">
              <div className="flex items-center gap-4">
                <div
                  className="box-border flex size-28 shrink-0 items-center justify-center rounded-2xl p-2"
                  style={{ background: `color-mix(in oklab, ${accent} 14%, var(--card))` }}
                >
                  {qrDataUrl ? (
                    <img className="size-full" src={qrDataUrl} alt={`QR code to join ${joinUrl}`} />
                  ) : null}
                </div>
                <Button type="button" className="flex-1 py-2.5" onClick={shareLink}>
                  <Share2Icon aria-hidden="true" />
                  Share Link
                </Button>
              </div>
              {shareStatus ? (
                <p className="text-xs text-muted-foreground" role="status">
                  {shareStatus}
                </p>
              ) : null}
            </div>
          ) : null}
        </CardContent>
      </Card>
    </section>
  )
}
