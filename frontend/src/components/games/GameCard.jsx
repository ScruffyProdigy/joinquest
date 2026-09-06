import { accentColorFor } from '../../lib/gameAccent'
import {
  gameCardMeta,
  gameCatalogHeroUrl,
  gameLiveActivityLabel,
  gamePagePath,
  gameTitleArtStyle,
  gameTitleArtUrl,
} from '../../lib/gameCard'
import CatalogGameLink from './CatalogGameLink'

const PILL_CLASS =
  'flex items-center gap-1 px-2 py-1 rounded-full shrink-0 border border-white/20 bg-black/[0.42] backdrop-blur-[6px]'
const PILL_TEXT_CLASS = 'text-[12px] font-bold leading-4 whitespace-nowrap text-white/[0.92]'

// Metadata glyphs, sized to sit on the pill's 16px line without shifting it.
const PILL_ICONS = {
  players: (
    <path d="M10 11H2V10C2 8.6193 3.11929 7.5 4.5 7.5H7.5C8.8807 7.5 10 8.6193 10 10V11ZM6 6.5C4.34315 6.5 3 5.15685 3 3.5C3 1.84315 4.34315 0.5 6 0.5C7.65685 0.5 9 1.84315 9 3.5C9 5.15685 7.65685 6.5 6 6.5Z" />
  ),
  duration: (
    <path d="M6 11C3.23857 11 1 8.7614 1 6C1 3.23857 3.23857 1 6 1C8.7614 1 11 3.23857 11 6C11 8.7614 8.7614 11 6 11ZM6.5 6V3.5H5.5V7H8.5V6H6.5Z" />
  ),
}

function CardPill({ icon, children }) {
  return (
    <span className={PILL_CLASS}>
      {icon ? (
        <svg width="12" height="12" viewBox="0 0 12 12" fill="white" aria-hidden="true" className="shrink-0">
          {PILL_ICONS[icon]}
        </svg>
      ) : null}
      <span className={PILL_TEXT_CLASS}>{children}</span>
    </span>
  )
}

export default function GameCard({ game }) {
  const accent = accentColorFor(game.slug || 'default', game.accentColor)
  const detailPath = gamePagePath(game)
  const heroUrl = gameCatalogHeroUrl(game)
  const meta = gameCardMeta(game)
  const liveActivity = gameLiveActivityLabel(game.playerActivity)
  const titleArtUrl = gameTitleArtUrl(game.titleArt)
  const titleArtStyle = gameTitleArtStyle(game.titleArt)

  const content = (
    <>
      <img
        className="absolute inset-0 w-full h-full object-cover object-top pointer-events-none"
        src={heroUrl}
        alt=""
        loading="lazy"
      />

      {/* The wordmark is composited live rather than baked into the hero so it can be
          scaled against the card and, later, swapped for the viewer's language. Some
          marks are placed where the scrim covers them; they are still drawn, because
          where a mark sits is the artwork's decision and the next pass on this card
          may well uncover it. */}
      {titleArtUrl && titleArtStyle ? (
        <img className="pointer-events-none" src={titleArtUrl} alt="" style={titleArtStyle} aria-hidden="true" />
      ) : null}

      {/* Who is on the game right now. Kept opposite the metadata row rather than in
          it: the row describes the game, this describes the moment. */}
      {liveActivity ? (
        <div className="absolute top-3 right-3 flex justify-end">
          <CardPill>{liveActivity}</CardPill>
        </div>
      ) : null}

      {/* The design splits the card 40/60 and hangs the text off the bottom. Held as a
          floor rather than a fixed line: a long name at phone width needs more than the
          bottom 60%, and growing the block is better than letting the text climb out of
          its own scrim onto bare artwork. */}
      <div className="absolute inset-x-0 bottom-0 min-h-[60%] flex flex-col justify-end">
        {/* Darkens whatever the text sits on. Masked so the blur fades in rather than
            cutting a visible line across the art. */}
        <div
          className="absolute inset-0 pointer-events-none"
          style={{
            background: 'linear-gradient(rgba(0, 0, 0, 0) 0%, rgba(0, 0, 0, 0.55) 100%)',
            backdropFilter: 'blur(20px)',
            WebkitBackdropFilter: 'blur(20px)',
            maskImage: 'linear-gradient(transparent 0%, black 40%)',
            WebkitMaskImage: 'linear-gradient(transparent 0%, black 40%)',
          }}
        />

        <div className="relative flex flex-col gap-2 px-4 pb-5">
          {meta.length > 0 ? (
            <div className="flex flex-wrap gap-1">
              {meta.map((pill) => (
                <CardPill key={pill.key} icon={pill.icon}>
                  {pill.label}
                </CardPill>
              ))}
            </div>
          ) : null}
          <h3 className="text-2xl font-bold leading-[30px] text-white m-0">{game.name}</h3>
          {game.shortDescription ? (
            <p className="text-[15px] leading-[21px] text-white m-0 line-clamp-2">{game.shortDescription}</p>
          ) : null}
        </div>
      </div>
    </>
  )

  // Fills the card while the hero loads, and stands in for it when a game has only
  // the placeholder art, so the slot never flashes as a bare grey rectangle.
  const shellStyle = { background: accent.badge }
  const shellClassName =
    'absolute inset-px rounded-2xl overflow-hidden text-left transition-transform hover:-translate-y-0.5 active:scale-[0.99] focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2'

  return (
    <li className="relative w-full pb-[75%]">
      {/* A blurred, slightly enlarged copy of the hero bleeding out from under the
          card edge — the art tints its own surroundings instead of a border doing it. */}
      <div
        className="absolute inset-0 rounded-2xl overflow-hidden pointer-events-none"
        aria-hidden="true"
        style={{ filter: 'blur(12px)', transform: 'scale(1.02)', opacity: 0.4 }}
      >
        <img className="absolute inset-0 w-full h-full object-cover" src={heroUrl} alt="" loading="lazy" />
      </div>

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
