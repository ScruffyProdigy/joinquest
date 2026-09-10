import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, vi, beforeEach } from 'vitest'
import SignInDialog from './SignInDialog'
import { AuthProvider } from './AuthProvider'
import { mockUnauthenticatedSession } from '../../test/setup'
import {
  SIGN_IN_BENEFITS,
  SIGN_IN_DIALOG_TITLE,
  SIGN_IN_MAYBE_LATER,
} from '../../lib/playerCopy'

function renderDialog(onOpenChange = vi.fn()) {
  render(
    <AuthProvider>
      <SignInDialog open onOpenChange={onOpenChange} />
    </AuthProvider>,
  )
  return onOpenChange
}

describe('SignInDialog', () => {
  beforeEach(() => {
    mockUnauthenticatedSession()
  })

  it('makes the case for an account with the rotating reasons', async () => {
    renderDialog()

    expect(await screen.findByRole('heading', { name: SIGN_IN_DIALOG_TITLE })).toBeInTheDocument()
    expect(screen.getByText(SIGN_IN_BENEFITS[0].headline)).toBeInTheDocument()
  })

  /**
   * Radix focuses the first tabbable child on open, which the benefit dots would
   * otherwise be: focus would land on "Reason 1 of 4" and, because the carousel
   * pauses while it holds focus, the rotation would never start.
   */
  it('opens with focus on the dialog rather than inside the carousel', async () => {
    renderDialog()

    await waitFor(() => {
      expect(document.activeElement).toHaveAttribute('data-slot', 'dialog-content')
    })
  })

  it('closes from Maybe later', async () => {
    const user = userEvent.setup()
    const onOpenChange = renderDialog()

    await user.click(await screen.findByRole('button', { name: SIGN_IN_MAYBE_LATER }))

    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
