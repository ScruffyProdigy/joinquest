import { render, screen, waitFor } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import App from '../App'
import { IDENTITY_GATE_HEADING } from '../lib/playerCopy'
import { mockAuthenticatedSession, mockUnauthenticatedSession } from '../test/setup'

describe('App Integration Tests', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
  })

  describe('User Journey: Content Discovery', () => {
    it('asks a logged-out visitor for a name and avatar before anything else', async () => {
      mockUnauthenticatedSession()
      render(<App />)

      await waitFor(() => {
        expect(screen.getByRole('heading', { name: IDENTITY_GATE_HEADING })).toBeInTheDocument()
      })
    })

    it('leads with the catalog and an account chip when logged in', async () => {
      mockAuthenticatedSession()
      render(<App />)

      expect(await screen.findByRole('link', { name: /player/ })).toHaveAttribute('href', '/account')
      expect(await screen.findByRole('heading', { level: 1, name: 'Find a game' })).toBeInTheDocument()
      expect(
        await screen.findByRole('heading', { name: 'Rock Paper Scissors Lizard Robot' }),
      ).toBeInTheDocument()
    })
  })
})
