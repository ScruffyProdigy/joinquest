import { APP_NAME } from '../../lib/brand'
import AppFooter from './AppFooter'
import { Link } from '../ui/link'

export default function LegalPage({ title, children }) {
  return (
    <main className="app-shell legal-page">
      <header className="legal-page__header">
        <p className="legal-page__back">
          <Link href="/">
            ← Back to {APP_NAME}
          </Link>
        </p>
        <h1>{title}</h1>
        <p className="legal-page__updated">Last updated: May 30, 2026</p>
      </header>

      <article className="panel-card legal-page__body">{children}</article>

      <AppFooter />
    </main>
  )
}
