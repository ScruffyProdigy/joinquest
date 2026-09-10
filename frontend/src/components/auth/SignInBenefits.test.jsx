import { render, screen, act, fireEvent } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, it, expect, afterEach, vi } from 'vitest'
import SignInBenefits from './SignInBenefits'
import { SIGN_IN_BENEFITS } from '../../lib/playerCopy'

const [first, second, , fourth] = SIGN_IN_BENEFITS

afterEach(() => {
  vi.useRealTimers()
})

function reduceMotion(matches) {
  window.matchMedia = vi.fn().mockImplementation((query) => ({
    matches: query.includes('prefers-reduced-motion') ? matches : false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  }))
}

describe('SignInBenefits', () => {
  it('opens on the first reason with a dot for each', () => {
    render(<SignInBenefits />)

    expect(screen.getByText(first.headline)).toBeInTheDocument()
    expect(screen.getByText(first.body)).toBeInTheDocument()
    expect(screen.getAllByRole('button')).toHaveLength(SIGN_IN_BENEFITS.length)
  })

  it('rotates to the next reason on its own', () => {
    vi.useFakeTimers()
    render(<SignInBenefits />)

    act(() => vi.advanceTimersByTime(4000))

    expect(screen.getByText(second.headline)).toBeInTheDocument()
    expect(screen.queryByText(first.headline)).not.toBeInTheDocument()
  })

  it('wraps back to the first reason after the last', () => {
    vi.useFakeTimers()
    render(<SignInBenefits />)

    act(() => vi.advanceTimersByTime(4000 * SIGN_IN_BENEFITS.length))

    expect(screen.getByText(first.headline)).toBeInTheDocument()
  })

  /**
   * The point of the dots is to read a card at your own pace, so the timer has
   * to stay stopped afterwards rather than moving on four seconds later.
   */
  it('stops rotating for good once a reason is picked', () => {
    vi.useFakeTimers()
    render(<SignInBenefits />)

    fireEvent.click(screen.getAllByRole('button')[3])
    expect(screen.getByText(fourth.headline)).toBeInTheDocument()

    act(() => vi.advanceTimersByTime(4000 * 3))

    expect(screen.getByText(fourth.headline)).toBeInTheDocument()
  })

  it('marks the showing reason as current for assistive tech', async () => {
    const user = userEvent.setup()
    render(<SignInBenefits />)

    const dots = screen.getAllByRole('button')
    expect(dots[0]).toHaveAttribute('aria-current', 'true')

    await user.click(dots[2])

    expect(dots[0]).not.toHaveAttribute('aria-current')
    expect(dots[2]).toHaveAttribute('aria-current', 'true')
  })

  it('does not rotate at all under prefers-reduced-motion', () => {
    reduceMotion(true)
    vi.useFakeTimers()
    render(<SignInBenefits />)

    act(() => vi.advanceTimersByTime(4000 * 3))

    expect(screen.getByText(first.headline)).toBeInTheDocument()
    reduceMotion(false)
  })

  /**
   * Auto-moving content that also announces itself talks over a screen reader
   * every four seconds, so it only becomes a live region once it has stopped.
   */
  it('announces only once it is no longer moving on its own', async () => {
    const user = userEvent.setup()
    const { container } = render(<SignInBenefits />)

    const card = container.querySelector('.sign-in-benefits__card')
    expect(card).toHaveAttribute('aria-live', 'off')

    await user.click(screen.getAllByRole('button')[1])

    expect(card).toHaveAttribute('aria-live', 'polite')
  })
})
