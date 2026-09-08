import { describe, expect, it, vi } from 'vitest'
import { fireEvent, render, screen } from '@testing-library/react'
import PreQueueOptionsSheet from './PreQueueOptionsSheet'

const helpersGroup = {
  key: 'helpers',
  kind: 'LOADOUT',
  label: 'Choose your two helpers',
  min: 2,
  max: 2,
}

const roster = {
  available: true,
  groups: [
    {
      key: 'helpers',
      choices: [
        { id: 'ferrus', label: 'Ferrus', description: 'Robot takes 1 mark', locked: false },
        { id: 'tempered', label: 'Tempered', locked: false },
        { id: 'chimera', label: 'Chimera', locked: false },
        {
          id: 'rust',
          label: 'Rust',
          locked: true,
          requirement: {
            __typename: 'RequirementLeaf',
            label: 'Casual wins',
            current: 3,
            target: 5,
          },
        },
      ],
    },
  ],
}

function renderSheet(props = {}) {
  const onConfirm = vi.fn()
  render(
    <PreQueueOptionsSheet
      open
      gameName="RPSLR"
      modeName="Helpers"
      groups={[helpersGroup]}
      queueOptions={roster}
      onConfirm={onConfirm}
      onClose={() => {}}
      {...props}
    />,
  )
  return { onConfirm }
}

describe('PreQueueOptionsSheet', () => {
  it('renders nothing when closed', () => {
    const { container } = render(<PreQueueOptionsSheet open={false} groups={[helpersGroup]} />)
    expect(container).toBeEmptyDOMElement()
  })

  it('shows the mode prompt and the roster', () => {
    renderSheet()
    expect(screen.getByText('Choose your two helpers')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: /Ferrus/ })).toBeInTheDocument()
  })

  it('keeps the join button disabled until the group is satisfied', () => {
    renderSheet()
    const join = screen.getByRole('button', { name: 'Look for group' })
    expect(join).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /Ferrus/ }))
    expect(join).toBeDisabled()

    fireEvent.click(screen.getByRole('button', { name: /Tempered/ }))
    expect(join).toBeEnabled()
  })

  it('sends the picks as one selection per group', () => {
    const { onConfirm } = renderSheet()
    fireEvent.click(screen.getByRole('button', { name: /Ferrus/ }))
    fireEvent.click(screen.getByRole('button', { name: /Tempered/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Look for group' }))

    expect(onConfirm).toHaveBeenCalledWith([
      { groupKey: 'helpers', optionIds: ['ferrus', 'tempered'] },
    ])
  })

  it('shows a locked option with its unlock progress instead of hiding it', () => {
    renderSheet()
    const locked = screen.getByRole('button', { name: /Rust/ })
    expect(locked).toBeDisabled()
    expect(screen.getByText('3/5 Casual wins')).toBeInTheDocument()
  })

  it('replaces the oldest pick when the group is already full', () => {
    const { onConfirm } = renderSheet()
    fireEvent.click(screen.getByRole('button', { name: /Ferrus/ }))
    fireEvent.click(screen.getByRole('button', { name: /Tempered/ }))
    fireEvent.click(screen.getByRole('button', { name: /Chimera/ }))
    fireEvent.click(screen.getByRole('button', { name: 'Look for group' }))

    expect(onConfirm).toHaveBeenCalledWith([
      { groupKey: 'helpers', optionIds: ['tempered', 'chimera'] },
    ])
  })

  it('deselects a chosen option when tapped again', () => {
    renderSheet()
    const ferrus = screen.getByRole('button', { name: /Ferrus/ })
    fireEvent.click(ferrus)
    expect(ferrus).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(ferrus)
    expect(ferrus).toHaveAttribute('aria-pressed', 'false')
  })

  it('blocks the join and names the reason when the game cannot serve a roster', () => {
    renderSheet({
      queueOptions: { available: false, unavailableReason: 'Cannot reach the game', groups: [] },
    })
    expect(screen.getByText('Cannot reach the game')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Look for group' })).toBeDisabled()
  })

  it('satisfies an optional group with no picks at all', () => {
    renderSheet({
      groups: [{ ...helpersGroup, key: 'skin', label: 'Pick a skin', min: 0, max: 1 }],
      queueOptions: { available: true, groups: [{ key: 'skin', choices: [] }] },
    })
    expect(screen.getByRole('button', { name: 'Look for group' })).toBeEnabled()
  })
})
