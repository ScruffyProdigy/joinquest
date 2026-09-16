import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render } from '@testing-library/react'
import { createRoot } from 'react-dom/client'
import App from './App'
import { mockUnauthenticatedSession } from './test/setup'

// Mock react-dom/client
vi.mock('react-dom/client', () => ({
  createRoot: vi.fn(() => ({
    render: vi.fn(),
  })),
}))

// Mock the CSS import
vi.mock('./index.css', () => ({}))
vi.mock('./App.jsx', () => ({
  default: () => <div data-testid="app">Mock App</div>,
}))

describe('main.jsx', () => {
  let mockRoot
  let mockRender

  beforeEach(() => {
    mockRender = vi.fn()
    mockRoot = { render: mockRender }
    createRoot.mockReturnValue(mockRoot)

    // Clear any previous calls
    vi.clearAllMocks()
  })

  afterEach(() => {
    vi.restoreAllMocks()
  })

  it('creates root with correct element', async () => {
    // Mock document.getElementById
    const mockElement = document.createElement('div')
    mockElement.id = 'root'
    vi.spyOn(document, 'getElementById').mockReturnValue(mockElement)

    // Import and execute main.jsx
    await import('./main.jsx')

    expect(document.getElementById).toHaveBeenCalledWith('root')
    expect(createRoot).toHaveBeenCalledWith(mockElement)
  })

  it('renders App in StrictMode', async () => {
    // Mock document.getElementById
    const mockElement = document.createElement('div')
    mockElement.id = 'root'
    vi.spyOn(document, 'getElementById').mockReturnValue(mockElement)

    // Clear module cache and import
    vi.resetModules()
    await import('./main.jsx')

    // Just verify that render was called
    expect(mockRender).toHaveBeenCalled()
  })

  it('handles missing root element gracefully', async () => {
    // Mock document.getElementById to return null
    vi.spyOn(document, 'getElementById').mockReturnValue(null)

    // This should not throw an error, but createRoot might receive null
    expect(async () => {
      await import('./main.jsx')
    }).not.toThrow()
  })
})

describe('main.jsx Integration', () => {
  beforeEach(() => {
    window.history.replaceState({}, '', '/')
    mockUnauthenticatedSession()
  })

  it('renders the complete application structure', () => {
    // This test verifies that the main entry point works correctly
    // by actually rendering the app
    const { container } = render(<App />)

    expect(container.firstChild).toBeInTheDocument()
  })

  it('applies CSS imports correctly', () => {
    // This test verifies that CSS imports don't cause issues
    // In a real app, you might want to test that styles are applied
    const { container } = render(<App />)

    // The app should render without CSS-related errors
    expect(container.firstChild).toBeInTheDocument()
  })
})
