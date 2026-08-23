import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import PlayerAvatar from './PlayerAvatar'

describe('PlayerAvatar', () => {
  it('renders image when avatarUrl is set', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat', avatarUrl: '/avatars/compass.png' }} />,
    )
    expect(container.querySelector('[data-slot="avatar-image"]')).toBeTruthy()
  })

  it('falls back to initial without avatarUrl', () => {
    render(<PlayerAvatar user={{ displayName: 'River' }} />)
    expect(screen.getByText('R')).toBeTruthy()
  })

  it('renders image from avatarKey when avatarUrl is missing', () => {
    const { container } = render(<PlayerAvatar user={{ displayName: 'River', avatarKey: 'storm' }} />)
    const img = container.querySelector('[data-slot="avatar-image"]')
    expect(img?.getAttribute('src')).toBe('/avatars/storm.png')
  })

  it('marks the avatar with a king ring when ring is king', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat', avatarUrl: '/avatars/compass.png' }} ring="king" />,
    )
    const avatar = container.querySelector('[data-slot="avatar"][data-ring="king"]')
    expect(avatar).toBeTruthy()
    expect(avatar.querySelector('[data-slot="avatar-image"]')).toBeTruthy()
  })
})
