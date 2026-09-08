import { APP_NAME } from '../../lib/brand'
import LegalPage from './LegalPage'
import { Link } from '../ui/link'

export default function TermsPage() {
  return (
    <LegalPage title="Terms of Service">
      <p>
        {APP_NAME} is an early-stage product. These terms are short on purpose — they describe how
        the service works today, not a long list of promises.
      </p>

      <h2>The service</h2>
      <p>
        {APP_NAME} helps people find groups and play multiplayer games together. We also offer tools
        for developers to connect games to the platform. Features, availability, and pricing may
        change, and things may break while we&apos;re still building.
      </p>

      <h2>Your account</h2>
      <p>
        You may need an account to use parts of the site. Keep your sign-in methods secure. You are
        responsible for activity under your account. We may suspend or remove accounts that abuse
        the service or violate these terms.
      </p>

      <h2>Third-party games</h2>
      <p>
        Games on {APP_NAME} are built and operated by their developers, not by us. We do not control
        game content, uptime, or how a developer handles your data inside their game. Play at your
        own discretion.
      </p>

      <h2>Acceptable use</h2>
      <p>
        Do not use {APP_NAME} for anything illegal, harmful, or disruptive — including harassment,
        cheating, attempts to break or overload the service, or scraping beyond normal use.
      </p>

      <h2>No warranties</h2>
      <p>
        {APP_NAME} is provided &ldquo;as is&rdquo; and &ldquo;as available.&rdquo; We do not
        guarantee uninterrupted access, error-free operation, or that the service will meet your
        expectations.
      </p>

      <h2>Limitation of liability</h2>
      <p>
        To the fullest extent permitted by law, {APP_NAME} and its operators are not liable for
        indirect, incidental, or consequential damages arising from your use of the service.
      </p>

      <h2>Changes</h2>
      <p>
        We may update these terms as the product evolves. Continued use after changes means you
        accept the updated terms. Material changes will be reflected on this page with a new
        &ldquo;Last updated&rdquo; date.
      </p>

      <h2>Contact</h2>
      <p>
        Questions about these terms? Email{' '}
        <Link href="mailto:support@joinquest.cc">
          support@joinquest.cc
        </Link>
        .
      </p>
    </LegalPage>
  )
}
