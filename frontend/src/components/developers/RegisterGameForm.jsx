import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import { useAuth } from '../auth/AuthProvider'
import {
  developerWelcomePath,
  registerMyGame,
  suggestSlugFromName,
} from '../../lib/developers'
import { navigateTo } from '../../lib/usePathname'

export default function RegisterGameForm() {
  const { user } = useAuth()
  const [name, setName] = useState('')
  const [slug, setSlug] = useState('')
  const [slugTouched, setSlugTouched] = useState(false)
  const [shortDescription, setShortDescription] = useState('')
  const [apiBaseUrl, setApiBaseUrl] = useState('')
  const [contactEmail, setContactEmail] = useState(user?.email ?? '')
  const [websiteUrl, setWebsiteUrl] = useState('')
  const [communityUrl, setCommunityUrl] = useState('')
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')
  const [connectError, setConnectError] = useState('')

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

  if (!user) {
    return (
      <p className="panel-copy">
        Sign in above to register your game. Registration is free and your game stays private until you
        are ready.
      </p>
    )
  }

  async function handleSubmit(event) {
    event.preventDefault()
    if (status === 'loading') {
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
