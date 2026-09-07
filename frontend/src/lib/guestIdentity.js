/**
 * Guest identities for the first-entry avatar picker.
 *
 * Names are generated fresh on every draw rather than picked from a fixed list, so
 * two guests are very unlikely to end up sharing a name inside the same game. Each
 * name's noun decides its sigil, so a Fox always lands on the canine silhouette.
 */

/**
 * Guest-tier avatar shapes. Family keys match the sigil renderer in
 * backend/internal/avatars/sigil.go, which draws the silhouette for each key.
 */
export const SIGIL_FAMILIES = [
  { key: 'canine', nouns: ['Fox', 'Wolf', 'Hound', 'Jackal', 'Coyote', 'Dingo'] },
  { key: 'feline', nouns: ['Lynx', 'Tiger', 'Panther', 'Puma', 'Ocelot', 'Cougar'] },
  { key: 'horned', nouns: ['Ox', 'Ram', 'Bull', 'Stag', 'Elk', 'Bison'] },
  { key: 'raptor', nouns: ['Eagle', 'Falcon', 'Hawk', 'Osprey', 'Kestrel', 'Harrier'] },
  { key: 'corvid', nouns: ['Raven', 'Crow', 'Rook', 'Magpie', 'Jay', 'Grackle'] },
  { key: 'ursine', nouns: ['Bear', 'Bruin', 'Kodiak', 'Grizzly', 'Ursa', 'Sable'] },
  { key: 'rodent', nouns: ['Mouse', 'Rat', 'Squirrel', 'Chipmunk', 'Vole', 'Marmot'] },
  { key: 'lagomorph', nouns: ['Hare', 'Rabbit', 'Cottontail', 'Jackrabbit', 'Pika', 'Lop'] },
  { key: 'serpent', nouns: ['Cobra', 'Viper', 'Python', 'Adder', 'Mamba', 'Krait'] },
  { key: 'cephalopod', nouns: ['Octopus', 'Squid', 'Nautilus', 'Kraken', 'Cuttle', 'Argonaut'] },
  { key: 'cetacean', nouns: ['Whale', 'Orca', 'Narwhal', 'Beluga', 'Dolphin', 'Porpoise'] },
  { key: 'chelonian', nouns: ['Turtle', 'Tortoise', 'Terrapin', 'Snapper', 'Slider', 'Loggerhead'] },
  { key: 'equine', nouns: ['Horse', 'Mare', 'Stallion', 'Mustang', 'Bronco', 'Colt'] },
  { key: 'proboscid', nouns: ['Elephant', 'Mammoth', 'Mastodon', 'Tusker', 'Jumbo', 'Behemoth'] },
  { key: 'suid', nouns: ['Boar', 'Hog', 'Warthog', 'Peccary', 'Razorback', 'Sow'] },
  { key: 'primate', nouns: ['Ape', 'Gorilla', 'Macaque', 'Lemur', 'Gibbon', 'Baboon'] },
  { key: 'amphibian', nouns: ['Frog', 'Toad', 'Newt', 'Salamander', 'Axolotl', 'Peeper'] },
  { key: 'crustacean', nouns: ['Crab', 'Lobster', 'Prawn', 'Shrimp', 'Crayfish', 'Hermit'] },
  { key: 'arachnid', nouns: ['Spider', 'Tarantula', 'Widow', 'Recluse', 'Orbweaver', 'Weaver'] },
  { key: 'waterfowl', nouns: ['Swan', 'Heron', 'Crane', 'Egret', 'Ibis', 'Stork'] },
]

/** The silhouette drawn on top of every tint. */
export const SIGIL_SILHOUETTE = '#f8fafc'

/**
 * The adjective in a guest name still describes its colour, so a FrostFox really is
 * an icy blue one — but the colour is drawn per guest rather than picked from a
 * palette, so the word names the hue's neighbourhood rather than an exact swatch.
 * Bands run [previous `until`, `until`), starting at 0.
 */
export const SIGIL_HUE_WORDS = [
  { until: 15, word: 'Blaze' },
  { until: 35, word: 'Ember' },
  { until: 50, word: 'Rust' },
  { until: 68, word: 'Solar' },
  { until: 95, word: 'Fern' },
  { until: 125, word: 'Moss' },
  { until: 152, word: 'Pine' },
  { until: 172, word: 'Jade' },
  { until: 190, word: 'Tide' },
  { until: 205, word: 'Nova' },
  { until: 222, word: 'Frost' },
  { until: 248, word: 'Storm' },
  { until: 268, word: 'Iris' },
  { until: 290, word: 'Dusk' },
  { until: 320, word: 'Bloom' },
  { until: 345, word: 'Dawn' },
  { until: 360, word: 'Blaze' },
]

/**
 * Saturation is drawn from Beta(4, 2) across this range: muted tints look washed
 * out under the near-white silhouette, and fully saturated ones look garish, so the
 * distribution tapers off at both ends and peaks around 82%.
 */
const SATURATION_MIN = 45
const SATURATION_MAX = 95

/**
 * Lightness is picked by *perceived* luminance rather than by the HSL lightness
 * channel, because HSL lightness is not perceptual — a yellow at L=50% is far
 * brighter than a blue at L=50%, and the near-white silhouette would vanish on it.
 * Drawing a target luminance and solving for L compensates for the hue exactly.
 *
 * The ceiling is where a disc reads at exactly 3:1 against the silhouette, the
 * WCAG floor for non-text contrast; the floor keeps the darkest draws from losing
 * their colour. That puts every disc between 3.0:1 and 4.8:1, which is where the
 * twelve hand-picked tints this replaced sat.
 */
