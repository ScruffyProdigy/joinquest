import { GREETING_AFTERNOON, GREETING_EVENING, GREETING_MORNING } from './playerCopy'

/** Afternoon starts at noon, evening at 6pm — the prototype's cutoffs. */
const AFTERNOON_START_HOUR = 12
const EVENING_START_HOUR = 18

/**
 * The greeting on its own, with no name attached. `now` is injectable so tests
 * can pick an hour instead of waiting for one.
 */
export function timeOfDayGreeting(now = new Date()) {
  const hour = now.getHours()
  if (hour < AFTERNOON_START_HOUR) {
    return GREETING_MORNING
  }
  if (hour < EVENING_START_HOUR) {
    return GREETING_AFTERNOON
  }
  return GREETING_EVENING
}

/**
 * The full home greeting. A visitor we have no name for just gets the greeting,
 * so the line never reads "Good afternoon, ".
 */
export function homeGreetingLine(displayName, now = new Date()) {
  const greeting = timeOfDayGreeting(now)
  const name = displayName?.trim()
  return name ? `${greeting}, ${name}` : greeting
}
