import {
  DEVELOPER_PROMO_BODY,
  DEVELOPER_PROMO_CTA,
  DEVELOPER_PROMO_TITLE,
  DEVELOPER_PROMO_TITLE_NO_RESULTS,
} from '../../lib/playerCopy'
import { DEVELOPER_LANDING_PATH } from '../../lib/developers'

/**
 * The developer pitch, sitting in the catalog list as a sibling of the game cards.
 * Deliberately has no hero art, player count, or duration pill — nothing that would
 * read as a playable game.
 *
 * It stays put when a search or filter is on. With nothing left to show it retitles
 * itself, turning the dead end into the pitch — the prototype's behaviour.
 */
export default function DeveloperPromoCard({ noResults = false }) {
  return (
    <li>
      {/* Padding lives on the inner box, not the sized one: box-sizing is content-box
          app-wide, so padding on a w-full element overflows the row. GameCard does the same. */}
      <a
        href={DEVELOPER_LANDING_PATH}
        className="block w-full rounded-2xl border border-dashed border-border bg-card/40 text-left transition-colors hover:border-primary/60 hover:bg-card/70 focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
      >
        <div className="px-4 py-4">
          <h3 className="m-0 text-base font-bold text-foreground">
            {noResults ? DEVELOPER_PROMO_TITLE_NO_RESULTS : DEVELOPER_PROMO_TITLE}
          </h3>
          <p className="mt-1 mb-0 text-sm leading-snug text-muted-foreground">
            {DEVELOPER_PROMO_BODY}
          </p>
          <span className="mt-2 inline-block text-sm font-semibold text-primary">
            {DEVELOPER_PROMO_CTA}
          </span>
        </div>
      </a>
    </li>
  )
}
