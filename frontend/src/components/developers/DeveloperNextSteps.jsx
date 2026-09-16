import { integrationNextSteps } from '../../lib/developers'

export default function DeveloperNextSteps({ game }) {
  const steps = integrationNextSteps(game)
  if (steps.length === 0) {
    return null
  }

  return (
    <section className="panel-card developer-next-steps" aria-labelledby="next-steps-heading">
      <h2 id="next-steps-heading">Next steps</h2>
      <ol className="developer-next-steps__list">
        {steps.map((step) => (
          <li
            key={step.id}
            className={`developer-next-steps__item developer-next-steps__item--${step.status}`}
          >
            <span className="developer-next-steps__label">{step.label}</span>
            {step.status === 'current' ? (
              <span className="developer-next-steps__badge">Current</span>
            ) : null}
            {step.status === 'done' ? (
              <span className="developer-next-steps__badge developer-next-steps__badge--done">
                Done
              </span>
            ) : null}
            <p className="developer-next-steps__hint">{step.hint}</p>
          </li>
        ))}
      </ol>
    </section>
  )
}
