import { render, screen } from '@testing-library/react'
import { describe, it, expect, beforeEach } from 'vitest'
import AccountPage from './AccountPage'
import { AuthProvider } from './AuthProvider'
import { mockAuthenticatedSession } from '../../test/setup'

function countQueries(name) {
  return global.fetch.mock.calls.filter((call) => {
    const body = call[1]?.body
    return body ? (JSON.parse(body).query ?? '').includes(name) : false
  }).length
}

describe('AccountPage', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/account')
  })

  // JQ-200: loadAccount fed myAccount.user back into the session, and the effect
  // that runs it was keyed on the user object's identity — so every response
  // scheduled the next fetch, and the pair repeated every ~20ms until the page
  // navigated away.
  it('queries myAccount and subscriptionAuth once each on load', async () => {
    mockAuthenticatedSession()

    render(
      <AuthProvider>
        <AccountPage />
      </AuthProvider>,
    )

    await screen.findByRole('heading', { name: 'Account settings' })
    // Let the page's one-shot loads land first: the linked email comes from
    // myAccount, the spirit-animal offer from the session card's eligibility
    // fetch. Anything still updating after this is a loop, not startup.
    await screen.findByText('player@example.com')
    await screen.findByText('Find my spirit animal')

    // Room for a feedback loop to show itself: the fetch mock resolves
    // immediately, so a loop racks up dozens of round trips in this window.
    // Deliberately not wrapped in act() — a loop never lets React go idle, so
    // act() would hang here instead of reporting the count.
    await new Promise((resolve) => setTimeout(resolve, 100))

    expect(countQueries('myAccount')).toBe(1)
    expect(countQueries('subscriptionAuth')).toBe(1)
  })
})
