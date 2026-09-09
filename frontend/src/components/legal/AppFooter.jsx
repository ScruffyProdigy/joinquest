import { Link } from '../ui/link'

export default function AppFooter() {
  return (
    <footer className="app-footer">
      <nav className="app-footer__nav" aria-label="Legal">
        <Link href="/terms">
          Terms of Service
        </Link>
        <span className="app-footer__sep" aria-hidden="true">
          ·
        </span>
        <Link href="/privacy">
          Privacy Policy
        </Link>
      </nav>
    </footer>
  )
}
