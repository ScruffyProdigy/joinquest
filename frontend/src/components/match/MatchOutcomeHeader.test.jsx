import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import MatchOutcomeHeader from './MatchOutcomeHeader'

const result = (viewer, overrides = {}) => ({
  complete: true,
  participants: [
    { user: { id: 'a', displayName: 'Ada' }, placement: 1, ...viewer },
    { user: { id: 'b', displayName: 'Bo' }, placement: 2, reason: 'COMPLETED' },
    { user: { id: 'c', displayName: 'Cy' }, placement: 3, reason: 'COMPLETED' },
  ],
  ...overrides,
})

describe('MatchOutcomeHeader', () => {
  it('answers "how did I do?" before anything else on the screen', () => {
    render(<MatchOutcomeHeader result={result({ reason: 'COMPLETED' })} viewerId="a" />)
    expect(screen.getByRole('heading', { name: '1st place.' })).toBeInTheDocument()
    expect(screen.getByText('You finished on top.')).toBeInTheDocument()
  })

  it('reads the viewer’s own row, not the winner’s', () => {
    render(<MatchOutcomeHeader result={result({ reason: 'COMPLETED' })} viewerId="c" />)
    expect(screen.getByRole('heading', { name: '3rd place.' })).toBeInTheDocument()
    expect(screen.getByText('Better luck next time.')).toBeInTheDocument()
  })

  /**
   * The mascot art does not exist yet. The prototype ships the slot as a labelled dashed
   * placeholder and so does this, deliberately: a screen that quietly omits it loses the
   * prototype's vertical rhythm and hides the fact that something is missing (JQ-277).
   */
  it('reserves the mascot slot, tinted by how the match went', () => {
    const { unmount } = render(
      <MatchOutcomeHeader result={result({ reason: 'COMPLETED' })} viewerId="a" />,
    )
    expect(screen.getByTestId('mascot-panel')).toHaveAttribute('data-mood', 'win')
    expect(screen.getByText('Mascot illustration · celebrating')).toBeInTheDocument()
    unmount()

    render(<MatchOutcomeHeader result={result({ reason: 'ELIMINATED', placement: 3 })} viewerId="a" />)
    expect(screen.getByTestId('mascot-panel')).toHaveAttribute('data-mood', 'lose')
    expect(screen.getByText('Mascot illustration · sympathetic')).toBeInTheDocument()
  })

  it('tells a player who is out in front that the others are still going', () => {
    render(
      <MatchOutcomeHeader result={result({ reason: 'COMPLETED' }, { complete: false })} viewerId="a" />,
    )
    expect(screen.getByRole('heading', { name: '1st place.' })).toBeInTheDocument()
    expect(screen.getByText('Waiting for others to finish.')).toBeInTheDocument()
    // Leading an unfinished match is the one wait that is good news.
    expect(screen.getByTestId('mascot-panel')).toHaveAttribute('data-mood', 'win')
  })

  // An unauthenticated or non-participant read has no outcome to report, and an empty hero
  // is worse than none.
  it('renders nothing when the viewer is not on the roster', () => {
    const { container } = render(
      <MatchOutcomeHeader result={result({ reason: 'COMPLETED' })} viewerId="nobody" />,
    )
    expect(container).toBeEmptyDOMElement()
  })
})
