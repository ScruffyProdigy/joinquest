import { render, screen, act, fireEvent } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import LaunchStep from './LaunchStep'
import { LAUNCH_COUNTDOWN_SECONDS, MATCH_FOUND_BEAT_MS, navigateToLaunchUrl } from '../../lib/launch'

vi.mock('../../lib/launch', async (importOriginal) => ({
  ...(await importOriginal()),
  navigateToLaunchUrl: vi.fn(),
}))

const matchedIntent = {
  queueId: 'q1',
  gameId: 'g1',
  gameName: 'Word Hunt',
  modeName: 'Arena',
  status: 'MATCHED',
  joinUrl: 'https://game.example/play?match=m1',
}

function setVisibility(state) {
  Object.defineProperty(document, 'visibilityState', {
    configurable: true,
    get: () => state,
  })
}

function renderLaunchStep(overrides = {}) {
  const props = {
    activeIntent: matchedIntent,
    activeTableSeat: null,
    busy: false,
    leaveError: null,
    onLeave: vi.fn(),
    ...overrides,
  }
  return { props, ...render(<LaunchStep {...props} />) }
}

/** Past the "You're in!" beat and all the way through the countdown. */
function runCountdown() {
  act(() => {
    vi.advanceTimersByTime(MATCH_FOUND_BEAT_MS)
  })
  act(() => {
    vi.advanceTimersByTime(LAUNCH_COUNTDOWN_SECONDS * 1000)
  })
}

describe('LaunchStep', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    setVisibility('visible')
    navigateToLaunchUrl.mockClear()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('marks the match forming before anything asks the player to act', () => {
    renderLaunchStep()

    expect(screen.getByRole('heading', { name: "You're in!" })).toBeInTheDocument()
    expect(screen.getByText('Word Hunt · Arena')).toBeInTheDocument()
    expect(navigateToLaunchUrl).not.toHaveBeenCalled()
  })

  it('counts down and carries the player into the game without a click', () => {
    renderLaunchStep()

    act(() => {
      vi.advanceTimersByTime(MATCH_FOUND_BEAT_MS)
    })
    expect(screen.getByRole('heading', { name: 'Ready to launch!' })).toBeInTheDocument()
    expect(screen.getByText(`Entering in ${LAUNCH_COUNTDOWN_SECONDS}…`)).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.getByText(`Entering in ${LAUNCH_COUNTDOWN_SECONDS - 1}…`)).toBeInTheDocument()
    expect(navigateToLaunchUrl).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime((LAUNCH_COUNTDOWN_SECONDS - 1) * 1000)
    })
    expect(navigateToLaunchUrl).toHaveBeenCalledWith(matchedIntent.joinUrl)
  })

  it('keeps a manual launch link for anyone whose countdown does not carry them', () => {
    renderLaunchStep()
    act(() => {
      vi.advanceTimersByTime(MATCH_FOUND_BEAT_MS)
    })

    expect(screen.getByRole('link', { name: 'Launch game' })).toHaveAttribute(
      'href',
      matchedIntent.joinUrl,
    )
  })

  it('holds the countdown rather than yanking a tab the player is not looking at', () => {
    setVisibility('hidden')
    renderLaunchStep()

    runCountdown()

    expect(navigateToLaunchUrl).not.toHaveBeenCalled()
    expect(
      screen.getByText('Countdown paused while you were away. Launch when you are ready.'),
    ).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Launch game' })).toBeInTheDocument()
  })

  it('stops the countdown when the player leaves the tab mid-count', () => {
    renderLaunchStep()
    act(() => {
      vi.advanceTimersByTime(MATCH_FOUND_BEAT_MS)
    })

    setVisibility('hidden')
    act(() => {
      vi.advanceTimersByTime(LAUNCH_COUNTDOWN_SECONDS * 1000)
    })

    expect(navigateToLaunchUrl).not.toHaveBeenCalled()
  })

  it('shows the pending state instead of a dead countdown while joinUrl is unprovisioned', () => {
    const { rerender, props } = renderLaunchStep({
      activeIntent: { ...matchedIntent, joinUrl: null },
    })

    runCountdown()

    expect(screen.getByText('Preparing your launch link…')).toBeInTheDocument()
    expect(screen.queryByRole('link', { name: 'Launch game' })).toBeNull()
    expect(navigateToLaunchUrl).not.toHaveBeenCalled()

    // The banner polls for the URL; the countdown starts when it lands.
    rerender(<LaunchStep {...props} activeIntent={matchedIntent} />)
    expect(screen.getByText(`Entering in ${LAUNCH_COUNTDOWN_SECONDS}…`)).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(LAUNCH_COUNTDOWN_SECONDS * 1000)
    })
    expect(navigateToLaunchUrl).toHaveBeenCalledWith(matchedIntent.joinUrl)
  })

  it('falls back to the table seat launch URL', () => {
    renderLaunchStep({
      activeIntent: { ...matchedIntent, joinUrl: null },
      activeTableSeat: { status: 'started', joinUrl: 'https://game.example/table' },
    })

    runCountdown()

    expect(navigateToLaunchUrl).toHaveBeenCalledWith('https://game.example/table')
  })

  it('lets the player leave, and stops carrying them anywhere once they do', () => {
    const onLeave = vi.fn()
    renderLaunchStep({ onLeave })
    act(() => {
      vi.advanceTimersByTime(MATCH_FOUND_BEAT_MS)
    })

    fireEvent.click(screen.getByRole('button', { name: 'Leave game' }))
    expect(onLeave).toHaveBeenCalledTimes(1)

    act(() => {
      vi.advanceTimersByTime(LAUNCH_COUNTDOWN_SECONDS * 1000)
    })
    expect(navigateToLaunchUrl).not.toHaveBeenCalled()
  })

  it('surfaces a leave failure', () => {
    renderLaunchStep({ leaveError: 'Could not leave right now. Please try again.' })

    expect(screen.getByRole('alert')).toHaveTextContent('Could not leave right now.')
  })
})
