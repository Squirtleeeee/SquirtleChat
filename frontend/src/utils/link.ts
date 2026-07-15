/** Extract first http(s) URL from plain text (reply envelope already stripped by caller). */
export function firstHttpUrl(text: string): string | null {
  const m = text.match(/https?:\/\/[^\s<>"']+/i)
  if (!m) return null
  return m[0].replace(/[.,;:!?)]+$/, '')
}
