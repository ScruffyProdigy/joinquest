import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import QRCode from 'qrcode'
import GroupInviteCard from './GroupInviteCard'

vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,stub') },
}))

const room = { joinUrl: 'https://joinquest.cc/room/ABC123', inviteCode: 'ABC123' }
const game = { slug: 'word-hunt', accentColor: '#f2b134' }

/** The creator's view: the share section is what they came here for, so it starts open. */
function renderOpen(props = {}) {
  return render(<GroupInviteCard room={room} game={game} defaultOpen {...props} />)
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('GroupInviteCard', () => {
  it('shows the QR code straight away, with nothing to click first', async () => {
    renderOpen()

    const qr = await screen.findByRole('img', { name: /qr code to join/i })
    expect(qr).toHaveAttribute('src', 'data:image/png;base64,stub')
    expect(screen.queryByRole('button', { name: 'QR' })).not.toBeInTheDocument()
  })

  it('offers Share Link and nothing else', async () => {
    renderOpen()
    await screen.findByRole('img', { name: /qr code to join/i })

    const shareControls = screen
      .getAllByRole('button')
      .filter((button) => button.getAttribute('aria-expanded') === null)
    expect(shareControls).toHaveLength(1)
    expect(shareControls[0]).toHaveAccessibleName('Share Link')
  })

  it('copies the invite link when the browser cannot share', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText)
    renderOpen()

    await user.click(screen.getByRole('button', { name: 'Share Link' }))

    expect(writeText).toHaveBeenCalledWith(expect.stringContaining(room.joinUrl))
    expect(await screen.findByText('Link copied!')).toBeInTheDocument()
  })

  it('hands the link to the native share sheet when there is one', async () => {
    const user = userEvent.setup()
    const share = vi.fn().mockResolvedValue(undefined)
    navigator.share = share
    renderOpen()

    await user.click(screen.getByRole('button', { name: 'Share Link' }))

    await waitFor(() => expect(share).toHaveBeenCalled())
    expect(share.mock.calls[0][0]).toMatchObject({ url: room.joinUrl })
    delete navigator.share
  })

  it('does not surface the room code', async () => {
    renderOpen()
    await screen.findByRole('img', { name: /qr code to join/i })

    expect(screen.queryByText(/ABC123/)).not.toBeInTheDocument()
  })

  /*
    JQ-305. Inviting is the first task after creating a room, and claiming a seat is the
    first task after following an invite to one — so the same section leads for one player
    and stays out of the other's way.
  */
  describe('hiding the section', () => {
    it('opens for the player who created the room', async () => {
      renderOpen()

      expect(screen.getByRole('button', { name: /invite friends/i })).toHaveAttribute(
        'aria-expanded',
        'true',
      )
      expect(await screen.findByRole('img', { name: /qr code to join/i })).toBeInTheDocument()
    })

    it('starts hidden for a player who arrived through a QR code or room link', () => {
      render(<GroupInviteCard room={room} game={game} defaultOpen={false} />)

      expect(screen.getByRole('button', { name: /invite friends/i })).toHaveAttribute(
        'aria-expanded',
        'false',
      )
      expect(screen.queryByRole('img', { name: /qr code to join/i })).not.toBeInTheDocument()
    })

    // An arriving player is not asked to render a QR nobody is looking at.
    it('generates no QR code while it is hidden', () => {
      render(<GroupInviteCard room={room} game={game} defaultOpen={false} />)

      expect(QRCode.toDataURL).not.toHaveBeenCalled()
    })

    it('lets the creator put it away', async () => {
      const user = userEvent.setup()
      renderOpen()
      await screen.findByRole('img', { name: /qr code to join/i })

      await user.click(screen.getByRole('button', { name: /invite friends/i }))

      expect(screen.queryByRole('img', { name: /qr code to join/i })).not.toBeInTheDocument()
      expect(screen.getByRole('button', { name: /invite friends/i })).toHaveAttribute(
        'aria-expanded',
        'false',
      )
    })

    it('lets an arriving player open it when they do want to invite someone', async () => {
      const user = userEvent.setup()
      render(<GroupInviteCard room={room} game={game} defaultOpen={false} />)

      await user.click(screen.getByRole('button', { name: /invite friends/i }))

      expect(await screen.findByRole('img', { name: /qr code to join/i })).toBeInTheDocument()
      expect(screen.getByRole('button', { name: 'Share Link' })).toBeInTheDocument()
    })

    /*
      The room arrives a render or two after the page does, so `defaultOpen` is false until
      the host resolves. The creator must still get their QR when it does.
    */
    it('opens once the room loads and names the viewer as its host', async () => {
      const { rerender } = render(<GroupInviteCard room={null} game={game} defaultOpen={false} />)
      expect(screen.queryByRole('img', { name: /qr code to join/i })).not.toBeInTheDocument()

      rerender(<GroupInviteCard room={room} game={game} defaultOpen />)

      expect(await screen.findByRole('img', { name: /qr code to join/i })).toBeInTheDocument()
    })

    // ...but a player who has already decided outranks the default that arrives late.
    it('keeps the player’s own choice when the default changes under it', async () => {
      const user = userEvent.setup()
      const { rerender } = render(<GroupInviteCard room={room} game={game} defaultOpen={false} />)

      await user.click(screen.getByRole('button', { name: /invite friends/i }))
      await screen.findByRole('img', { name: /qr code to join/i })
      rerender(<GroupInviteCard room={room} game={game} defaultOpen={false} />)

      expect(screen.getByRole('img', { name: /qr code to join/i })).toBeInTheDocument()
    })
  })
})