const LUMINANCE_MIN = 0.16
const LUMINANCE_MAX = 0.28

/** Four digits keeps names distinct without making them unreadable. */
const NUMBER_MIN = 1000
const NUMBER_MAX = 9999

/** How many identities the picker offers at once. */
export const GUEST_IDENTITY_CHOICES = 6

function randomInt(max) {
  return Math.floor(Math.random() * max)
}

function pickOne(items) {
  return items[randomInt(items.length)]
}

/** Returns `count` items from `items` in random order, without repeats. */
function sampleWithoutReplacement(items, count) {
  const pool = [...items]
  const picked = []
  while (picked.length < count && pool.length > 0) {
    picked.push(...pool.splice(randomInt(pool.length), 1))
  }
  return picked
}

/**
 * Draws from Beta(k, n + 1 - k) by taking the k-th smallest of `n` uniform draws,
 * which is exactly that distribution. Cheaper and clearer than a Beta sampler for
 * the small integer shapes we want: Beta(2, 2) leans to the middle, Beta(4, 2)
 * leans high while still tapering off before the top of the range.
 */
function betaFromOrderStatistic(k, n) {
  const draws = Array.from({ length: n }, () => Math.random()).sort((a, b) => a - b)
  return draws[k - 1]
}

/** Converts HSL (h in degrees, s and l in percent) to 8-bit RGB. */
function hslToRgb(h, s, l) {
  const saturation = s / 100
  const lightness = l / 100
  const chroma = (1 - Math.abs(2 * lightness - 1)) * saturation
  const sector = (((h % 360) + 360) % 360) / 60
  const second = chroma * (1 - Math.abs((sector % 2) - 1))
  const base = lightness - chroma / 2
  const rgb = [
    [chroma, second, 0],
    [second, chroma, 0],
    [0, chroma, second],
    [0, second, chroma],
    [second, 0, chroma],
    [chroma, 0, second],
  ][Math.floor(sector) % 6]
  return rgb.map((channel) => Math.round((channel + base) * 255))
}

/** WCAG relative luminance — the perceptual brightness the eye actually reads. */
export function relativeLuminance([r, g, b]) {
  const [rl, gl, bl] = [r, g, b].map((channel) => {
    const v = channel / 255
    return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
  })
  return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl
}

/**
 * Finds the HSL lightness that puts `hue` at `saturation` on `target` luminance.
 * Luminance rises monotonically with lightness at a fixed hue and saturation, so a
 * bisection always converges; 20 steps lands well inside a single 0-255 channel.
 */
function lightnessForLuminance(hue, saturation, target) {
  let low = 0
  let high = 100
  for (let i = 0; i < 20; i += 1) {
    const mid = (low + high) / 2
    if (relativeLuminance(hslToRgb(hue, saturation, mid)) < target) {
      low = mid
    } else {
      high = mid
    }
  }
  return (low + high) / 2
}

function rgbToHex([r, g, b]) {
  return [r, g, b].map((channel) => channel.toString(16).padStart(2, '0')).join('')
}

/** The colour word whose band covers `hue`. */
export function hueWord(hue) {
  const wrapped = (((hue % 360) + 360) % 360)
  return SIGIL_HUE_WORDS.find((band) => wrapped < band.until).word
}

/**
 * Builds one tint: a hue, a saturation that leans high, and a lightness solved for
 * a perceived luminance that leans to the middle of a legible band.
 */
export function generateTint(hue = Math.random() * 360) {
  const saturation =
    SATURATION_MIN + (SATURATION_MAX - SATURATION_MIN) * betaFromOrderStatistic(4, 5)
  const luminance = LUMINANCE_MIN + (LUMINANCE_MAX - LUMINANCE_MIN) * betaFromOrderStatistic(2, 3)
  const lightness = lightnessForLuminance(hue, saturation, luminance)
  return { hex: rgbToHex(hslToRgb(hue, saturation, lightness)), word: hueWord(hue) }
}

/**
 * Splits the colour wheel into `count` sectors and draws one hue from each, so the
 * row of choices never comes up as six near-identical blues. Shuffled afterwards so
 * the picker does not read as a rainbow gradient.
 */
function spreadHues(count) {
  const sector = 360 / count
  const offset = Math.random() * 360
  const hues = Array.from(
    { length: count },
    (_, index) => (offset + index * sector + Math.random() * sector) % 360,
  )
  return sampleWithoutReplacement(hues, hues.length)
}

export function sigilAvatarKey(familyKey, hex) {
  return `sigil-${familyKey}-${hex}`
}

export function sigilImageUrl(familyKey, hex) {
  return `/avatars/sigils/${familyKey}-${hex}.svg`
}

/**
 * Builds one identity: a tint word, a noun from `family`, and a 4-digit number.
 * The number is what keeps two guests from colliding on the same name.
 */
export function generateGuestIdentity(family, tint = generateTint()) {
  const number = NUMBER_MIN + randomInt(NUMBER_MAX - NUMBER_MIN + 1)
  return {
    name: `${tint.word}${pickOne(family.nouns)}${number}`,
    avatarKey: sigilAvatarKey(family.key, tint.hex),
    imageUrl: sigilImageUrl(family.key, tint.hex),
  }
}

/**
 * Builds a fresh set of choices for the picker — one per sigil family, so the row
 * shows six distinct silhouettes rather than six variations of the same animal,
 * each in a hue drawn from its own slice of the wheel.
 */
export function generateGuestIdentities(count = GUEST_IDENTITY_CHOICES) {
  const families = sampleWithoutReplacement(SIGIL_FAMILIES, count)
  const hues = spreadHues(families.length)
  return families.map((family, index) => generateGuestIdentity(family, generateTint(hues[index])))
}
