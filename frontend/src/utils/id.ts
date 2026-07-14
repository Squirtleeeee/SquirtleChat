/** Snowflake IDs must stay strings in JS (exceeds Number.MAX_SAFE_INTEGER). */
export type UserId = string

export function idStr(v: string | number | undefined | null): UserId {
  if (v == null || v === '') return ''
  return String(v)
}

export function directConvId(a: UserId, b: UserId): string {
  if (!a || !b) return ''
  return BigInt(a) < BigInt(b) ? `${a}_${b}` : `${b}_${a}`
}

export function sameId(a: string | number | undefined, b: string | number | undefined): boolean {
  return idStr(a) === idStr(b)
}
