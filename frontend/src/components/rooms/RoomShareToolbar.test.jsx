// frontend/src/components/rooms/RoomShareToolbar.test.jsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import RoomShareToolbar from './RoomShareToolbar'

vi.mock('qrcode', () => ({
  default: { toDataURL: vi.fn().mockResolvedValue('data:image/png;base64,stub') },
}))

describe('RoomShareToolbar', () => {
  it('shows the room code', () => {
    render(<RoomShareToolbar joinUrl="https://joinquest.cc/r/abc" inviteCode="ABCD" />)
    expect(screen.getByText('ABCD')).toBeInTheDocument()
  })

  it('opens the QR dialog when QR is clicked', async () => {
    const user = userEvent.setup()
    render(<RoomShareToolbar joinUrl="https://joinquest.cc/r/abc" inviteCode="ABCD" />)
    await user.click(screen.getByRole('button', { name: 'QR' }))
    expect(await screen.findByText('Scan to join')).toBeInTheDocument()
  })

  it('copies the invite link when Copy is clicked', async () => {
    const user = userEvent.setup()
    const mockWriteText = vi.fn().mockResolvedValue(undefined)
    vi.spyOn(navigator.clipboard, 'writeText').mockImplementation(mockWriteText)
    render(<RoomShareToolbar joinUrl="https://joinquest.cc/r/abc" inviteCode="ABCD" />)
    await user.click(screen.getByRole('button', { name: 'Copy' }))
    expect(mockWriteText).toHaveBeenCalled()
    expect(await screen.findByText('Copied!')).toBeInTheDocument()
  })
})
