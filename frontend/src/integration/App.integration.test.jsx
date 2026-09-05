import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import App from '../App'
import { IDENTITY_GATE_HEADING } from '../lib/playerCopy'
import { mockAuthenticatedSession, mockDemoGames, mockUnauthenticatedSession } from '../test/setup'

describe('App Integration Tests', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  describe('User Journey: Content Discovery', () => {
    it('lets a logged-out visitor browse the catalog without picking a name first', async () => {
      mockUnauthenticatedSession({ games: mockDemoGames })
      render(<App />)

      expect(screen.getByRole('heading', { level: 1, name: 'JoinQuest' })).toBeInTheDocument()
      expect(screen.getByText('Find your group. Play together.')).toBeInTheDocument()

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
      })
      expect(screen.queryByRole('heading', { name: IDENTITY_GATE_HEADING })).not.toBeInTheDocument()
    })

    it('shows account details when logged in', async () => {
      mockAuthenticatedSession()
      render(<App />)

      await waitFor(() => {
        expect(screen.getByText('player@example.com')).toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'Available games' })).toBeInTheDocument()
        expect(screen.getByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
      })
    })
  })
})
