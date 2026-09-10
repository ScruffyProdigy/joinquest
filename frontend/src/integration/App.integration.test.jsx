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

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' })).toBeInTheDocument()
      })
      expect(screen.queryByRole('heading', { name: IDENTITY_GATE_HEADING })).not.toBeInTheDocument()
    })

    it('leads with the catalog and an account chip when logged in', async () => {
      mockAuthenticatedSession()
      render(<App />)

      expect(await screen.findByRole('link', { name: /player/ })).toHaveAttribute('href', '/account')
      expect(await screen.findByRole('heading', { level: 1, name: 'Solo or squad, just join.' })).toBeInTheDocument()
      expect(
        await screen.findByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' }),
      ).toBeInTheDocument()
    })
  })
})
