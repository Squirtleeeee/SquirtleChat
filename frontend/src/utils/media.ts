/** Gateway origin without /api/v1 (for /uploads and avatars). */
export function apiOrigin() {
  return import.meta.env.VITE_API_BASE?.replace('/api/v1', '') || 'http://localhost:8080'
}

/**
 * Resolve uploaded media URL for display/download.
 * Rewrites legacy MinIO direct URLs to gateway /uploads proxy.
 */
export function mediaUrl(url?: string) {
  if (!url) return ''
  if (url.startsWith('blob:') || url.startsWith('data:')) return url

  const minioMatch = url.match(/^https?:\/\/[^/]+\/squirtlechat\/(.+)$/i)
  if (minioMatch) {
    return `${apiOrigin()}/uploads/${minioMatch[1]}`
  }

  if (url.startsWith('http')) return url
  if (url.startsWith('/')) return `${apiOrigin()}${url}`
  return `${apiOrigin()}/${url}`
}
