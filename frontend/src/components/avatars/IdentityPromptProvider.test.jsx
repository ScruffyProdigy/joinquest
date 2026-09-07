import { describe, expect, it, vi, beforeEach } from 'vitest'
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import IdentityPromptProvider, { useIdentityPrompt } from './IdentityPromptProvider'
import { notifyIdentityRequired } from '../../lib/identityPrompt'

const authState = { user: null, loading: false, refreshSession: async () => {} }

vi.mock('../auth/AuthProvider', () => ({
  useAuth: () => authState,
}))

vi.mock('./IdentityGate', () => ({
  default: () => <div data-testid="identity-gate">Pick an avatar</div>,
}))

function completeUser() {
  return { id: 'user-1', isGuest: true, displayName: 'Ryan', avatarKey: 'sigil-canine' }
}

function namelessUser() {
  return { id: 'user-1', isGuest: true, displayName: null, avatarKey: null }
}

function Joiner({ action }) {
  const { requireIdentity } = useIdentityPrompt()
  return (
    <button type="button" onClick={() => void requireIdentity(action).catch(() => {})}>
      Look for a group
    </button>
  )
}

function renderJoiner(action) {
  return render(
    <IdentityPromptProvider>
      <Joiner action={action} />
    </IdentityPromptProvider>,
  )
}

/** Stand in for the profile save: the session gains a name and an avatar. */
async function finishIdentity(rerender, action) {
  authState.user = completeUser()
  rerender(
    <IdentityPromptProvider>
      <Joiner action={action} />
    </IdentityPromptProvider>,
  )
}

describe('IdentityPromptProvider', () => {
  beforeEach(() => {
    authState.user = null
    authState.loading = false
    authState.refreshSession = async () => {}
  })

  it('does not prompt until an action asks for identity', () => {
    renderJoiner(vi.fn())
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('raises the prompt instead of running an action for a nameless visitor', async () => {
    const action = vi.fn()
    renderJoiner(action)

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))

    expect(screen.getByTestId('identity-gate')).toBeInTheDocument()
    expect(action).not.toHaveBeenCalled()
  })

  it('replays the pending action once identity is chosen', async () => {
    const action = vi.fn().mockResolvedValue('queued')
    const { rerender } = renderJoiner(action)

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))
    expect(action).not.toHaveBeenCalled()

    await finishIdentity(rerender, action)

    await waitFor(() => expect(action).toHaveBeenCalledTimes(1))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('runs the action straight through when the profile is already complete', async () => {
    authState.user = completeUser()
    const action = vi.fn().mockResolvedValue('queued')
    renderJoiner(action)

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))

    await waitFor(() => expect(action).toHaveBeenCalledTimes(1))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('prompts and replays when the backend rejects a session whose name was dropped', async () => {
    authState.user = completeUser()
    const action = vi
      .fn()
      .mockRejectedValueOnce(new Error('identity required'))
      .mockResolvedValue('queued')
    // Re-reading the session is what reveals the missing name (JQ-182).
    authState.refreshSession = async () => {
      authState.user = namelessUser()
    }
    const { rerender } = renderJoiner(action)

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))

    await waitFor(() => expect(screen.getByTestId('identity-gate')).toBeInTheDocument())
    expect(action).toHaveBeenCalledTimes(1)

    await finishIdentity(rerender, action)

    await waitFor(() => expect(action).toHaveBeenCalledTimes(2))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('retries once without a prompt when the refreshed session is complete', async () => {
    authState.user = completeUser()
    const action = vi
      .fn()
      .mockRejectedValueOnce(new Error('identity required'))
      .mockResolvedValue('queued')
    renderJoiner(action)

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))

    await waitFor(() => expect(action).toHaveBeenCalledTimes(2))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('reports a failed replay to the caller rather than reopening the prompt', async () => {
    const rejected = []
    const action = vi.fn().mockRejectedValue(new Error('This mode is full'))
    function FailingJoiner() {
      const { requireIdentity } = useIdentityPrompt()
      return (
        <button
          type="button"
          onClick={() => void requireIdentity(action).catch((err) => rejected.push(err.message))}
        >
          Look for a group
        </button>
      )
    }
    const { rerender } = render(
      <IdentityPromptProvider>
        <FailingJoiner />
      </IdentityPromptProvider>,
    )

    await userEvent.click(screen.getByRole('button', { name: 'Look for a group' }))
    authState.user = completeUser()
    rerender(
      <IdentityPromptProvider>
        <FailingJoiner />
      </IdentityPromptProvider>,
    )

    await waitFor(() => expect(rejected).toEqual(['This mode is full']))
    expect(screen.queryByTestId('identity-gate')).not.toBeInTheDocument()
  })

  it('opens the prompt for a rejection raised outside requireIdentity', async () => {
    render(
      <IdentityPromptProvider>
        <span>catalog</span>
      </IdentityPromptProvider>,
    )

    act(() => notifyIdentityRequired())

    await waitFor(() => expect(screen.getByTestId('identity-gate')).toBeInTheDocument())
  })
})
