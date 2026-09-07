import '@testing-library/jest-dom'
import { configure } from '@testing-library/react'
import { beforeEach, vi } from 'vitest'
import { clearSubscriptionAuthCache } from '../lib/queue'

// findBy*/waitFor default to 1s, which is too tight for the catalog's fetch chain
// once coverage instrumentation slows things down, so the suite flakes in CI.
configure({ asyncUtilTimeout: 5000 })

vi.mock('graphql-ws', () => ({
  createClient: vi.fn(() => ({
    subscribe: vi.fn(() => () => {}),
    dispose: vi.fn(),
  })),
}))

// Mock window.env for testing
Object.defineProperty(window, 'env', {
  writable: true,
  configurable: true,
  value: {
    REACT_APP_ENV: 'test',
    REACT_APP_API_BASE_URL: '',
  },
})

// Mock window.matchMedia
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
})

global.IntersectionObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

global.ResizeObserver = vi.fn().mockImplementation(() => ({
  observe: vi.fn(),
  unobserve: vi.fn(),
  disconnect: vi.fn(),
}))

export const mockDemoGames = [
  {
    id: 'game-1',
    slug: 'rock-paper-scissors-lizard-robot',
    name: 'Rock Paper Scissors Lizard Robot',
    iconUrl: '/games/rpslr-icon.png',
    heroUrl: '/games/rpslr-hero.jpg',
    shortDescription: 'Best-of-five duel with move cooldown.',
    tags: ['competitive', '1v1'],
    createdAt: '2026-01-01T00:00:00Z',
    activeSessions: [{ id: 'session-1' }],
    modes: [
      {
        modeKey: 'duel',
        seats: [{ queuePath: null }, { queuePath: null }],
        queues: [{ id: 'queue-1', name: 'Default', playersToStart: 2, status: 'active' }],
      },
    ],
  },
]

const defaultQueueStatus = { queued: false, queuedCount: 0 }

function createFetchMock(handlers) {
  return vi.fn(async (_url, init) => {
    const body = JSON.parse(init?.body ?? '{}')
    const query = body.query ?? ''
    let data = {}

    if (query.includes('createGuestSession')) {
      data = {
        createGuestSession: handlers.me ?? {
          id: 'guest-1',
          email: null,
          displayName: null,
          isGuest: true,
          createdAt: '2026-01-01T00:00:00Z',
        },
      }
    } else if (query.includes('requestSignIn')) {
      data = { requestSignIn: true }
    } else if (query.includes('completeSignInWithCode')) {
      data = { completeSignInWithCode: handlers.me ?? null }
    } else if (query.includes('completeSignInWithLink')) {
      data = { completeSignInWithLink: handlers.me ?? null }
    } else if (query.includes('logout')) {
      data = { logout: true }
    } else if (query.includes('subscriptionAuth')) {
      data = { subscriptionAuth: handlers.subscriptionAuth ?? 'Bearer test-token' }
    } else if (query.includes('myActiveIntent')) {
      data = { myActiveIntent: handlers.myActiveIntent ?? null }
    } else if (query.includes('myQueueStatus')) {
      data = { myQueueStatus: handlers.myQueueStatus ?? defaultQueueStatus }
    } else if (query.includes('games {')) {
      data = { games: handlers.games ?? [] }
    } else if (query.includes('starterAvatars')) {
      data = { starterAvatars: handlers.starterAvatars ?? [] }
    } else if (query.includes('mySpiritAnimalJourneyEligibility')) {
      data = {
        mySpiritAnimalJourneyEligibility: handlers.mySpiritAnimalJourneyEligibility ?? {
          canBegin: true,
          cooldownEndsAt: null,
          daysRemaining: null,
        },
      }
    } else if (query.includes('mySpiritAnimalReading')) {
      data = { mySpiritAnimalReading: handlers.mySpiritAnimalReading ?? null }
    } else if (query.includes('beginSpiritAnimalReading')) {
      data = { beginSpiritAnimalReading: handlers.beginSpiritAnimalReading ?? null }
    } else if (query.includes('submitSpiritAnimalAnswers')) {
      data = { submitSpiritAnimalAnswers: handlers.submitSpiritAnimalAnswers ?? null }
    } else if (query.includes('selectSpiritAnimalTotem')) {
      data = {
        selectSpiritAnimalTotem: handlers.selectSpiritAnimalTotem ?? {
          ...handlers.me,
          avatarSource: 'SPIRIT_ANIMAL',
          avatarUrl: '/spirit-avatars/mock/ember-fox.png',
          avatarKey: null,
        },
      }
    } else if (query.includes('updatePlayerProfile')) {
      data = {
        updatePlayerProfile: handlers.updatePlayerProfile ?? {
          ...handlers.me,
          displayName: body.variables?.displayName ?? handlers.me?.displayName,
          // Saving through this mutation is what marks a name as chosen.
                avatarKey: body.variables?.avatarKey ?? handlers.me?.avatarKey,
        },
      }
    } else if (/\bme\s*\{/.test(query) || query.includes('query Me')) {
      // Word boundary matters: a plain includes('me {') also matches "game {", so any
      // room query carrying tables { game { … } } was answered as `me` and never
      // returned a room.
      data = { me: handlers.me ?? null }
    } else if (query.includes('joinQueue')) {
      data = { joinQueue: handlers.joinQueue ?? { queued: true, queuedCount: 1 } }
    } else if (query.includes('leaveQueue')) {
      data = { leaveQueue: true }
    } else if (query.includes('myRoom')) {
      data = { myRoom: handlers.myRoom ?? null }
    } else if (query.includes('createRoom')) {
      data = { createRoom: handlers.createRoom ?? null }
    } else {
      data = handlers.fallback ?? {}
    }

    return {
      ok: true,
      json: async () => ({ data }),
    }
  })
}

export function mockGraphQLResponse(data) {
  global.fetch = vi.fn().mockResolvedValue({
    ok: true,
    json: async () => ({ data }),
  })
}

export function mockUnauthenticatedSession(overrides = {}) {
  global.fetch = createFetchMock({ me: null, games: [], ...overrides })
}

export function mockAuthenticatedSession(
  user = {
    id: 'user-1',
    email: 'player@example.com',
    displayName: 'player',
    avatarKey: 'compass',
    avatarUrl: '/avatars/compass.png',
    avatarSource: 'STARTER',
    isGuest: false,
    createdAt: '2026-01-01T00:00:00Z',
  },
  overrides = {},
) {
  global.fetch = createFetchMock({
    me: user,
    games: mockDemoGames,
    subscriptionAuth: 'Bearer test-token',
    myQueueStatus: defaultQueueStatus,
    myActiveIntent: null,
    myRoom: null,
    ...overrides,
  })
}

beforeEach(() => {
  clearSubscriptionAuthCache()
})
