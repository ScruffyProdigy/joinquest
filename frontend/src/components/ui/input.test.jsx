import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { Input } from './input'

describe('Input', () => {
  it('renders a text input by default', () => {
    render(<Input aria-label="Name" />)
    const input = screen.getByRole('textbox', { name: 'Name' })
    expect(input).toBeInTheDocument()
    expect(input).toHaveAttribute('type', 'text')
  })

  it('applies the input-background class', () => {
    render(<Input aria-label="Name" />)
    expect(screen.getByRole('textbox')).toHaveClass('bg-input-background')
  })

  it('forwards value, onChange, and other native props', async () => {
    const onChange = vi.fn()
    render(<Input aria-label="Email" type="email" value="a@b.com" onChange={onChange} />)
    const input = screen.getByRole('textbox', { name: 'Email' })
    expect(input).toHaveAttribute('type', 'email')
    expect(input).toHaveValue('a@b.com')
    await userEvent.type(input, 'x')
    expect(onChange).toHaveBeenCalled()
  })

  it('forwards a ref to the native input element', () => {
    const ref = { current: null }
    render(<Input aria-label="Name" ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLInputElement)
  })

  it('merges a custom className with the base classes', () => {
    render(<Input aria-label="Name" className="mt-4" />)
    expect(screen.getByRole('textbox')).toHaveClass('mt-4')
    expect(screen.getByRole('textbox')).toHaveClass('rounded-md')
  })
})
