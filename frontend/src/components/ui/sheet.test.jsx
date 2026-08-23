import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi } from 'vitest'
import { Sheet, SheetContent, SheetTitle } from './sheet'

describe('Sheet', () => {
  it('renders content when open', () => {
    render(
      <Sheet open onOpenChange={() => {}}>
        <SheetContent side="bottom">
          <SheetTitle>Choose your seat</SheetTitle>
        </SheetContent>
      </Sheet>,
    )
    expect(screen.getByText('Choose your seat')).toBeInTheDocument()
  })

  it('calls onOpenChange(false) when the close button is activated', async () => {
    const user = userEvent.setup()
    const onOpenChange = vi.fn()
    render(
      <Sheet open onOpenChange={onOpenChange}>
        <SheetContent side="bottom">
          <SheetTitle>Choose your seat</SheetTitle>
        </SheetContent>
      </Sheet>,
    )
    await user.click(screen.getByRole('button', { name: 'Close' }))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
