// frontend/src/components/ui/dialog.test.jsx
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { Dialog, DialogContent, DialogTitle } from './dialog'

describe('Dialog', () => {
  it('renders content when open', () => {
    render(
      <Dialog open onOpenChange={() => {}}>
        <DialogContent>
          <DialogTitle>Scan to join</DialogTitle>
        </DialogContent>
      </Dialog>,
    )
    expect(screen.getByText('Scan to join')).toBeInTheDocument()
  })

  it('calls onOpenChange(false) when the close button is activated', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(
      <Dialog open onOpenChange={onOpenChange}>
        <DialogContent>
          <DialogTitle>Scan to join</DialogTitle>
        </DialogContent>
      </Dialog>,
    )
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
