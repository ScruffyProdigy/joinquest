import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'

/**
 * JQ-72: link and button chrome belongs to the ui/ primitives, not to page code.
 * A convention in AGENTS.md did not hold -- 36 bare anchors accumulated -- so the
 * rule is enforced here instead, where CI runs it.
 */
const NO_BARE_ANCHOR = {
  selector: 'JSXOpeningElement[name.name="a"]',
  message:
    'Use <Link> from components/ui/link.jsx rather than a bare <a>. If this anchor is structural (it wraps a card or an avatar, or is a button that navigates), add the file to ANCHOR_ALLOWED in eslint.config.js with a comment saying why.',
}

const NO_BARE_BUTTON = {
  selector: 'JSXOpeningElement[name.name="button"]',
  message:
    'Use <Button> from components/ui/button.jsx, or <Link> for a link-styled action, rather than a bare <button>.',
}

/**
 * Anchors kept on purpose, each with the reasoning at the call site. These wrap a
 * card or an avatar, or sit inside <Button asChild> -- already a brand component
 * via Radix Slot. Permanent, not debt.
 */
const ANCHOR_ALLOWED = [
  'src/components/games/CatalogGameLink.jsx',
  'src/components/developers/DeveloperPromoCard.jsx',
  'src/components/home/AccountChip.jsx',
  'src/components/auth/LinkEmailPage.jsx',
  'src/components/auth/UserSessionCard.jsx',
  'src/components/developers/DeveloperAuthGate.jsx',
  'src/components/games/IntentBanner.jsx',
]

/**
 * Bare <button> that predates the rule. This list is debt, not licence: it exists
 * so the rule can land without a second migration riding along on JQ-72. Shrink it,
 * never add to it.
 */
const BUTTON_ALLOWED = [
  'src/components/games/PreQueueOptionsSheet.jsx',
  'src/components/games/GameModesPanel.jsx',
  'src/components/developers/DeveloperMcpWizard.jsx',
  'src/components/developers/DeveloperLandingPage.jsx',
  'src/components/dev/ComponentLibrarySection.jsx',
  'src/components/rooms/RoomPanel.jsx',
  'src/components/developers/YourGamesStrip.jsx',
  'src/components/avatars/SpiritAnimalFlow.jsx',
  'src/components/avatars/PlayerProfileEditor.jsx',
  'src/components/avatars/GuestIdentityPicker.jsx',
  'src/components/avatars/AvatarPrompt.jsx',
]

/**
 * link.jsx is the component that *provides* the anchor and the button, so it is the
 * one place both are correct. GameQueueActions has both a structural anchor and
 * pre-existing bare buttons.
 */
const PRIMITIVE_OR_BOTH = [
  'src/components/ui/link.jsx',
  'src/components/games/GameQueueActions.jsx',
]

export default [
  {
    ignores: ['dist', 'node_modules', 'playwright-report', 'coverage'],
  },
  {
    files: ['**/*.{js,jsx}'],
    languageOptions: {
      ecmaVersion: 2020,
      globals: {
        ...globals.browser,
        ...globals.node,
        ...globals.es2020,
      },
      parserOptions: {
        ecmaVersion: 'latest',
        ecmaFeatures: { jsx: true },
        sourceType: 'module',
      },
    },
    plugins: {
      'react-hooks': reactHooks,
      'react-refresh': reactRefresh,
    },
    rules: {
      ...js.configs.recommended.rules,
      ...reactHooks.configs.recommended.rules,
      'react-refresh/only-export-components': [
        'warn',
        { allowConstantExport: true },
      ],
      'no-unused-vars': ['error', { 
        varsIgnorePattern: '^[A-Z_]',
        argsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_'
      }],
    },
  },
  {
    files: ['**/*.test.{js,jsx}', '**/test/**/*.{js,jsx}', '**/tests/**/*.{js,jsx}'],
    languageOptions: {
      globals: {
        ...globals.browser,
        ...globals.node,
        ...globals.es2020,
        vi: 'readonly',
        test: 'readonly',
        expect: 'readonly',
        describe: 'readonly',
        it: 'readonly',
        beforeEach: 'readonly',
        afterEach: 'readonly',
        beforeAll: 'readonly',
        afterAll: 'readonly',
      },
    },
    rules: {
      'no-unused-vars': ['error', { 
        varsIgnorePattern: '^[A-Z_]',
        argsIgnorePattern: '^_',
        caughtErrorsIgnorePattern: '^_'
      }],
    },
  },
  {
    files: ['playwright.config.js', 'vite.config.js'],
    languageOptions: {
      globals: {
        ...globals.node,
        ...globals.es2020,
      },
    },
  },
  // JQ-72. Order matters: each block below replaces the rule for the files it names,
  // so the narrower allowances come after the blanket ban.
  {
    files: ['src/**/*.jsx'],
    ignores: ['**/*.test.jsx'],
    rules: {
      'no-restricted-syntax': ['error', NO_BARE_ANCHOR, NO_BARE_BUTTON],
    },
  },
  {
    files: ANCHOR_ALLOWED,
    rules: { 'no-restricted-syntax': ['error', NO_BARE_BUTTON] },
  },
  {
    files: BUTTON_ALLOWED,
    rules: { 'no-restricted-syntax': ['error', NO_BARE_ANCHOR] },
  },
  {
    files: PRIMITIVE_OR_BOTH,
    rules: { 'no-restricted-syntax': 'off' },
  },
]
