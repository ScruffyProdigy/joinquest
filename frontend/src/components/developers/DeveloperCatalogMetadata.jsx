import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import {
  fetchCatalogTagTaxonomy,
  updateMyGameMetadata,
} from '../../lib/developers'
import { parseHex } from '../../lib/gameAccent'

// Neutral placeholder for the swatch when accentColor doesn't parse yet
// (empty, mid-typing, or invalid) — matches the palette's "indigo" entry.
const SWATCH_FALLBACK_COLOR = '#6366f1'

export default function DeveloperCatalogMetadata({ game, onSaved }) {
  const [name, setName] = useState(game?.name ?? '')
  const [shortDescription, setShortDescription] = useState(game?.shortDescription ?? '')
  const [longDescription, setLongDescription] = useState(game?.longDescription ?? '')
  const [howToPlay, setHowToPlay] = useState(game?.howToPlay ?? '')
  const [contactEmail, setContactEmail] = useState(game?.contactEmail ?? '')
  const [websiteUrl, setWebsiteUrl] = useState(game?.websiteUrl ?? '')
  const [communityUrl, setCommunityUrl] = useState(game?.communityUrl ?? '')
  const [selectedTags, setSelectedTags] = useState(game?.tags ?? [])
  const [accentColor, setAccentColor] = useState(game?.accentColor ?? '')
  const [taxonomy, setTaxonomy] = useState([])
  const [status, setStatus] = useState('idle')
  const [error, setError] = useState('')

  useEffect(() => {
    setName(game?.name ?? '')
    setShortDescription(game?.shortDescription ?? '')
    setLongDescription(game?.longDescription ?? '')
    setHowToPlay(game?.howToPlay ?? '')
    setContactEmail(game?.contactEmail ?? '')
    setWebsiteUrl(game?.websiteUrl ?? '')
    setCommunityUrl(game?.communityUrl ?? '')
    setSelectedTags(game?.tags ?? [])
    setAccentColor(game?.accentColor ?? '')
  }, [game])

  useEffect(() => {
    void fetchCatalogTagTaxonomy()
      .then(setTaxonomy)
      .catch(() => setTaxonomy([]))
  }, [])

  function toggleTag(tagId) {
    setSelectedTags((prev) => {
      if (prev.includes(tagId)) {
        return prev.filter((t) => t !== tagId)
      }
      if (prev.length >= 3) {
        return prev
      }
      return [...prev, tagId]
    })
  }

  async function handleSubmit(event) {
    event.preventDefault()
    if (!game?.id) {
      return
    }
    setStatus('saving')
    setError('')
    try {
      const updated = await updateMyGameMetadata({
        gameId: game.id,
        name: name.trim(),
        shortDescription: shortDescription.trim(),
        longDescription: longDescription.trim(),
        howToPlay: howToPlay.trim(),
        contactEmail: contactEmail.trim(),
        websiteUrl: websiteUrl.trim(),
        communityUrl: communityUrl.trim(),
        tags: selectedTags,
        accentColor: accentColor.trim(),
      })
      onSaved?.(updated)
      setStatus('idle')
    } catch (err) {
      setError(err.message || 'Could not save catalog listing.')
      setStatus('idle')
    }
  }

  return (
    <section className="panel-card" aria-labelledby="catalog-metadata-heading">
      <h2 id="catalog-metadata-heading">Catalog listing</h2>
      <p className="panel-copy">
        Player-facing copy and contact details. Agents can draft these after discovery — edit and
        save here. Slug stays fixed after registration.
      </p>
      <form className="developer-form" onSubmit={(event) => void handleSubmit(event)}>
        <div className="developer-form__field">
          <label htmlFor="dev-game-display-name">Display name</label>
          <input
            id="dev-game-display-name"
            type="text"
            value={name}
            onChange={(event) => setName(event.target.value)}
            required
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-short-description">Short description</label>
          <textarea
            id="dev-short-description"
            rows={2}
            maxLength={200}
            value={shortDescription}
            onChange={(event) => setShortDescription(event.target.value)}
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-long-description">Long description</label>
          <textarea
            id="dev-long-description"
            rows={5}
            value={longDescription}
            onChange={(event) => setLongDescription(event.target.value)}
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-how-to-play">How to play</label>
          <textarea
            id="dev-how-to-play"
            rows={4}
            value={howToPlay}
            onChange={(event) => setHowToPlay(event.target.value)}
            placeholder="Step-by-step for first-time players"
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-contact-email-edit">Contact email</label>
          <input
            id="dev-contact-email-edit"
            type="email"
            value={contactEmail}
            onChange={(event) => setContactEmail(event.target.value)}
            required
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-website-edit">Website (optional)</label>
          <input
            id="dev-website-edit"
            type="url"
            value={websiteUrl}
            onChange={(event) => setWebsiteUrl(event.target.value)}
            placeholder="https://"
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-community-edit">Community URL (optional)</label>
          <input
            id="dev-community-edit"
            type="url"
            value={communityUrl}
            onChange={(event) => setCommunityUrl(event.target.value)}
            placeholder="https://discord.gg/…"
          />
        </div>
        <div className="developer-form__field">
          <label htmlFor="dev-accent-color">Catalog accent color (optional)</label>
          <div className="flex items-center gap-2">
            <input
              id="dev-accent-color"
              type="color"
              value={parseHex(accentColor) ?? SWATCH_FALLBACK_COLOR}
              onChange={(event) => setAccentColor(event.target.value)}
            />
            <input
              type="text"
              aria-label="Accent color hex value"
              value={accentColor}
              onChange={(event) => setAccentColor(event.target.value)}
              placeholder="#7c3aed"
            />
            <Button type="button" variant="ghost" onClick={() => setAccentColor('')}>
              Use default
            </Button>
          </div>
          <p className="panel-copy">
            Tints your catalog card and game page. Leave blank to use a color picked from your slug.
          </p>
        </div>
        {taxonomy.length > 0 ? (
          <fieldset className="developer-form__field developer-tag-picker">
            <legend>Tags (up to 3)</legend>
            <ul className="developer-tag-picker__list">
              {taxonomy.map((tag) => (
                <li key={tag.id}>
                  <label className="developer-tag-picker__option">
                    <input
                      type="checkbox"
                      checked={selectedTags.includes(tag.id)}
                      onChange={() => toggleTag(tag.id)}
                    />
                    <span>{tag.label}</span>
                  </label>
                </li>
              ))}
            </ul>
          </fieldset>
        ) : null}
        <Button type="submit" variant="secondary" disabled={status === 'saving'}>
          {status === 'saving' ? 'Saving…' : 'Save listing'}
        </Button>
        {error ? (
          <p className="status-message status-message-error" role="alert">
            {error}
          </p>
        ) : null}
      </form>
    </section>
  )
}
