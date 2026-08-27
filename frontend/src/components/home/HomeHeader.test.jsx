import { render, screen } from '@testing-library/react'
import { describe, it, expect, afterEach, vi } from 'vitest'
import HomeHeader from './HomeHeader'
import { AuthProvider } from '../auth/AuthProvider'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../../test/setup'

function renderHeader() {
  return render(
    <AuthProvider>
      <HomeHeader />
    </AuthProvider>,
  )
}

afterEach(() => {
  vi.useRealTimers()
})

describe('HomeHeader', () => {
  it('leads with the catalog heading', async () => {
    mockUnauthenticatedSession()
    renderHeader()

    expect(await screen.findByRole('heading', { level: 1, name: 'Find a game' })).toBeInTheDocument()
  })

  it('greets a known player by name', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(new Date(2026, 0, 1, 14, 0))
    mockAuthenticatedSession()
    renderHeader()

    expect(await screen.findByText('Good afternoon, player')).toBeInTheDocument()
  })

  it('greets a guest who has no name yet without a trailing comma', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(new Date(2026, 0, 1, 14, 0))
    mockAuthenticatedSession({
      id: 'guest-1',
      email: null,
      displayName: null,
      isGuest: true,
      createdAt: '2026-01-01T00:00:00Z',
    })
    renderHeader()

    expect(await screen.findByText('Good afternoon')).toBeInTheDocument()
  })

  it('greets a visitor we have no name for without a trailing comma', async () => {
    vi.useFakeTimers({ shouldAdvanceTime: true })
    vi.setSystemTime(new Date(2026, 0, 1, 9, 0))
    mockUnauthenticatedSession()
    renderHeader()

    expect(await screen.findByText('Good morning')).toBeInTheDocument()
  })
})
