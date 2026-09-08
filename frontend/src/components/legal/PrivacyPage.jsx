import { APP_NAME } from '../../lib/brand'
import LegalPage from './LegalPage'
import { Link } from '../ui/link'

export default function PrivacyPage() {
  return (
    <LegalPage title="Privacy Policy">
      <p>
        {APP_NAME} is an early-stage product. This policy explains what we collect, why, and what
        choices you have — without pretending we have a giant legal department.
      </p>

      <h2>What we collect</h2>
      <ul>
        <li>
          <strong>Account information</strong> — such as your email address, display name, and
          sign-in identifiers when you use email magic links or sign in with a provider (for
          example Discord or Google).
        </li>
        <li>
          <strong>Usage information</strong> — such as games you view, rooms you join, and basic
          logs needed to operate and debug the service.
        </li>
        <li>
          <strong>Developer information</strong> — if you register a game, we store the details you
          provide (API URLs, descriptions, API keys you generate, integration check results).
        </li>
        <li>
          <strong>Technical data</strong> — such as IP address, browser type, and cookies used for
          session management and security.
        </li>
      </ul>

      <h2>How we use it</h2>
      <p>
        We use this information to run {APP_NAME}: authenticate you, match you with games and
        players, host developer tools, prevent abuse, and improve the product. We do not sell your
        personal information.
      </p>

      <h2>Sharing</h2>
      <p>
        We may share data with service providers that help us host and operate the site (for
        example cloud hosting or email delivery). When you play a third-party game, its developer may
        receive information needed to provision a match — such as a player identifier or seat token
        — according to their own practices.
      </p>
      <p>
        We may also disclose information if required by law or to protect the safety and integrity
        of the service.
      </p>

      <h2>Retention</h2>
      <p>
        We keep information for as long as your account exists or as needed to operate the service,
        comply with legal obligations, and resolve disputes. We may delete inactive data over time
        as the product matures.
      </p>

      <h2>Your choices</h2>
      <p>
        You can sign out, disconnect linked sign-in providers from your account settings, and
        contact us to ask about your data. Because we&apos;re early, some self-service deletion
        options may not exist yet — email us and we&apos;ll help.
      </p>

      <h2>Security</h2>
      <p>
        We take reasonable steps to protect your information, but no online service is perfectly
        secure. Use a strong, unique sign-in method and report concerns to us.
      </p>

      <h2>Children</h2>
      <p>
        {APP_NAME} is not directed at children under 13, and we do not knowingly collect personal
        information from them.
      </p>

      <h2>Changes</h2>
      <p>
        We may update this policy as the product evolves. Continued use after changes means you
        accept the updated policy. Check the &ldquo;Last updated&rdquo; date at the top of this
        page.
      </p>

      <h2>Contact</h2>
      <p>
        Privacy questions? Email{' '}
        <Link href="mailto:support@joinquest.cc">
          support@joinquest.cc
        </Link>
        .
      </p>
    </LegalPage>
  )
}
