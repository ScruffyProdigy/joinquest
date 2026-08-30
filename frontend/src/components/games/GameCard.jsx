import { accentColorFor } from '../../lib/gameAccent'
import {
  gameCatalogHeroUrl,
  gameGenreModeLabel,
  gameLiveActivityLabel,
  gamePagePath,
  gamePlayerCountLabel,
} from '../../lib/gameCard'
import CatalogGameLink from './CatalogGameLink'

const BADGE_CLASS =
  'inline-flex items-center px-2 py-1 rounded-full text-[11px] font-semibold leading-none bg-black/[0.42] backdrop-blur-[6px] text-white/[0.92]'

// Always renders its box, even when empty, so the two groups keep their sides of the
// justify-between row instead of one sliding over when the other has nothing to show.
function CardBadges({ labels, className }) {
  return (
    <div className={className}>
      {labels.filter(Boolean).map((label) => (
        <span key={label} className={BADGE_CLASS}>
          {label}
        </span>
      ))}
    </div>
  )
}

function CardHero({ game, genreMode, playerCount, liveActivity }) {
  const accent = accentColorFor(game.slug || 'default', game.accentColor)
  return (
    <div className="relative overflow-hidden aspect-[5/2]" style={{ background: accent.badge }}>
      <img
        className="absolute inset-0 w-full h-full object-cover"
        src={gameCatalogHeroUrl(game)}
        alt=""
        loading="lazy"
      />
      {/* What the game is on the left, who is on it right now on the right. One row so the
          two groups can never overlap on a narrow card. */}
      <div className="absolute top-2.5 left-2.5 right-2.5 flex items-start justify-between gap-2">
        <CardBadges labels={[genreMode, playerCount]} className="flex flex-wrap gap-1" />
        <CardBadges labels={[liveActivity]} className="flex flex-wrap justify-end gap-1" />
      </div>
    </div>
  )
}

function CardBody({ game }) {
  return (
    <div className="px-4 py-3">
      <h3 className="font-bold text-base text-foreground m-0">{game.name}</h3>
      {game.shortDescription ? (
        <p className="text-sm text-muted-foreground leading-snug line-clamp-2 mt-1 mb-0">
          {game.shortDescription}
        </p>
      ) : null}
    </div>
  )
}

export default function GameCard({ game }) {
  const accent = accentColorFor(game.slug || 'default', game.accentColor)
  const detailPath = gamePagePath(game)
  const genreMode = gameGenreModeLabel(game.tags)
  const playerCount = gamePlayerCountLabel(game.modes)
  const liveActivity = gameLiveActivityLabel(game.playerActivity)

  const shellClassName =
    'block w-full rounded-2xl overflow-hidden text-left shadow-sm transition-transform hover:-translate-y-0.5 hover:shadow-md active:scale-[0.99] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
  const shellStyle = { border: `1.5px solid ${accent.border}`, background: accent.cardBg }

  const content = (
    <>
      <CardHero
        game={game}
        genreMode={genreMode}
        playerCount={playerCount}
        liveActivity={liveActivity}
      />
      <CardBody game={game} />
    </>
  )

  return (
    <li>
      {detailPath ? (
        <CatalogGameLink slug={game.slug} href={detailPath} className={shellClassName} style={shellStyle}>
          {content}
        </CatalogGameLink>
      ) : (
        <div className={shellClassName} style={shellStyle}>
          {content}
        </div>
      )}
    </li>
  )
}
