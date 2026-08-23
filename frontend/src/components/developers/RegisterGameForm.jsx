import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { useAuth } from '../auth/AuthProvider'
import {
  developerWelcomePath,
  registerMyGame,
  suggestSlugFromName,
} from '../../lib/developers'
import {
  clearRegistrationDraft,
  readRegistrationDraft,
  saveRegistrationDraft,
} from '../../lib/developerDraft'
import { navigateTo } from '../../lib/usePathname'

export default function RegisterGameForm({ onAccountRequired }) {
  const { user } = useAuth()
  const [draft] = useState(() => readRegistrationDraft())
  const [name, setName] = useState(draft?.name ?? '')
  const [slug, setSlug] = useState(draft?.slug ?? '')
  // A restored slug is already the developer's choice — don't re-derive it from the name.
  const [slugTouched, setSlugTouched] = useState(Boolean(draft?.slug))
  const [shortDescription, setShortDescription] = useState(draft?.shortDescription ?? '')
  const [apiBaseUrl, setApiBaseUrl] = useState(draft?.apiBaseUrl ?? '')
  const [contactEmail, setContactEmail] = useState(draft?.contactEmail ?? user?.email ?? '')
  const [websiteUrl, setWebsiteUrl] = useState(draft?.websiteUrl ?? '')
  const [communityUrl, setCommunityUrl] = useState(draft?.communityUrl ?? '')
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')
  const [connectError, setConnectError] = useState('')

  // Registering a game needs a durable identity; guests and signed-out visitors get prompted.
  const needsAccount = !user || user.isGuest

  const fields = {
    name,
    slug,
    shortDescription,
    apiBaseUrl,
    contactEmail,
    websiteUrl,
    communityUrl,
  }

  useEffect(() => {
    if (!slugTouched) {
      setSlug(suggestSlugFromName(name))
    }
  }, [name, slugTouched])

  useEffect(() => {
    if (user?.email && !contactEmail) {
      setContactEmail(user.email)
    }
  }, [user?.email, contactEmail])

  useEffect(() => {
    // Keep the in-progress form recoverable if signing up navigates the page away.
    if (Object.values(fields).some((value) => value.trim() !== '')) {
      saveRegistrationDraft(fields)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [name, slug, shortDescription, apiBaseUrl, contactEmail, websiteUrl, communityUrl])

  async function handleSubmit(event) {
    event.preventDefault()
    if (status === 'loading') {
      return
    }

    if (needsAccount) {
      saveRegistrationDraft(fields)
      onAccountRequired?.()
      return
    }

    setStatus('loading')
    setError('')
    setConnectError('')

    try {
      const result = await registerMyGame({
        name: name.trim(),
        slug: slug.trim(),
        shortDescription: shortDescription.trim(),
        apiBaseUrl: apiBaseUrl.trim(),
        contactEmail: contactEmail.trim(),
        websiteUrl: websiteUrl.trim() || null,
        communityUrl: communityUrl.trim() || null,
      })
      if (!result.connected && result.connectError) {
        setConnectError(result.connectError)
      }
      clearRegistrationDraft()
      navigateTo(developerWelcomePath(result.game.id))
    } catch (err) {
      setError(err.message || 'Could not register game.')
      setStatus('idle')
    }
  }

  return (
    <form className="developer-form" onSubmit={(event) => void handleSubmit(event)}>
      <div className="developer-form__field">
        <label htmlFor="dev-game-name">Game name</label>
        <input
          id="dev-game-name"
          type="text"
          required
          value={name}
          onChange={(event) => setName(event.target.value)}
        />
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-game-slug">Slug</label>
        <input
          id="dev-game-slug"
          type="text"
          required
          pattern="[a-z0-9]+(?:-[a-z0-9]+)*"
          title="Lowercase letters, numbers, and hyphens only"
          value={slug}
          onChange={(event) => {
            setSlugTouched(true)
            setSlug(event.target.value)
          }}
        />
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-short-description">Short description</label>
        <textarea
          id="dev-short-description"
          required
          rows={3}
          value={shortDescription}
          onChange={(event) => setShortDescription(event.target.value)}
        />
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-api-base">API base URL</label>
        <input
          id="dev-api-base"
          type="url"
          required
          placeholder="https://mygame.example.com"
          value={apiBaseUrl}
          onChange={(event) => setApiBaseUrl(event.target.value)}
        />
        <p className="developer-form__hint">Must be a public HTTPS URL — localhost will not work.</p>
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-contact-email">Contact email</label>
        <input
          id="dev-contact-email"
          type="email"
          required
          value={contactEmail}
          onChange={(event) => setContactEmail(event.target.value)}
        />
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-website">Website (optional)</label>
        <input
          id="dev-website"
          type="url"
          value={websiteUrl}
          onChange={(event) => setWebsiteUrl(event.target.value)}
        />
      </div>

      <div className="developer-form__field">
        <label htmlFor="dev-community">Discord or community link (optional)</label>
        <input
          id="dev-community"
          type="url"
          value={communityUrl}
          onChange={(event) => setCommunityUrl(event.target.value)}
        />
      </div>

      {error ? (
        <p className="status-message status-message-error" role="alert">
          {error}
        </p>
      ) : null}
      {connectError ? (
        <p className="status-message status-message-error" role="alert">
          {connectError}
        </p>
      ) : null}

      <Button type="submit" variant="default" disabled={status === 'loading'}>
        {status === 'loading' ? 'Registering…' : 'Register game'}
      </Button>
    </form>
  )
}
