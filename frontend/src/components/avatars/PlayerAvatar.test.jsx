import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerAvatar from './PlayerAvatar'

describe('PlayerAvatar', () => {
  it('renders image when avatarUrl is set', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat', avatarUrl: '/avatars/compass.png' }} />,
    )
    expect(container.querySelector('img.player-avatar')).toBeTruthy()
  })

  it('falls back to initial without avatarUrl', () => {
    render(<PlayerAvatar user={{ displayName: 'River' }} />)
    expect(screen.getByText('R')).toBeTruthy()
  })

  it('renders image from avatarKey when avatarUrl is missing', () => {
    const { container } = render(<PlayerAvatar user={{ displayName: 'River', avatarKey: 'storm' }} />)
    const img = container.querySelector('img.player-avatar')
    expect(img?.getAttribute('src')).toBe('/avatars/storm.png')
  })

  it('wraps the avatar in a gold ring when ring is king', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat', avatarUrl: '/avatars/compass.png' }} ring="king" />,
    )
    expect(container.querySelector('.player-avatar-frame--king')).toBeTruthy()
    expect(container.querySelector('.player-avatar-frame--king img.player-avatar')).toBeTruthy()
  })
})
