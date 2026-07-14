export type ReplyMeta = {
  /** display name of quoted sender */
  n: string
  /** short preview of quoted body */
  p: string
  /** client_msg_id of quoted message (optional jump target) */
  c?: string
}

const START = '⟦sq-reply⟧'
const END = '⟦/sq-reply⟧'

export function buildReplyContent(meta: ReplyMeta, text: string): string {
  const payload = JSON.stringify({
    n: meta.n.slice(0, 32),
    p: meta.p.slice(0, 80),
    ...(meta.c ? { c: meta.c } : {}),
  })
  return `${START}${payload}${END}\n${text}`
}

export function parseReplyContent(content: string): { reply: ReplyMeta | null; text: string } {
  if (!content.startsWith(START)) return { reply: null, text: content }
  const end = content.indexOf(END)
  if (end < 0) return { reply: null, text: content }
  const raw = content.slice(START.length, end)
  let reply: ReplyMeta | null = null
  try {
    const o = JSON.parse(raw) as ReplyMeta
    if (o && typeof o.n === 'string' && typeof o.p === 'string') {
      reply = { n: o.n, p: o.p, c: typeof o.c === 'string' ? o.c : undefined }
    }
  } catch {
    return { reply: null, text: content }
  }
  let text = content.slice(end + END.length)
  if (text.startsWith('\n')) text = text.slice(1)
  return { reply, text }
}

export function stripReplyForPreview(content: string): string {
  return parseReplyContent(content).text
}
