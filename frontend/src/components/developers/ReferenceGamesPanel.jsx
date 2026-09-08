import { APP_NAME } from '../../lib/brand'
import { Link } from '../ui/link'

const REFERENCE_GAMES = [
  {
    name: 'Rock Paper Scissors Lizard Robot',
    blurb: 'Smallest duel — best starting point for the API handshake and tests.',
    github: 'https://github.com/ScruffyProdigy/rpslr',
    live: 'https://rpsls-duel.win',
  },
  {
    name: 'Word Hunt',
    blurb: 'Party game with a richer client — path launch URLs and multi-player sync.',
    github: 'https://github.com/ScruffyProdigy/wordhunt',
    live: 'https://word-hunt-arena.win',
  },
]

export default function ReferenceGamesPanel() {
  return (
    <section className="panel-card developer-reference-games" aria-labelledby="reference-games-heading">
      <h2 id="reference-games-heading">See working examples</h2>
      <p className="panel-copy">
        These games run on {APP_NAME} today. Play them, then browse the source on GitHub — same
        handshake your game will use, plus a real client players actually play.
      </p>
      <ul className="developer-reference-games__list">
        {REFERENCE_GAMES.map((game) => (
          <li key={game.github} className="developer-reference-games__item">
            <h3 className="developer-reference-games__title">{game.name}</h3>
            <p className="panel-copy">{game.blurb}</p>
            <p className="developer-reference-games__links">
              <Link href={game.live} target="_blank" rel="noreferrer">
                Play live
              </Link>
              <span aria-hidden="true"> · </span>
              <Link href={game.github} target="_blank" rel="noreferrer">
                View on GitHub
              </Link>
            </p>
          </li>
        ))}
      </ul>
    </section>
  )
}
