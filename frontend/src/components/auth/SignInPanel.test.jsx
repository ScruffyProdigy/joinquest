import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, beforeEach } from 'vitest'
import { AuthProvider } from './AuthProvider'
import SignInPanel from './SignInPanel'
import { PLAY_AS_GUEST, SIGN_IN_HEADING } from '../../lib/playerCopy'
import { mockUnauthenticatedSession } from '../../test/setup'

function renderPanel(props = {}) {
  return render(
    <AuthProvider>
      <SignInPanel {...props} />
    </AuthProvider>,
  )
}

describe('SignInPanel', () => {
  beforeEach(() => {
    mockUnauthenticatedSession()
  })

  it('shows the default heading and guest option', async () => {
    renderPanel()

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: SIGN_IN_HEADING })).toBeInTheDocument()
    })
    expect(screen.getByRole('button', { name: PLAY_AS_GUEST })).toBeInTheDocument()
  })

  it('renders a caller-supplied heading instead of the default', async () => {
    renderPanel({ heading: 'Build on JoinQuest' })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: 'Build on JoinQuest' })).toBeInTheDocument()
    })
    expect(screen.queryByRole('heading', { name: SIGN_IN_HEADING })).not.toBeInTheDocument()
  })

  it('renders no heading when heading is null, keeping the card labelled', async () => {
    renderPanel({ heading: null })

    await waitFor(() => {
      expect(screen.getByLabelText('Email')).toBeInTheDocument()
    })
    expect(screen.queryByRole('heading')).not.toBeInTheDocument()
    // Without a heading to point at, the card must drop the reference rather than dangle it.
    expect(document.querySelector('[aria-labelledby="sign-in-heading"]')).toBeNull()
  })

  it('hides the guest option when showGuestOption is false', async () => {
    renderPanel({ showGuestOption: false })

    await waitFor(() => {
      expect(screen.getByRole('heading', { name: SIGN_IN_HEADING })).toBeInTheDocument()
    })
    expect(screen.queryByRole('button', { name: PLAY_AS_GUEST })).not.toBeInTheDocument()
  })
})
