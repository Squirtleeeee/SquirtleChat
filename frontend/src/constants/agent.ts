/** Built-in AI assistant bot account (backend username). */

export const AGENT_USERNAME = 'squirtle_ai'

export const AGENT_NICKNAME = '杰尼龟龟'

/** Local Squirtle artwork (PokeAPI official-artwork #7). */

export const AGENT_AVATAR = '/agent/squirtle.png'



export function isAgentProfile(user?: { username?: string } | null) {

  return user?.username === AGENT_USERNAME

}



export function agentAvatarSrc(user?: { username?: string } | null): string {

  return isAgentProfile(user) ? AGENT_AVATAR : ''

}



export function agentDisplayName(user?: { username?: string; nickname?: string } | null): string {

  return isAgentProfile(user) ? AGENT_NICKNAME : user?.nickname || user?.username || ''

}


