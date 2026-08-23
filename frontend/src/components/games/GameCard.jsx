import { accentColorFor } from '../../lib/gameAccent'
import {
  gameCatalogHeroUrl,
  gameGenreModeLabel,
  gamePagePath,
  gamePlayerCountLabel,
} from '../../lib/gameCard'
import CatalogGameLink from './CatalogGameLink'

const BADGE_CLASS =
  'inline-flex items-center px-2 py-1 rounded-full text-[11px] font-semibold leading-none bg-black/[0.42] backdrop-blur-[6px] text-white/[0.92]'

function CardBadges({ genreMode, playerCount }) {
  if (!genreMode && !playerCount) {
    return null
  }
  return (
    <div className="absolute top-2.5 left-2.5 flex flex-wrap gap-1">
      {genreMode ? <span className={BADGE_CLASS}>{genreMode}</span> : null}
      {playerCount ? <span className={BADGE_CLASS}>{playerCount}</span> : null}
    </div>
  )
}

function CardHero({ game, genreMode, playerCount }) {
  const accent = accentColorFor(game.slug || 'default', game.accentColor)
  return (
    <div className="relative overflow-hidden aspect-[5/2]" style={{ background: accent.badge }}>
      <img
        className="absolute inset-0 w-full h-full object-cover"
        src={gameCatalogHeroUrl(game)}
        alt=""
        loading="lazy"
      />
      <CardBadges genreMode={genreMode} playerCount={playerCount} />
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

  const shellClassName =
    'block w-full rounded-2xl overflow-hidden text-left shadow-sm transition-transform hover:-translate-y-0.5 hover:shadow-md active:scale-[0.99] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'
  const shellStyle = { border: `1.5px solid ${accent.border}`, background: accent.cardBg }

  const content = (
    <>
      <CardHero game={game} genreMode={genreMode} playerCount={playerCount} />
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
