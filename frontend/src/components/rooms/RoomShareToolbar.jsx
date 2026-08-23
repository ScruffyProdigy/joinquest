import { useEffect, useState } from 'react'
import QRCode from 'qrcode'
import { roomShareText } from '../../lib/rooms'
import { IconCopy, IconQr, IconShare } from '../icons/ShareIcons'
import { Card, CardContent } from '../ui/card'
import { Button } from '../ui/button'
import { Dialog, DialogContent, DialogTitle, DialogDescription } from '../ui/dialog'

function IconSms(props) {
  return (
    <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true" {...props}>
      <path d="M21 11.5a8.38 8.38 0 0 1-.9 3.8 8.5 8.5 0 0 1-7.6 4.7 8.38 8.38 0 0 1-3.8-.9L3 21l1.9-5.7a8.38 8.38 0 0 1-.9-3.8 8.5 8.5 0 0 1 4.7-7.6 8.38 8.38 0 0 1 3.8-.9h.5a8.48 8.48 0 0 1 8 8v.5z" />
    </svg>
  )
}

export default function RoomShareToolbar({ joinUrl, inviteCode }) {
  const [qrOpen, setQrOpen] = useState(false)
  const [qrDataUrl, setQrDataUrl] = useState('')
  const [copyStatus, setCopyStatus] = useState('')

  useEffect(() => {
    if (!qrOpen || !joinUrl) {
      return undefined
    }
    let cancelled = false
    QRCode.toDataURL(joinUrl, { width: 240, margin: 2 })
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
  }, [qrOpen, joinUrl])

  async function copyLink() {
    const text = roomShareText(joinUrl)
    try {
      await navigator.clipboard.writeText(text)
      setCopyStatus('Copied!')
    } catch {
      setCopyStatus('Could not copy')
    }
    window.setTimeout(() => setCopyStatus(''), 2000)
  }

  async function nativeShare() {
    const text = roomShareText(joinUrl)
    if (navigator.share) {
      try {
        await navigator.share({ title: 'JoinQuest room', text, url: joinUrl })
      } catch {
        // user cancelled
      }
      return
    }
    await copyLink()
  }

  function openTextMessage() {
    const body = encodeURIComponent(roomShareText(joinUrl))
    window.location.href = `sms:?body=${body}`
  }

  return (
    <Card className="gap-3 py-4">
      <CardContent className="flex flex-col gap-3 px-4">
        <p className="text-sm text-muted-foreground">
          Room code: <strong className="font-mono-display text-foreground">{inviteCode}</strong>
        </p>
        <div className="flex flex-wrap gap-2" role="group" aria-label="Share room">
          <Button type="button" variant="secondary" size="sm" onClick={copyLink}>
            <IconCopy aria-hidden="true" />
            Copy
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={() => setQrOpen(true)}>
            <IconQr aria-hidden="true" />
            QR
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={nativeShare}>
            <IconShare aria-hidden="true" />
            Share
          </Button>
          <Button type="button" variant="secondary" size="sm" onClick={openTextMessage}>
            <IconSms aria-hidden="true" />
            Text
          </Button>
        </div>
        {copyStatus ? (
          <p className="text-xs text-muted-foreground" role="status">
            {copyStatus}
          </p>
        ) : null}
      </CardContent>

      <Dialog open={qrOpen} onOpenChange={setQrOpen}>
        <DialogContent>
          <DialogTitle>Scan to join</DialogTitle>
          <DialogDescription>Friends can open this link to join your room.</DialogDescription>
          {qrDataUrl ? (
            <img className="mx-auto h-60 w-60" src={qrDataUrl} alt={`QR code for ${joinUrl}`} />
          ) : (
            <p className="status-message" role="status">
              Generating QR…
            </p>
          )}
          <p className="break-all text-center text-xs text-muted-foreground">{joinUrl}</p>
        </DialogContent>
      </Dialog>
    </Card>
  )
}
