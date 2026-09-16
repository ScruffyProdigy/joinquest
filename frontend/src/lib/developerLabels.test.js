import { describe, expect, it } from 'vitest'
import {
  checkFixHint,
  checkSectionTitle,
  checkStatusLabel,
  visibilityLabel,
} from './developerLabels'

describe('visibilityLabel', () => {
  it('maps visibility enums', () => {
    expect(visibilityLabel('PRIVATE_TESTING')).toBe('Private testing')
    expect(visibilityLabel('DRAFT')).toBe('Draft')
  })
})

describe('checkSectionTitle', () => {
  it('groups checks by their id prefix', () => {
    expect(checkSectionTitle('manifest.reach_api')).toBe('Manifest')
    expect(checkSectionTitle('provision.happy_path')).toBe('Provisioning')
    expect(checkSectionTitle('jwt.jwks')).toBe('JWT verification')
    expect(checkSectionTitle('')).toBe('Checks')
  })
})

describe('checkStatusLabel', () => {
  it('maps status enums', () => {
    expect(checkStatusLabel('PASS')).toBe('Pass')
    expect(checkStatusLabel('FAIL')).toBe('Fail')
    expect(checkStatusLabel('SKIP')).toBe('Skipped')
  })
})

describe('checkFixHint', () => {
  it('falls back to the guide for an unknown check id', () => {
    expect(checkFixHint('manifest.reach_api')).toContain('public HTTPS API')
    expect(checkFixHint('not.a.real.check')).toBe(
      'See the integration guide below for details.',
    )
  })
})
