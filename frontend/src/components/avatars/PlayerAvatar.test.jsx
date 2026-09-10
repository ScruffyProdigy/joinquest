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

  // JQ-265. Dimming is the visible half and the label is the half a screen reader gets,
  // so both are asserted: an away member who only faded would be invisible to anyone not
  // comparing avatars side by side.
  it('dims an away member and names it in the label', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat', avatarUrl: '/avatars/compass.png' }} away />,
    )
    const avatar = container.querySelector('[data-slot="avatar"][data-away="true"]')
    expect(avatar).toBeTruthy()
    expect(avatar.className).toContain('opacity-40')
    expect(avatar.getAttribute('title')).toBe('Pat (away)')
  })

  it('leaves a present member undimmed and unlabelled', () => {
    const { container } = render(<PlayerAvatar user={{ displayName: 'Pat' }} />)
    const avatar = container.querySelector('[data-slot="avatar"]')
    expect(avatar.getAttribute('data-away')).toBe(null)
    expect(avatar.className).not.toContain('opacity-40')
    expect(avatar.getAttribute('title')).toBe('Pat')
  })

  // The king ring and the away dim are independent readings of the same person, so a
  // king whose phone died has to keep both.
  it('keeps the king ring on an away king and names both', () => {
    const { container } = render(
      <PlayerAvatar user={{ displayName: 'Pat' }} ring="king" away />,
    )
    const avatar = container.querySelector('[data-slot="avatar"][data-away="true"]')
    expect(avatar.getAttribute('data-ring')).toBe('king')
    expect(avatar.getAttribute('title')).toContain('(away)')
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
