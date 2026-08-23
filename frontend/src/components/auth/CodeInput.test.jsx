import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CodeInput from './CodeInput'

function Labeled(props) {
  return (
    <>
      <label htmlFor="test-code">Sign-in code</label>
      <CodeInput id="test-code" {...props} />
    </>
  )
}

describe('CodeInput', () => {
  it('renders a single input associated with its label', () => {
    render(<Labeled value="" onChange={() => {}} />)
    expect(screen.getAllByLabelText('Sign-in code')).toHaveLength(1)
  })

  it('calls onChange as the user types', async () => {
    const onChange = vi.fn()
    render(<Labeled value="" onChange={onChange} />)
    await userEvent.type(screen.getByLabelText('Sign-in code'), '1')
    expect(onChange).toHaveBeenCalled()
  })

  it('renders each typed digit in its own decorative box', () => {
    render(<Labeled value="12" onChange={() => {}} />)
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
  })

  it('forwards maxLength and disabled to the native input', () => {
    render(<Labeled value="" onChange={() => {}} maxLength={6} disabled />)
    const input = screen.getByLabelText('Sign-in code')
    expect(input).toHaveAttribute('maxLength', '6')
    expect(input).toBeDisabled()
  })

  it('forwards a ref to the native input element', () => {
    const ref = { current: null }
    render(<Labeled value="" onChange={() => {}} ref={ref} />)
    expect(ref.current).toBeInstanceOf(HTMLInputElement)
  })
})
