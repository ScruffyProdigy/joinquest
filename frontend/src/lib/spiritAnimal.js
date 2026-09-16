import { graphqlRequest, isAuthRequiredError, isTransientServerError } from './graphql'

export const MY_SPIRIT_ANIMAL_JOURNEY_ELIGIBILITY = `
  query MySpiritAnimalJourneyEligibility {
    mySpiritAnimalJourneyEligibility {
      canBegin
      cooldownEndsAt
      daysRemaining
    }
  }
`

export async function fetchSpiritAnimalJourneyEligibility() {
  const data = await graphqlRequest(MY_SPIRIT_ANIMAL_JOURNEY_ELIGIBILITY)
  return data.mySpiritAnimalJourneyEligibility
}

export function formatSpiritAnimalJourneyCooldown(daysRemaining, cooldownEndsAt) {
  if (daysRemaining != null && daysRemaining > 0) {
    return daysRemaining === 1
      ? 'Your next spirit animal journey opens in 1 day.'
      : `Your next spirit animal journey opens in ${daysRemaining} days.`
  }
  if (cooldownEndsAt) {
    const date = new Date(cooldownEndsAt)
    if (!Number.isNaN(date.getTime())) {
      return `Your next spirit animal journey opens on ${date.toLocaleDateString(undefined, { month: 'long', day: 'numeric', year: 'numeric' })}.`
    }
  }
  return 'Your next spirit animal journey is not available yet.'
}

const SPIRIT_ANIMAL_READING_FIELDS = `
  id
  status
  draw
  errorMessage
  cardQuestions {
    slot
    slotName
    card
    cardMeaningInGeneral
    cardMeaningForSlot
    question
    answers { id label }
  }
  personality {
    overview
    journeySummary { compass coin storm campfire beacon }
    coreThemes
    strengths
    socialIdentity
  }
  mascotOverview
  totems {
    name
    animal
    personalitySummary
    imageUrl
    fitScore
    affinity
    readingEmphasis
    whyChooseThisAvatar
  }
  selectedTotemName
  imagesMissing
  phaseStartedAt
  estimatedPhaseSeconds
`

export const MY_SPIRIT_ANIMAL_READING = `
  query MySpiritAnimalReading {
    mySpiritAnimalReading {
      ${SPIRIT_ANIMAL_READING_FIELDS}
    }
  }
`

const BEGIN_SPIRIT_ANIMAL_READING = `
  mutation BeginSpiritAnimalReading($forceRestart: Boolean) {
    beginSpiritAnimalReading(forceRestart: $forceRestart) {
      ${SPIRIT_ANIMAL_READING_FIELDS}
    }
  }
`

const SUBMIT_SPIRIT_ANIMAL_ANSWERS = `
  mutation SubmitSpiritAnimalAnswers($answers: [ID!]!) {
    submitSpiritAnimalAnswers(answers: $answers) {
      ${SPIRIT_ANIMAL_READING_FIELDS}
    }
  }
`

const REGENERATE_SPIRIT_ANIMAL_IMAGES = `
  mutation RegenerateSpiritAnimalImages {
    regenerateSpiritAnimalImages {
      ${SPIRIT_ANIMAL_READING_FIELDS}
    }
  }
`

const SELECT_SPIRIT_ANIMAL_TOTEM = `
  mutation SelectSpiritAnimalTotem($totemName: String!) {
    selectSpiritAnimalTotem(totemName: $totemName) {
      id
      email
      displayName
      avatarUrl
      avatarKey
      avatarSource
      createdAt
    }
  }
`

export async function fetchMySpiritAnimalReading() {
  const data = await graphqlRequest(MY_SPIRIT_ANIMAL_READING)
  return data.mySpiritAnimalReading
}

export async function fetchMySpiritAnimalReadingWithRetry() {
  return withFetchRetries(() => fetchMySpiritAnimalReading())
}

export async function beginSpiritAnimalReading({ forceRestart = false } = {}) {
  return withFetchRetries(async () => {
    const data = await graphqlRequest(BEGIN_SPIRIT_ANIMAL_READING, { forceRestart })
    return data.beginSpiritAnimalReading
  })
}

export async function submitSpiritAnimalAnswers(answers) {
  const data = await graphqlRequest(SUBMIT_SPIRIT_ANIMAL_ANSWERS, { answers })
  return data.submitSpiritAnimalAnswers
}

