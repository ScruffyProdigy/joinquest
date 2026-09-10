import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { Share2Icon } from 'lucide-react'
import { accentBaseFor } from '../../lib/gameAccent'
import { roomShareText } from '../../lib/rooms'
import { Button } from '../ui/button'
import { Card, CardContent } from '../ui/card'

/** 112 CSS px at 3x, so the code stays crisp on a phone. */
const QR_PIXELS = 336

/**
 * The group view shares by QR and link. The invite code still exists on the room and
 * still works everywhere else — it is simply not the thing a player needs here.
 *
 * The QR is on the card, not behind a button (JQ-251). A friend standing next to you
 * scans it; a friend somewhere else gets the link. Two ways to invite, two controls —
 * Copy, QR, Share and Text were four ways to send the same URL.
 */
export default function GroupInviteCard({ room, game }) {
  const accent = accentBaseFor(game?.slug, game?.accentColor)
  const joinUrl = room?.joinUrl
  const [qrDataUrl, setQrDataUrl] = useState('')
  const [shareStatus, setShareStatus] = useState('')

  useEffect(() => {
    if (!joinUrl) {
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
  }, [joinUrl, accent])

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
          <p className="text-xs font-semibold uppercase tracking-wide text-muted-foreground">
            Invite friends
          </p>
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
        </CardContent>
      </Card>
    </section>
  )
}
