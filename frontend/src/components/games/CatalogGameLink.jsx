import { navigateToGameDetail } from '../../lib/catalogNavigation'

/**
 * A raw <a> on purpose (JQ-72): this wraps a whole game card rather than a run of
 * text, so it carries the caller's layout classes and wants none of the Link
 * component's own colour or spacing. What it needs from an anchor is the href --
 * middle-click and cmd-click keep working while the plain click is intercepted
 * for client-side navigation.
 */
export default function CatalogGameLink({ href, slug, className, children, ...props }) {
  function handleClick(event) {
    if (!slug || event.defaultPrevented) {
      return
    }
    if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) {
      return
    }
    event.preventDefault()
    navigateToGameDetail(slug)
  }

  return (
    <a href={href} className={className} onClick={handleClick} {...props}>
      {children}
    </a>
  )
}
