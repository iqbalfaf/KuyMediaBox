import { api, runtime } from '../api'
import type { SubscriptionInfo } from '../types'

/** Followed channels, playlists and profiles. */
export const subs = $state<{ list: SubscriptionInfo[]; open: boolean }>({ list: [], open: false })

let listening = false

export async function loadSubs() {
  if (!listening) {
    listening = true
    runtime.on('subscriptions:changed', (list: SubscriptionInfo[]) => (subs.list = list ?? []))
  }
  try {
    subs.list = (await api.listSubscriptions()) ?? []
  } catch {
    /* stays empty */
  }
}
