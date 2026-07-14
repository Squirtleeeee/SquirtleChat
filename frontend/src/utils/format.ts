/** Format message time for chat bubbles. Pass showSeconds for HH:mm:ss. */
export function formatMessageTime(iso?: string, showSeconds = false): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const isToday =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  const timeOpts: Intl.DateTimeFormatOptions = {
    hour: '2-digit',
    minute: '2-digit',
    ...(showSeconds ? { second: '2-digit' as const } : {}),
  }
  const hm = d.toLocaleTimeString('zh-CN', timeOpts)
  if (isToday) return hm
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  const isYesterday =
    d.getFullYear() === yesterday.getFullYear() &&
    d.getMonth() === yesterday.getMonth() &&
    d.getDate() === yesterday.getDate()
  if (isYesterday) return `昨天 ${hm}`
  if (d.getFullYear() === now.getFullYear()) {
    return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', ...timeOpts })
  }
  return d.toLocaleString('zh-CN', {
    year: 'numeric',
    month: 'numeric',
    day: 'numeric',
    ...timeOpts,
  })
}

export function previewMessage(content: string, msgType = 1): string {
  if (msgType === 4 || content === '[已撤回]') return '[已撤回]'
  if (msgType === 2) return '[图片]'
  if (msgType === 3) return '[文件]'
  if (content.startsWith('{') && content.includes('"url"')) return '[文件]'
  let t = content.trim()
  if (t.startsWith('⟦sq-reply⟧')) {
    const end = t.indexOf('⟦/sq-reply⟧')
    if (end >= 0) {
      t = t.slice(end + '⟦/sq-reply⟧'.length).replace(/^\n/, '').trim()
    }
  }
  if (t.length <= 24) return t
  return `${t.slice(0, 24)}…`
}

/** Relative time for conversation list (微信风格). */
export function formatListTime(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = Date.now()
  const diff = now - d.getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3600_000) return `${Math.floor(diff / 60_000)}分钟前`
  const today = new Date()
  const isToday =
    d.getFullYear() === today.getFullYear() &&
    d.getMonth() === today.getMonth() &&
    d.getDate() === today.getDate()
  const hm = d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit' })
  if (isToday) return hm
  const yesterday = new Date(today)
  yesterday.setDate(today.getDate() - 1)
  const isYesterday =
    d.getFullYear() === yesterday.getFullYear() &&
    d.getMonth() === yesterday.getMonth() &&
    d.getDate() === yesterday.getDate()
  if (isYesterday) return '昨天'
  if (d.getFullYear() === today.getFullYear()) {
    return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric' })
  }
  return d.toLocaleString('zh-CN', { year: 'numeric', month: 'numeric', day: 'numeric' })
}

export function dayKey(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return `${d.getFullYear()}-${d.getMonth()}-${d.getDate()}`
}

export function formatDateDivider(iso?: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const now = new Date()
  const isToday =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate()
  if (isToday) return '今天'
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  const isYesterday =
    d.getFullYear() === yesterday.getFullYear() &&
    d.getMonth() === yesterday.getMonth() &&
    d.getDate() === yesterday.getDate()
  if (isYesterday) return '昨天'
  if (d.getFullYear() === now.getFullYear()) {
    return d.toLocaleString('zh-CN', { month: 'long', day: 'numeric' })
  }
  return d.toLocaleString('zh-CN', { year: 'numeric', month: 'long', day: 'numeric' })
}