export async function regenerateSpiritAnimalImages() {
  const data = await graphqlRequest(REGENERATE_SPIRIT_ANIMAL_IMAGES)
  return data.regenerateSpiritAnimalImages
}

export async function selectSpiritAnimalTotem(totemName) {
  const data = await graphqlRequest(SELECT_SPIRIT_ANIMAL_TOTEM, { totemName })
  return data.selectSpiritAnimalTotem
}

export function isSpiritAnimalProcessing(status) {
  return status === 'GENERATING_QUESTIONS' || status === 'PROCESSING'
}

export function isSpiritAnimalFailed(status) {
  return status === 'FAILED'
}

export function secondsRemainingFromPhase(phaseStartedAt, estimatedPhaseSeconds) {
  if (!phaseStartedAt || !estimatedPhaseSeconds) {
    return null
  }
  const started = new Date(phaseStartedAt).getTime()
  if (Number.isNaN(started)) {
    return null
  }
  const elapsed = Math.floor((Date.now() - started) / 1000)
  return Math.max(0, estimatedPhaseSeconds - elapsed)
}

export function formatPhaseCountdown(remainingSeconds) {
  if (remainingSeconds == null) {
    return null
  }
  if (remainingSeconds <= 0) {
    return 'Almost there…'
  }
  if (remainingSeconds === 1) {
    return 'About 1 second left'
  }
  if (remainingSeconds < 60) {
    return `About ${remainingSeconds} seconds left`
  }
  const minutes = Math.ceil(remainingSeconds / 60)
  return minutes === 1 ? 'About 1 minute left' : `About ${minutes} minutes left`
}

async function withFetchRetries(fn, { attempts = 4, delayMs = 750 } = {}) {
  let lastErr
  for (let attempt = 0; attempt < attempts; attempt += 1) {
    try {
      return await fn()
    } catch (err) {
      lastErr = err
      if (!isTransientServerError(err.message) || attempt === attempts - 1) {
        throw err
      }
      await new Promise((resolve) => setTimeout(resolve, delayMs * (attempt + 1)))
    }
  }
  throw lastErr
}

export function isSpiritAnimalAuthError(message) {
  return isAuthRequiredError(message)
}

export function friendlySpiritAnimalError(message) {
  const text = message?.trim() || ''
  if (isAuthRequiredError(text)) {
    return 'Your session expired — sign in again to continue.'
  }
  if (isTransientServerError(text)) {
    return 'The server is briefly unavailable — wait a few seconds, then try again.'
  }
  if (/next spirit animal journey opens/i.test(text)) {
    return text
  }
  if (/too many reading starts/i.test(text)) {
    return text
  }
  if (/dall-e-3|dall-e-2/i.test(text)) {
    return 'That reading used a retired image model. Tap Begin reading to start fresh.'
  }
  if (/permission denied/i.test(text)) {
    return 'Could not save mascot images on the server. Try again in a moment.'
  }
  if (/quota|429|billing/i.test(text)) {
    return 'OpenAI quota exceeded — add billing or credits at platform.openai.com, then try again.'
  }
  if (/expected 5 answers|invalid questions/i.test(text)) {
    return 'We could not parse the tarot questions. Tap Try again to draw a fresh reading.'
  }
  if (/timed out|deadline exceeded/i.test(text)) {
    return 'Mascot generation is taking longer than expected. Try again in a moment.'
  }
  if (/server_error|http 500|http 502|http 503|upstream connect error/i.test(text)) {
    return 'OpenAI had a temporary hiccup while drawing your mascots. Tap Start over to try again.'
  }
  return text || 'Something went wrong while summoning your mascots.'
}

export async function pollSpiritAnimalReading(
  fetchReading,
  { intervalMs = 2000, timeoutMs = 600000 } = {},
) {
  const started = Date.now()
  for (;;) {
    if (Date.now() - started > timeoutMs) {
      throw new Error('Spirit animal reading timed out')
    }
    const latest = await withFetchRetries(() => fetchReading())
    if (!latest || !isSpiritAnimalProcessing(latest.status)) {
      return latest
    }
    await new Promise((resolve) => setTimeout(resolve, intervalMs))
  }
}
