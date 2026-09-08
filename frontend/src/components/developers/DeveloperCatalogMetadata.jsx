import { useEffect, useState } from 'react'
import { Button } from '../ui/button'
import {
  fetchCatalogAxisTaxonomy,
  updateMyGameMetadata,
} from '../../lib/developers'
import { accentBaseFor } from '../../lib/gameAccent'

export default function DeveloperCatalogMetadata({ game, onSaved }) {
  const [name, setName] = useState(game?.name ?? '')
  const [shortDescription, setShortDescription] = useState(game?.shortDescription ?? '')
  const [longDescription, setLongDescription] = useState(game?.longDescription ?? '')
  const [howToPlay, setHowToPlay] = useState(game?.howToPlay ?? '')
  const [contactEmail, setContactEmail] = useState(game?.contactEmail ?? '')
  const [websiteUrl, setWebsiteUrl] = useState(game?.websiteUrl ?? '')
  const [communityUrl, setCommunityUrl] = useState(game?.communityUrl ?? '')
  const [genre, setGenre] = useState(game?.genre ?? '')
  const [difficulty, setDifficulty] = useState(game?.difficulty ?? '')
  const [accentColor, setAccentColor] = useState(game?.accentColor ?? '')
  const [taxonomy, setTaxonomy] = useState({ genre: [], difficulty: [] })
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
    setGenre(game?.genre ?? '')
    setDifficulty(game?.difficulty ?? '')
    setAccentColor(game?.accentColor ?? '')
  }, [game])

  useEffect(() => {
    void fetchCatalogAxisTaxonomy()
      .then(setTaxonomy)
      .catch(() => setTaxonomy({ genre: [], difficulty: [] }))
  }, [])

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
        genre,
        difficulty,
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
              // Falls back to the game's hashed default, so the swatch always
              // shows the color the card actually renders in.
              value={accentBaseFor(game?.slug, accentColor)}
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
        {taxonomy.genre.length > 0 ? (
          <div className="developer-form__field">
            <label htmlFor="dev-game-genre">Genre</label>
            <select id="dev-game-genre" value={genre} onChange={(event) => setGenre(event.target.value)}>
              <option value="">No genre</option>
              {taxonomy.genre.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="panel-copy">
              One per game — it is the label on your catalog card and the filter players browse by.
            </p>
          </div>
        ) : null}
        {taxonomy.difficulty.length > 0 ? (
          <div className="developer-form__field">
            <label htmlFor="dev-game-difficulty">Difficulty (optional)</label>
            <select
              id="dev-game-difficulty"
              value={difficulty}
              onChange={(event) => setDifficulty(event.target.value)}
            >
              <option value="">Not declared</option>
              {taxonomy.difficulty.map((option) => (
                <option key={option.id} value={option.id}>
                  {option.label}
                </option>
              ))}
            </select>
            <p className="panel-copy">
              How much a new player needs to know before their first round is any fun.
            </p>
          </div>
        ) : null}
        <p className="panel-copy">
          Each mode's social shape (1v1, teams, co-op…) comes from your game-modes manifest, not
          this form — modes of the same game often differ.
        </p>
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
