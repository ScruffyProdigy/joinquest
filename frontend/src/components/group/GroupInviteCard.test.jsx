import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import GroupInviteCard from './GroupInviteCard'

vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,stub') },
}))

const room = { joinUrl: 'https://joinquest.cc/room/ABC123', inviteCode: 'ABC123' }
const game = { slug: 'word-hunt', accentColor: '#f2b134' }

beforeEach(() => {
  vi.clearAllMocks()
})

describe('GroupInviteCard', () => {
  it('shows the QR code straight away, with nothing to click first', async () => {
    render(<GroupInviteCard room={room} game={game} />)

    const qr = await screen.findByRole('img', { name: /qr code to join/i })
    expect(qr).toHaveAttribute('src', 'data:image/png;base64,stub')
    expect(screen.queryByRole('button', { name: 'QR' })).not.toBeInTheDocument()
  })

  it('offers Share Link and nothing else', async () => {
    render(<GroupInviteCard room={room} game={game} />)
    await screen.findByRole('img', { name: /qr code to join/i })

    const buttons = screen.getAllByRole('button')
    expect(buttons).toHaveLength(1)
    expect(buttons[0]).toHaveAccessibleName('Share Link')
  })

  it('copies the invite link when the browser cannot share', async () => {
    const user = userEvent.setup()
    const writeText = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(writeText)
    render(<GroupInviteCard room={room} game={game} />)

    await user.click(screen.getByRole('button', { name: 'Share Link' }))

    expect(writeText).toHaveBeenCalledWith(expect.stringContaining(room.joinUrl))
    expect(await screen.findByText('Link copied!')).toBeInTheDocument()
  })

  it('hands the link to the native share sheet when there is one', async () => {
    const user = userEvent.setup()
    const share = vi.fn().mockResolvedValue(undefined)
    navigator.share = share
    render(<GroupInviteCard room={room} game={game} />)

    await user.click(screen.getByRole('button', { name: 'Share Link' }))

    await waitFor(() => expect(share).toHaveBeenCalled())
    expect(share.mock.calls[0][0]).toMatchObject({ url: room.joinUrl })
    delete navigator.share
  })

  it('does not surface the room code', async () => {
    render(<GroupInviteCard room={room} game={game} />)
    await screen.findByRole('img', { name: /qr code to join/i })

    expect(screen.queryByText(/ABC123/)).not.toBeInTheDocument()
  })
})
