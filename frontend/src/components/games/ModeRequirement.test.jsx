import { describe, expect, it } from 'vitest'
import { render, screen } from '@testing-library/react'
import ModeRequirement from './ModeRequirement'

describe('ModeRequirement', () => {
  it('renders nothing for a null requirement', () => {
    const { container } = render(<ModeRequirement requirement={null} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('renders a leaf as current/target label', () => {
    render(
      <ModeRequirement
        requirement={{ __typename: 'RequirementLeaf', label: 'Ranked matches', current: 12, target: 50 }}
      />,
    )
    expect(screen.getByText('12/50 Ranked matches')).toBeInTheDocument()
  })

  it('renders an ALL group joined by "and"', () => {
    render(
      <ModeRequirement
        requirement={{
          __typename: 'RequirementGroup',
          label: 'Commander requirements',
          operator: 'ALL',
          children: [
            { __typename: 'RequirementLeaf', label: 'Standard wins', current: 18, target: 25 },
            { __typename: 'RequirementLeaf', label: 'Unique decks used', current: 3, target: 5 },
          ],
        }}
      />,
    )
    expect(screen.getByText('18/25 Standard wins')).toBeInTheDocument()
    expect(screen.getByText('and')).toBeInTheDocument()
    expect(screen.getByText('3/5 Unique decks used')).toBeInTheDocument()
  })

  it('renders an ANY group joined by "or"', () => {
    render(
      <ModeRequirement
        requirement={{
          __typename: 'RequirementGroup',
          label: 'x',
          operator: 'ANY',
          children: [
            { __typename: 'RequirementLeaf', label: 'A', current: 1, target: 2 },
            { __typename: 'RequirementLeaf', label: 'B', current: 3, target: 4 },
          ],
        }}
      />,
    )
    expect(screen.getByText('or')).toBeInTheDocument()
  })
})
