import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest'
import {
  DEVELOPER_DRAFT_KEY,
  clearRegistrationDraft,
  readRegistrationDraft,
  saveRegistrationDraft,
} from './developerDraft'

const draft = {
  name: 'Word Hunt',
  slug: 'word-hunt',
  shortDescription: 'Find the words fastest.',
  apiBaseUrl: 'https://api.wordhunt.example.com',
  contactEmail: 'dev@example.com',
  websiteUrl: '',
  communityUrl: '',
}

describe('developerDraft', () => {
  beforeEach(() => {
    sessionStorage.clear()
  })

  afterEach(() => {
    sessionStorage.clear()
    vi.restoreAllMocks()
  })

  it('round-trips a registration draft through sessionStorage', () => {
    saveRegistrationDraft(draft)

    expect(readRegistrationDraft()).toEqual(draft)
  })

  it('returns null when nothing has been saved', () => {
    expect(readRegistrationDraft()).toBeNull()
  })

  it('clearRegistrationDraft removes the stored draft', () => {
    saveRegistrationDraft(draft)

    clearRegistrationDraft()

    expect(sessionStorage.getItem(DEVELOPER_DRAFT_KEY)).toBeNull()
    expect(readRegistrationDraft()).toBeNull()
  })

  it('returns null when the stored draft is not valid JSON', () => {
    sessionStorage.setItem(DEVELOPER_DRAFT_KEY, '{not json')

    expect(readRegistrationDraft()).toBeNull()
  })

  it('returns null when the stored draft is not an object', () => {
    sessionStorage.setItem(DEVELOPER_DRAFT_KEY, '"just a string"')

    expect(readRegistrationDraft()).toBeNull()
  })

  it('saveRegistrationDraft swallows storage failures', () => {
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError')
    })

    expect(() => saveRegistrationDraft(draft)).not.toThrow()
  })
})
