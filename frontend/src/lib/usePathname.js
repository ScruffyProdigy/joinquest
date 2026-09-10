import { useEffect, useState } from 'react'

export function usePathname() {
  const [pathname, setPathname] = useState(() => window.location.pathname)

  useEffect(() => {
    const onPopState = () => setPathname(window.location.pathname)
    window.addEventListener('popstate', onPopState)
    return () => window.removeEventListener('popstate', onPopState)
  }, [])

  return pathname
}

let appNavigating = false

/**
 * True only while the app's own navigation is dispatching its popstate. A pop the
 * browser raised — a Back press, or the iOS edge swipe — never sets it, which is the
 * only thing that tells the two apart: `navigateTo` synthesises the same event.
 */
export function isAppNavigation() {
  return appNavigating
}

export function navigateTo(path, { replace = false } = {}) {
  if (replace) {
    window.history.replaceState(null, '', path)
  } else {
    window.history.pushState(null, '', path)
  }
  appNavigating = true
  try {
    window.dispatchEvent(new PopStateEvent('popstate'))
  } finally {
    appNavigating = false
  }
}
