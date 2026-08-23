import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { AuthProvider } from '../auth/AuthProvider'
import DeveloperHomeBlock from './DeveloperHomeBlock'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'

vi.mock('../../lib/developers', async (importOriginal) => {
  const actual = await importOriginal()
  return {
    ...actual,
    fetchMyGames: vi.fn().mockResolvedValue([]),
  }
})

const guestUser = {
  id: 'guest-1',
  email: null,
  displayName: 'guest#123456',
  isGuest: true,
  createdAt: '2026-01-01T00:00:00Z',
}

function renderHomeBlock() {
  return render(
    <AuthProvider>
      <DeveloperHomeBlock />
    </AuthProvider>,
  )
}

async function expectDeveloperEntryPoint() {
  await waitFor(() => {
    expect(screen.getByRole('link', { name: /get started for developers/i })).toHaveAttribute(
      'href',
      '/developers',
    )
  })
}

// JQ-41: the prototype hid this behind !isGuest. Guests must keep a path to the dev portal.
describe('DeveloperHomeBlock', () => {
  beforeEach(() => {
    window.history.replaceState(null, '', '/')
  })

  it('offers the developer portal to a signed-out visitor', async () => {
    mockUnauthenticatedSession()
    renderHomeBlock()

    await expectDeveloperEntryPoint()
  })

  it('offers the developer portal to a guest', async () => {
    mockAuthenticatedSession(guestUser)
    renderHomeBlock()

    await expectDeveloperEntryPoint()
  })

  it('offers the developer portal to a signed-in account', async () => {
    mockAuthenticatedSession()
    renderHomeBlock()

    await expectDeveloperEntryPoint()
  })
})
