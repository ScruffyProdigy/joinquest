import { useMemo } from 'react'
import { Button } from '../ui/button'
import { getGraphQLUrl } from '../../lib/env'
import {
  buildExampleProvisionPayload,
  buildProvisionCurl,
  lobbyUrlsFromGraphQL,
} from '../../lib/exampleProvision'

export default function DeveloperProvisionExample({ game, credentials }) {
  const example = useMemo(() => {
    if (!game || !credentials?.serviceToken) {
      return null
    }
    try {
      const graphqlUrl = getGraphQLUrl()
      const resolvedGraphql = graphqlUrl.startsWith('/') ? 'http://localhost:8080/graphql' : graphqlUrl
      const urls = lobbyUrlsFromGraphQL(resolvedGraphql)
      const payload = buildExampleProvisionPayload({
        game,
        credentials,
        ...urls,
      })
      const curl = buildProvisionCurl(game.apiBaseUrl, payload)
      return { payload, curl }
    } catch {
      return null
    }
  }, [game, credentials])

  if (!example) {
    return null
  }

  const payloadText = JSON.stringify(example.payload, null, 2)

  async function handleCopy(text) {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      // ignore
    }
  }

  return (
    <section className="panel-card developer-provision" aria-labelledby="provision-heading">
      <h2 id="provision-heading">Example provision request</h2>
      <p className="panel-copy">
        POST this JSON to <code>{game.apiBaseUrl}/api/v1/matches</code> while debugging. JoinQuest uses
        similar payloads when starting matches.
      </p>
      <pre className="developer-provision__body">{payloadText}</pre>
      <div className="developer-actions__row">
        <Button type="button" variant="secondary" onClick={() => void handleCopy(payloadText)}>
          Copy JSON
        </Button>
        <Button type="button" variant="secondary" onClick={() => void handleCopy(example.curl)}>
          Copy curl
        </Button>
      </div>
    </section>
  )
}
