import { useEffect, useState } from 'react'
import {
  SIGN_IN_BENEFITS,
  SIGN_IN_BENEFITS_LABEL,
  signInBenefitDotLabel,
} from '../../lib/playerCopy'

/** The prototype's own cadence. */
const ROTATE_MS = 4000

function prefersReducedMotion() {
  return Boolean(window.matchMedia?.('(prefers-reduced-motion: reduce)')?.matches)
}

/**
 * A 6px position indicator, not an action with a label, so it is a bare
 * <button> rather than a `Button` -- the pill geometry and padding `Button`
 * exists to give every action are exactly what a dot must not have. Named and
 * kept here for the same reason TabButton and RouteCard are: an anonymous
 * <button> mid-render is a missing component, not an exception.
 */
function BenefitDot({ index, current, onSelect }) {
  return (
    <button
      type="button"
      className="sign-in-benefits__dot"
      aria-label={signInBenefitDotLabel(index, SIGN_IN_BENEFITS.length)}
      aria-current={current ? 'true' : undefined}
      onClick={onSelect}
    />
  )
}

/**
 * The case for an account, one reason at a time (JQ-249). The prototype rotates
 * four cards on a timer; this keeps the rotation but adds what auto-moving
 * content needs to be usable:
 *
 * - it stops on hover and on keyboard focus, and for good once a dot is picked,
 *   so a reader is never racing the timer (WCAG 2.2.2);
 * - it announces politely only while it is *not* rotating on its own, so a
 *   screen reader is not interrupted every four seconds;
 * - it does not rotate at all under `prefers-reduced-motion`.
 *
 * Colour is deliberately not the prototype's four hardcoded hexes: brand colour
 * is a token here, never written into a component (JQ-72, JQ-222).
 */
export default function SignInBenefits() {
  const [index, setIndex] = useState(0)
  const [rotating, setRotating] = useState(() => !prefersReducedMotion())
  const [paused, setPaused] = useState(false)

  useEffect(() => {
    if (!rotating || paused) {
      return undefined
    }
    const timer = setInterval(() => {
      setIndex((current) => (current + 1) % SIGN_IN_BENEFITS.length)
    }, ROTATE_MS)
    return () => clearInterval(timer)
  }, [rotating, paused])

  const benefit = SIGN_IN_BENEFITS[index]
  const autoRotating = rotating && !paused

  return (
    <div
      className="sign-in-benefits"
      aria-roledescription="carousel"
      aria-label={SIGN_IN_BENEFITS_LABEL}
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocus={() => setPaused(true)}
      onBlur={() => setPaused(false)}
    >
      <div className="sign-in-benefits__card" aria-live={autoRotating ? 'off' : 'polite'}>
        {/* Keyed so React remounts on every change and the entrance replays. */}
        <div key={benefit.key} className="sign-in-benefits__copy">
          <p className="sign-in-benefits__headline">{benefit.headline}</p>
          <p className="sign-in-benefits__body">{benefit.body}</p>
        </div>
      </div>

      <div className="sign-in-benefits__dots">
        {SIGN_IN_BENEFITS.map((item, dotIndex) => (
          <BenefitDot
            key={item.key}
            index={dotIndex}
            current={dotIndex === index}
            onSelect={() => {
              // Picking a card is a deliberate choice to read it, so the timer
              // stops for good rather than pulling it away four seconds later.
              setRotating(false)
              setIndex(dotIndex)
            }}
          />
        ))}
      </div>
    </div>
  )
}
