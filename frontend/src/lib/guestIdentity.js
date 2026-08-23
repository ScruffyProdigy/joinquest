/**
 * Guest identities for the first-entry avatar picker.
 *
 * Names are generated fresh on every draw rather than picked from a fixed list, so
 * two guests are very unlikely to end up sharing a name inside the same game. Each
 * name's noun decides its sigil, so FrostFox always lands on the canine silhouette.
 */

/**
 * Guest-tier avatar shapes. Family keys match the sigil parser in
 * backend/internal/avatars/catalog.go.
 */
export const SIGIL_FAMILIES = [
  { key: 'canine', nouns: ['Fox', 'Wolf', 'Hound', 'Jackal', 'Coyote', 'Dingo'] },
  { key: 'feline', nouns: ['Lynx', 'Tiger', 'Panther', 'Puma', 'Ocelot', 'Cougar'] },
  { key: 'horned', nouns: ['Ox', 'Ram', 'Bull', 'Stag', 'Elk', 'Bison'] },
  { key: 'raptor', nouns: ['Eagle', 'Falcon', 'Hawk', 'Osprey', 'Kestrel', 'Harrier'] },
  { key: 'corvid', nouns: ['Raven', 'Crow', 'Rook', 'Magpie', 'Jay', 'Grackle'] },
  { key: 'ursine', nouns: ['Bear', 'Bruin', 'Kodiak', 'Grizzly', 'Ursa', 'Sable'] },
]

/**
 * The adjective in a guest name is also its colour, so FrostFox really is the icy
 * blue one. Tints are deep enough to carry the near-white silhouette on top.
 */
export const SIGIL_TINTS = [
  { key: 'frost', word: 'Frost', hex: '#0284c7' },
  { key: 'ember', word: 'Ember', hex: '#ea580c' },
  { key: 'blaze', word: 'Blaze', hex: '#dc2626' },
  { key: 'dawn', word: 'Dawn', hex: '#e11d48' },
  { key: 'dusk', word: 'Dusk', hex: '#9333ea' },
  { key: 'storm', word: 'Storm', hex: '#4f46e5' },
  { key: 'moss', word: 'Moss', hex: '#16a34a' },
  { key: 'tide', word: 'Tide', hex: '#0d9488' },
  { key: 'solar', word: 'Solar', hex: '#ca8a04' },
  { key: 'nova', word: 'Nova', hex: '#0891b2' },
  { key: 'rust', word: 'Rust', hex: '#d97706' },
  { key: 'bloom', word: 'Bloom', hex: '#db2777' },
]

/** The silhouette drawn on top of every tint. */
export const SIGIL_SILHOUETTE = '#f8fafc'

export function sigilAvatarKey(familyKey, tintKey) {
  return `sigil-${familyKey}-${tintKey}`
}

export function sigilImageUrl(familyKey, tintKey) {
  return `/avatars/sigils/${familyKey}-${tintKey}.svg`
}

/** Four digits keeps names distinct without making them unreadable. */
const NUMBER_MIN = 1000
const NUMBER_MAX = 9999

/** How many identities the picker offers at once. */
export const GUEST_IDENTITY_CHOICES = 6

/** Guest display names the backend hands out before a player picks one. */
const GENERATED_GUEST_NAME = /^guest#\d+$/i

/** Names still carrying the backend's provisional marker. */
const PROVISIONAL_SUFFIX = ' (new)'

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
 * Builds one identity: a tint word, a noun from `family`, and a 4-digit number.
 * The number is what keeps two guests from colliding on the same name.
 */
export function generateGuestIdentity(family, tint = pickOne(SIGIL_TINTS)) {
  const number = NUMBER_MIN + randomInt(NUMBER_MAX - NUMBER_MIN + 1)
  return {
    name: `${tint.word}${pickOne(family.nouns)}${number}`,
    avatarKey: sigilAvatarKey(family.key, tint.key),
    imageUrl: sigilImageUrl(family.key, tint.key),
  }
}

/**
 * Builds a fresh set of choices for the picker — one per sigil family, so the row
 * shows six distinct silhouettes rather than six variations of the same animal.
 * Tints are drawn without replacement too, so no two choices share a colour.
 */
export function generateGuestIdentities(count = GUEST_IDENTITY_CHOICES) {
  const families = sampleWithoutReplacement(SIGIL_FAMILIES, count)
  const tints = sampleWithoutReplacement(SIGIL_TINTS, families.length)
  return families.map((family, index) => generateGuestIdentity(family, tints[index]))
}

/** True while a display name is still a placeholder the player never chose. */
export function isGeneratedDisplayName(name) {
  const trimmed = name?.trim() ?? ''
  return trimmed === '' || GENERATED_GUEST_NAME.test(trimmed) || trimmed.endsWith(PROVISIONAL_SUFFIX)
}

/**
 * The single trigger for the picker overlay. Timing is still under discussion, so
 * keep the whole condition here — moving the overlay later means changing this
 * function and nothing else.
 */
export function needsIdentity(user) {
  if (!user) {
    return true
  }
  const hasAvatar = Boolean(user.avatarKey?.trim()) || Boolean(user.avatarUrl?.trim())
  return !hasAvatar || isGeneratedDisplayName(user.displayName)
}
