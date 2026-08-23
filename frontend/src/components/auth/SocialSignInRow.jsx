import { useEffect, useState } from 'react'
import { fetchEnabledOAuthProviders, startOAuthSignIn } from '../../lib/oauth'
import { Button } from '../ui/button'
import OAuthProviderIcon, { oauthProviderLabel } from './OAuthProviderIcon'

export default function SocialSignInRow() {
  const [providers, setProviders] = useState([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const enabled = await fetchEnabledOAuthProviders()
        if (!cancelled) {
          setProviders(enabled)
        }
      } catch {
        if (!cancelled) {
          setProviders([])
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [])

  if (loading || providers.length === 0) {
    return null
  }

  return (
    <div className="flex flex-col gap-2" aria-label="Social sign-in options">
      {providers.map((provider) => (
        <Button
          key={provider}
          type="button"
          variant="outline"
          className="w-full justify-start gap-3"
          onClick={() => startOAuthSignIn(provider)}
        >
          <OAuthProviderIcon provider={provider} />
          Continue with {oauthProviderLabel(provider)}
        </Button>
      ))}
    </div>
  )
}
