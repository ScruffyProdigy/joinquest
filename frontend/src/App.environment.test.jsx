import { render, screen } from '@testing-library/react'
import { describe, it, expect, beforeEach, afterEach } from 'vitest'
import App from './App'
import { mockUnauthenticatedSession } from './test/setup'

describe('App Environment Integration', () => {
  let originalWindowEnv

  beforeEach(() => {
    originalWindowEnv = window.env
    window.history.replaceState({}, '', '/')
    mockUnauthenticatedSession()
  })

  afterEach(() => {
    // Restore original window.env
    window.env = originalWindowEnv
  })

  it('renders with environment configuration available', () => {
    render(<App />)
    
    // The app should render without crashing
    expect(screen.getByRole('heading', { level: 1, name: 'Solo or squad, just join.' })).toBeInTheDocument()
    // JQ-71 moved the developer promo into the catalog list, so it arrives with the
    // games rather than on first paint. Its own coverage lives in DeveloperPromoCard
    // and GameLobby; this test is about the environment config.
    
    // Environment should be available
    expect(window.env).toBeDefined()
    expect(window.env.REACT_APP_ENV).toBeDefined()
    expect(window.env.REACT_APP_API_BASE_URL).toBeDefined()
  })

  it('handles missing environment gracefully', () => {
    // Remove window.env
    delete window.env
    
    // App should still render
    render(<App />)
    expect(screen.getByRole('heading', { level: 1, name: 'Solo or squad, just join.' })).toBeInTheDocument()
  })

  it('can access environment variables in component', () => {
    // Mock a component that uses environment variables
    const TestComponent = () => {
      const apiUrl = window.env?.REACT_APP_API_BASE_URL || ''
      const environment = window.env?.REACT_APP_ENV || 'development'
      
      return (
        <div>
          <span data-testid="api-url">{apiUrl}</span>
          <span data-testid="environment">{environment}</span>
        </div>
      )
    }
    
    render(<TestComponent />)
    
    expect(screen.getByTestId('api-url')).toBeInTheDocument()
    expect(screen.getByTestId('environment')).toBeInTheDocument()
  })

  it('environment variables are accessible during render', () => {
    let capturedEnv = null
    
    const TestComponent = () => {
      capturedEnv = window.env
      return <div>Test</div>
    }
    
    render(<TestComponent />)
    
    expect(capturedEnv).toBeDefined()
    expect(capturedEnv.REACT_APP_ENV).toBeDefined()
    expect(capturedEnv.REACT_APP_API_BASE_URL).toBeDefined()
  })
})
