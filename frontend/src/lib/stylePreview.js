/** @returns {boolean} */
export function isStylePreviewEnabled() {
  if (typeof window === 'undefined') {
    return false
  }
  return window.env?.REACT_APP_STYLE_PREVIEW === 'true'
}
