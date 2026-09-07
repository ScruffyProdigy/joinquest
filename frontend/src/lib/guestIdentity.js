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
  { key: 'lepidopteran', nouns: ['Moth', 'Monarch', 'Swallowtail', 'Luna', 'Admiral', 'Skipper'] },
  { key: 'echinoderm', nouns: ['Starfish', 'Seastar', 'Sunstar', 'Brittlestar', 'Urchin', 'Cushion'] },
  { key: 'gastropod', nouns: ['Snail', 'Whelk', 'Conch', 'Periwinkle', 'Limpet', 'Cowrie'] },
  { key: 'spheniscid', nouns: ['Penguin', 'Emperor', 'Gentoo', 'Adelie', 'Rockhopper', 'Auk'] },
]

/**
 * A sigil is drawn in one of two marks: near-white on a dark disc, near-black on a
 * light one, whichever reads more strongly. Carrying both is what lets the disc use
 * the whole lightness range rather than only the dark end a pale mark can sit on.
 */
export const SIGIL_PALE = '#f8fafc'
export const SIGIL_INK = '#10141a'

/** Disc luminance above which the dark mark wins. Both clear 4.2:1 at the crossover. */
export const SIGIL_MARK_CROSSOVER = 0.189

/**
 * The adjective in a guest name still describes the disc. The wheel is cut into
 * sixteen equal slices of *perceived* hue rather than of HSL degrees, so each word
 * is equally likely and each one covers about as much visible colour as the next.
 * Cutting by HSL degrees instead gave green four words for a region that occupies
 * under a tenth of perceived hue, which is why so many guests came out green.
 */
export const SIGIL_HUE_WORDS = [
  'Blaze', 'Ember', 'Rust', 'Solar', 'Fern', 'Moss', 'Jade', 'Tide',
  'Nova', 'Frost', 'Storm', 'Iris', 'Dusk', 'Bloom', 'Rose', 'Dawn',
]

/**
 * Saturation is drawn from Beta(3, 2) across this range. It has no effect on
 * legibility — contrast is governed by the luminance below, which is chosen
 * independently — so the bounds are purely about how the set feels, and they are
 * wide: a muted slate guest next to a vivid magenta one is the point. The
 * distribution still leans high, peaking around 76%, and tapers off at both ends.
 */
const SATURATION_MIN = 25
const SATURATION_MAX = 100

/**
 * Lightness is drawn in CIE L*, which is perceptually even, and then solved for in
 * HSL, which is not — a yellow at HSL L=50% is far brighter than a blue at the
 * same number. Drawing a target and solving for the channel compensates for the hue
 * exactly, the same trick the hue itself uses.
 *
 * The range is this wide because the mark is two-tone. A pale-only mark pins every
 * disc below L*≈60 so it stays dark enough to carry white; letting the mark go dark
 * on light discs opens the top of the scale, and it is where most of the variety in
 * the set now comes from. The bounds are where colour itself gives out: below 32 a
 * disc reads black, above 88 it washes out to paper.
 */
const LIGHTNESS_MIN = 32
const LIGHTNESS_MAX = 88

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

