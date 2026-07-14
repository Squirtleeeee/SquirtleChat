import axios, { type AxiosError } from 'axios'

export type ApiBody<T = unknown> = {
  code: number
  msg: string
  data?: T
}

export class ApiError extends Error {
  code: number

  constructor(message: string, code = -1) {
    super(message)
    this.name = 'ApiError'
    this.code = code
  }
}

const codeFallback: Record<number, string> = {
  40001: '请求参数不正确，请检查后重试',
  40101: '账号或密码错误，请重新输入',
  40301: '没有权限执行此操作',
  40401: '请求的资源不存在',
  40901: '操作冲突，请刷新后重试',
  50001: '服务器内部错误，请稍后重试',
}

const englishToZh: [string, string][] = [
  ['invalid credentials', '账号或密码错误，请重新输入'],
  ['username and password required', '用户名和密码不能为空'],
  ['duplicate entry', '数据已存在，请勿重复提交'],
  ['user not found', '用户不存在，请检查用户 ID'],
  ['not found', '请求的资源不存在'],
  ['cannot add self', '不能添加自己为好友'],
  ['not friends', '你们还不是好友，请先添加好友'],
  ['invalid message', '消息内容无效'],
  ['invalid token', '登录状态无效，请重新登录'],
  ['token required', '请先登录后再连接'],
  ['unauthorized', '未登录或登录已过期，请重新登录'],
  ['network error', '无法连接服务器，请确认后端服务已启动'],
  ['econnrefused', '无法连接服务器，请确认后端服务已启动'],
  ['timeout', '请求超时，请稍后重试'],
  ['request failed with status code 502', '服务网关错误，请确认后端服务已启动'],
  ['request failed with status code 401', '未登录或登录已过期，请重新登录'],
  ['request failed with status code 404', '请求的接口不存在，请重启后端服务'],
  ['request failed with status code 500', '服务器错误，请稍后重试'],
  ['field validation', '请填写完整的必填信息'],
]

function translateMsg(msg: string): string {
  const text = msg.trim()
  if (!text) return '操作失败，请稍后重试'
  if (/[\u4e00-\u9fff]/.test(text)) return text

  const lower = text.toLowerCase()
  if (lower.includes('duplicate entry') && lower.includes('username')) {
    return '用户名已被注册，请更换用户名或直接登录'
  }
  for (const [en, zh] of englishToZh) {
    if (lower.includes(en)) return zh
  }
  if (lower.includes('request failed with status code')) {
    return '网络请求失败，请稍后重试'
  }
  return '操作失败，请稍后重试'
}

export function unwrapApiData<T>(body: ApiBody<T>): T {
  if (body.code !== 0) {
    const msg = translateMsg(body.msg) || codeFallback[body.code] || '操作失败，请稍后重试'
    throw new ApiError(msg, body.code)
  }
  if (body.data === undefined) {
    throw new ApiError('服务器返回数据异常，请稍后重试', body.code)
  }
  return body.data
}

export function parseError(err: unknown): string {
  if (err instanceof ApiError) return err.message
  if (axios.isAxiosError(err)) return parseAxiosError(err)
  if (err instanceof Error && err.message) return translateMsg(err.message)
  return '操作失败，请稍后重试'
}

function parseAxiosError(err: AxiosError): string {
  const body = err.response?.data as ApiBody | undefined
  if (body?.msg) return translateMsg(body.msg)
  if (body?.code && codeFallback[body.code]) return codeFallback[body.code]

  const status = err.response?.status
  if (status === 502) return '服务网关错误，请确认后端服务已启动'
  if (status === 401) return '未登录或登录已过期，请重新登录'
  if (status === 404) return '请求的接口不存在，请重启后端服务（scripts/start-backend.ps1）'
  if (status && status >= 500) return '服务器错误，请稍后重试'
  if (!err.response) return '无法连接服务器，请检查网络或确认后端已启动'

  return translateMsg(err.message)
}
