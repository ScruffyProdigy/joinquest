/**
 * Guest identities for the first-entry avatar picker.
 *
 * Names are generated fresh on every draw rather than picked from a fixed list, so
 * two guests are very unlikely to end up sharing a name inside the same game. Each
 * name's noun decides its sigil, so FrostFox always lands on the canine silhouette.
 */

/** Guest-tier avatars. Keys match SigilCatalog in backend/internal/avatars/catalog.go. */
export const SIGIL_FAMILIES = [
  {
    key: 'sigil-canine',
    imageUrl: '/avatars/sigil-canine.svg',
    nouns: ['Fox', 'Wolf', 'Hound', 'Jackal', 'Coyote', 'Dingo'],
  },
  {
    key: 'sigil-feline',
    imageUrl: '/avatars/sigil-feline.svg',
    nouns: ['Lynx', 'Tiger', 'Panther', 'Puma', 'Ocelot', 'Cougar'],
  },
  {
    key: 'sigil-horned',
    imageUrl: '/avatars/sigil-horned.svg',
    nouns: ['Ox', 'Ram', 'Bull', 'Stag', 'Elk', 'Bison'],
  },
  {
    key: 'sigil-raptor',
    imageUrl: '/avatars/sigil-raptor.svg',
    nouns: ['Eagle', 'Falcon', 'Hawk', 'Osprey', 'Kestrel', 'Harrier'],
  },
  {
    key: 'sigil-corvid',
    imageUrl: '/avatars/sigil-corvid.svg',
    nouns: ['Raven', 'Crow', 'Rook', 'Magpie', 'Jay', 'Grackle'],
  },
  {
    key: 'sigil-ursine',
    imageUrl: '/avatars/sigil-ursine.svg',
    nouns: ['Bear', 'Bruin', 'Kodiak', 'Grizzly', 'Ursa', 'Sable'],
  },
]

export const GUEST_NAME_ADJECTIVES = [
  'Frost', 'Ember', 'Swift', 'Bold', 'Nova', 'Ash', 'Wise', 'Blaze', 'Dusk', 'Dawn',
  'Iron', 'Storm', 'Wild', 'Lone', 'Quick', 'Grim', 'Bright', 'Shadow', 'Silver', 'Onyx',
]

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
 * Builds one identity: an adjective, a noun from `family`, and a 4-digit number.
 * The number is what keeps two guests from colliding on the same name.
 */
export function generateGuestIdentity(family, adjective = pickOne(GUEST_NAME_ADJECTIVES)) {
  const number = NUMBER_MIN + randomInt(NUMBER_MAX - NUMBER_MIN + 1)
  const name = `${adjective}${pickOne(family.nouns)}${number}`
  return { name, avatarKey: family.key, imageUrl: family.imageUrl }
}

/**
 * Builds a fresh set of choices for the picker — one per sigil family, so the row
 * shows six distinct silhouettes rather than six variations of the same animal.
 * Adjectives are drawn without replacement too, so no two choices rhyme.
 */
export function generateGuestIdentities(count = GUEST_IDENTITY_CHOICES) {
  const families = sampleWithoutReplacement(SIGIL_FAMILIES, count)
  const adjectives = sampleWithoutReplacement(GUEST_NAME_ADJECTIVES, families.length)
  return families.map((family, index) => generateGuestIdentity(family, adjectives[index]))
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