/** CIE L* to relative luminance. L* is spaced the way the eye reads lightness. */
function luminanceForLightness(lstar) {
  const f = (lstar + 16) / 116
  return f > 6 / 29 ? f ** 3 : 3 * (6 / 29) ** 2 * (f - 4 / 29)
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

/** Undoes the sRGB transfer function, so channels can be mixed by light. */
function toLinear(channel) {
  const v = channel / 255
  return v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4
}

/** WCAG relative luminance — the perceptual brightness the eye actually reads. */
export function relativeLuminance([r, g, b]) {
  const [rl, gl, bl] = [r, g, b].map(toLinear)
  return 0.2126 * rl + 0.7152 * gl + 0.0722 * bl
}

/**
 * The OKLab hue angle — where a colour sits on the wheel the eye draws, rather
 * than the one sRGB happens to be parameterised by. The two disagree badly: HSL
 * spends 75 degrees on green, across which perceived hue barely moves at all.
 */
export function perceivedHue(hue, saturation, lightness) {
  const [r, g, b] = hslToRgb(hue, saturation, lightness).map(toLinear)
  const l = Math.cbrt(0.4122214708 * r + 0.5363325363 * g + 0.0514459929 * b)
  const m = Math.cbrt(0.2119034982 * r + 0.6806995451 * g + 0.1073969566 * b)
  const s = Math.cbrt(0.0883024619 * r + 0.2817188376 * g + 0.6299787005 * b)
  const a = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s
  const bb = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s
  return ((Math.atan2(bb, a) * 180) / Math.PI + 360) % 360
}

/**
 * Finds the HSL hue sitting `offset` degrees of *perceived* hue around the wheel
 * from HSL red, at the saturation and luminance this draw will use. Perceived hue
 * rises monotonically with HSL hue, so a bisection finds it.
 *
 * This is solved per draw rather than read from a baked table because the mapping
 * shifts by as much as 14 perceived degrees across the saturation and luminance
 * ranges above — more than half a word's worth.
 */
function hueForPerceivedOffset(offset, saturation, luminance) {
  const at = (hue) =>
    perceivedHue(hue, saturation, lightnessForLuminance(hue, saturation, luminance))
  const origin = at(0)
  const travelled = (hue) => (at(hue) - origin + 360) % 360

  let low = 0
  let high = 359.99
  for (let i = 0; i < 15; i += 1) {
    const mid = (low + high) / 2
    if (travelled(mid) < offset) {
      low = mid
    } else {
      high = mid
    }
  }
  return (low + high) / 2
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

/** The colour word for a position `offset` degrees around the perceived wheel. */
export function hueWord(offset) {
  const slice = 360 / SIGIL_HUE_WORDS.length
  return SIGIL_HUE_WORDS[Math.floor(((((offset % 360) + 360) % 360)) / slice)]
}

/**
 * Builds one tint from a position on the perceived colour wheel: a saturation that
 * leans high, a lightness solved for a perceived luminance that leans to the middle
 * of the legible band, and the HSL hue that lands on `offset` given both.
 */
export function generateTint(offset = Math.random() * 360) {
  const saturation =
    SATURATION_MIN + (SATURATION_MAX - SATURATION_MIN) * betaFromOrderStatistic(3, 4)
  const luminance = luminanceForLightness(
    LIGHTNESS_MIN + (LIGHTNESS_MAX - LIGHTNESS_MIN) * betaFromOrderStatistic(2, 3),
  )
  const hue = hueForPerceivedOffset(offset, saturation, luminance)
  const lightness = lightnessForLuminance(hue, saturation, luminance)
  return { hex: rgbToHex(hslToRgb(hue, saturation, lightness)), word: hueWord(offset) }
}

/**
 * Splits the perceived colour wheel into `count` sectors and draws one position
 * from each, so the row of choices never comes up as six near-identical blues.
 * Shuffled afterwards so the picker does not read as a rainbow gradient.
 */
function spreadHues(count) {
  const sector = 360 / count
  const offset = Math.random() * 360
  const positions = Array.from(
    { length: count },
    (_, index) => (offset + index * sector + Math.random() * sector) % 360,
  )
  return sampleWithoutReplacement(positions, positions.length)
}

/** The mark a disc of `hex` is drawn in — matches the renderer in sigil.go. */
export function sigilMark(hex) {
  const rgb = [0, 2, 4].map((offset) => parseInt(hex.slice(offset, offset + 2), 16))
  return relativeLuminance(rgb) > SIGIL_MARK_CROSSOVER ? SIGIL_INK : SIGIL_PALE
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
  const positions = spreadHues(families.length)
  return families.map((family, index) =>
    generateGuestIdentity(family, generateTint(positions[index])),
  )
}
