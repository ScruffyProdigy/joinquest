import { describe, it, expect, beforeEach } from 'vitest'
import { isStylePreviewEnabled } from './stylePreview'

describe('isStylePreviewEnabled', () => {
  beforeEach(() => {
    window.env = { REACT_APP_STYLE_PREVIEW: '' }
  })

  it('is off when the flag is unset', () => {
    expect(isStylePreviewEnabled()).toBe(false)
  })

  it('is on when REACT_APP_STYLE_PREVIEW is "true"', () => {
    window.env.REACT_APP_STYLE_PREVIEW = 'true'
    expect(isStylePreviewEnabled()).toBe(true)
  })

  it('is off for any other value', () => {
    window.env.REACT_APP_STYLE_PREVIEW = 'false'
    expect(isStylePreviewEnabled()).toBe(false)
  })
})
