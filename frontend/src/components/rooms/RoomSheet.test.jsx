import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import RoomSheet from './RoomSheet'

describe('RoomSheet', () => {
  it('renders children when open', () => {
    render(
      <RoomSheet open onDismiss={() => {}}>
        <p>Room contents</p>
      </RoomSheet>,
    )
    expect(screen.getByText('Room contents')).toBeInTheDocument()
  })

  it('does not render content when closed', () => {
    render(
      <RoomSheet open={false} onDismiss={() => {}}>
        <p>Room contents</p>
      </RoomSheet>,
    )
    expect(screen.queryByText('Room contents')).not.toBeInTheDocument()
  })

  it('calls onDismiss when the close button is activated', async () => {
    const user = userEvent.setup()
    const onDismiss = vi.fn()
    render(
      <RoomSheet open onDismiss={onDismiss}>
        <p>Room contents</p>
      </RoomSheet>,
    )
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(onDismiss).toHaveBeenCalled()
  })
})
