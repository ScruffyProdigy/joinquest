import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { Link } from './link'
import { linkVariants } from './link-variants'

describe('Link', () => {
  it('renders an anchor carrying the href when given one', () => {
    render(<Link href="/account">Account settings</Link>)
    const link = screen.getByRole('link', { name: 'Account settings' })
    expect(link.tagName).toBe('A')
    expect(link).toHaveAttribute('href', '/account')
  })

  it('renders a non-submitting button when there is no href', () => {
    render(<Link onClick={() => {}}>Use a different email</Link>)
    const action = screen.getByRole('button', { name: 'Use a different email' })
    expect(action.tagName).toBe('BUTTON')
    expect(action).toHaveAttribute('type', 'button')
  })

  it('keeps an explicit type on a button-rendered link', () => {
    render(<Link type="submit">Save</Link>)
    expect(screen.getByRole('button')).toHaveAttribute('type', 'submit')
  })

  it('applies the primary text class for the default inline variant', () => {
    render(<Link href="/">Home</Link>)
    expect(screen.getByRole('link')).toHaveClass('text-primary')
  })

  it("applies the prototype's outlined pill classes for the pill variant", () => {
    render(
      <Link variant="pill" href="/play">
        Launch game
      </Link>,
    )
    const link = screen.getByRole('link')
    expect(link).toHaveClass('border-[1.5px]')
    expect(link).toHaveClass('border-primary')
    expect(link).toHaveClass('text-primary')
    expect(link).toHaveClass('rounded-[99px]')
    expect(link).toHaveClass('font-bold')
    expect(link).toHaveClass('active:opacity-80')
  })

  it("applies the prototype's muted treatment for the quiet variant", () => {
    render(<Link variant="quiet">Maybe later</Link>)
    const action = screen.getByRole('button')
    expect(action).toHaveClass('text-muted-foreground')
    expect(action).toHaveClass('font-bold')
    expect(action).toHaveClass('active:opacity-60')
  })

  it('never underlines, in any variant or state', () => {
    for (const variant of ['inline', 'pill', 'quiet']) {
      const classes = linkVariants({ variant }).split(/\s+/)
      expect(classes).toContain('no-underline')
      expect(classes.filter((c) => c.endsWith('underline') && c !== 'no-underline')).toEqual([])
    }
  })

  it('neutralises user-agent button chrome, so both elements render alike', () => {
    // Exempting link-slotted buttons from the app's bare-button reset drops them
    // to the UA default (an outset border and its own padding), so the component
    // has to clear that itself rather than inherit a reset from anywhere.
    const classes = linkVariants({ variant: 'quiet' }).split(/\s+/)
    expect(classes).toContain('appearance-none')
    expect(classes).toContain('border-0')
    expect(classes).toContain('p-0')
    expect(classes).toContain('[font-family:inherit]')
  })

  it('uses the same focus-visible ring as the other brand controls', () => {
    render(<Link href="/">Home</Link>)
    const link = screen.getByRole('link')
    expect(link).toHaveClass('focus-visible:ring-ring/50')
    expect(link).toHaveClass('focus-visible:ring-[3px]')
  })

  it('merges a caller className over the variant classes', () => {
    render(
      <Link href="/" className="self-start">
        Home
      </Link>,
    )
    expect(screen.getByRole('link')).toHaveClass('self-start')
  })
})

/**
 * index.css styles bare `a` and `button` elements app-wide. Both halves of Link
 * are bare by that definition, so without an opt-out the same component picks up
 * chunky button padding and a teal hover border when it renders a button, and a
 * colour shift when it renders an anchor -- the exact drift JQ-72 exists to end.
 */
describe('Link against the global element styles', () => {
  const css = readFileSync(resolve(process.cwd(), 'src/index.css'), 'utf8')

  const selectors = css
    .split('}')
    .map((block) => block.split('{')[0].trim())
    .filter(Boolean)

  it('exempts link-slotted buttons from the bare-button reset', () => {
    const bareButtonRules = selectors.filter((sel) => /^button[:\s]/.test(sel))
    expect(bareButtonRules.length).toBeGreaterThan(0)
    for (const sel of bareButtonRules) {
      expect(sel).toContain(':not([data-slot="link"])')
    }
  })

  it('exempts link-slotted anchors from the bare-anchor hover colour', () => {
    const bareAnchorRules = selectors.filter((sel) => /^a[:,\s]/.test(sel) || sel === 'a')
    expect(bareAnchorRules.length).toBeGreaterThan(0)
    for (const sel of bareAnchorRules) {
      expect(sel).toContain(':not([data-slot="link"])')
    }
  })
})
