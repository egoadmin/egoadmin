const CHANNEL_NAME = 'egoadmin-heartbeat'
const EVENT_KEY = 'egoadmin:heartbeat:event'
const LEADER_KEY = 'egoadmin:heartbeat:leader'
const LAST_BEAT_KEY = 'egoadmin:heartbeat:lastBeatAt'
const TABS_KEY = 'egoadmin:heartbeat:tabs'

const HEARTBEAT_INTERVAL_MS = 60_000
const LEADER_LEASE_MS = 150_000
const LEADER_CHECK_MS = 20_000
const TAB_LEASE_MS = 45_000

type HeartbeatMessageType = 'logout' | 'stop' | 'token-updated' | 'leader'

interface HeartbeatMessage {
  id: string
  tabId: string
  type: HeartbeatMessageType
  at: number
}

interface LeaderLease {
  tabId: string
  expiresAt: number
}

type TabRegistry = Record<string, number>

export interface HeartbeatCallbacks {
  hasToken: () => boolean
  getToken: () => string
  heartbeat: () => Promise<unknown>
  onLogout: () => void
  onTokenUpdated: () => void
  offlineOnPageLeave: () => boolean
}

function randomID() {
  if (typeof crypto !== 'undefined' && 'randomUUID' in crypto) {
    return crypto.randomUUID()
  }
  return `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

function readJSON<T>(key: string, fallback: T): T {
  const raw = localStorage.getItem(key)
  if (!raw) {
    return fallback
  }
  try {
    return JSON.parse(raw) as T
  } catch {
    return fallback
  }
}

function writeMessage(message: HeartbeatMessage) {
  localStorage.setItem(EVENT_KEY, JSON.stringify(message))
}

function getAPIBaseURL() {
  if (import.meta.env.DEV && import.meta.env.VITE_OPEN_PROXY === 'true') {
    return '/proxy/api'
  }
  return `${window.__APP_CONFIG__?.apiBaseUrl ?? import.meta.env.VITE_APP_API_BASEURL}/api`
}

function sendLogoutKeepalive(token: string) {
  if (!token) {
    return
  }
  void fetch(`${getAPIBaseURL()}/user.v1.UserService/Logout`, {
    method: 'POST',
    headers: {
      Authorization: `Bearer ${token}`,
      'Content-Type': 'application/json',
    },
    body: '{}',
    keepalive: true,
  }).catch(() => {})
}

class HeartbeatCoordinator {
  private readonly tabId = randomID()
  private callbacks: HeartbeatCallbacks | null = null
  private channel: BroadcastChannel | null = null
  private checkTimer: ReturnType<typeof setInterval> | null = null
  private heartbeatTimer: ReturnType<typeof setTimeout> | null = null
  private tabTimer: ReturnType<typeof setInterval> | null = null
  private started = false

  start(callbacks: HeartbeatCallbacks) {
    this.callbacks = callbacks
    if (!callbacks.hasToken()) {
      this.stop()
      return
    }
    if (!this.started) {
      this.started = true
      this.channel = this.createChannel()
      window.addEventListener('storage', this.handleStorage)
      window.addEventListener('visibilitychange', this.handleVisibilityChange)
      window.addEventListener('pagehide', this.handlePageHide)
      this.tabTimer = setInterval(() => this.renewTab(), LEADER_CHECK_MS)
      this.checkTimer = setInterval(() => this.checkLeader(), LEADER_CHECK_MS)
    }
    this.renewTab()
    this.checkLeader()
  }

  stop(options: { broadcast?: boolean; clearLeader?: boolean } = {}) {
    if (options.broadcast) {
      this.post('stop')
    }
    this.stopHeartbeat()
    if (options.clearLeader !== false) {
      this.clearLeader()
    }
    this.removeTab()
    if (this.checkTimer) {
      clearInterval(this.checkTimer)
      this.checkTimer = null
    }
    if (this.tabTimer) {
      clearInterval(this.tabTimer)
      this.tabTimer = null
    }
    if (this.started) {
      window.removeEventListener('storage', this.handleStorage)
      window.removeEventListener('visibilitychange', this.handleVisibilityChange)
      window.removeEventListener('pagehide', this.handlePageHide)
      this.channel?.close()
      this.channel = null
      this.started = false
    }
  }

  broadcastLogout() {
    this.post('logout')
    this.stop()
  }

  broadcastTokenUpdated() {
    this.post('token-updated')
  }

  private createChannel() {
    if (typeof BroadcastChannel === 'undefined') {
      return null
    }
    const channel = new BroadcastChannel(CHANNEL_NAME)
    channel.onmessage = (event: MessageEvent<HeartbeatMessage>) => {
      this.handleMessage(event.data)
    }
    return channel
  }

  private post(type: HeartbeatMessageType) {
    const message: HeartbeatMessage = {
      id: randomID(),
      tabId: this.tabId,
      type,
      at: Date.now(),
    }
    this.channel?.postMessage(message)
    writeMessage(message)
  }

  private handleMessage(message: HeartbeatMessage | null | undefined) {
    if (!message || message.tabId === this.tabId) {
      return
    }
    switch (message.type) {
      case 'logout':
        this.callbacks?.onLogout()
        this.stop({ clearLeader: true })
        break
      case 'stop':
        this.stop({ clearLeader: true })
        break
      case 'token-updated':
        this.callbacks?.onTokenUpdated()
        if (this.callbacks?.hasToken()) {
          this.checkLeader()
        }
        break
      case 'leader':
        if (!this.isLeader()) {
          this.stopHeartbeat()
        }
        break
    }
  }

  private readonly handleStorage = (event: StorageEvent) => {
    if (event.key !== EVENT_KEY || !event.newValue) {
      return
    }
    try {
      this.handleMessage(JSON.parse(event.newValue) as HeartbeatMessage)
    } catch {
      // Ignore malformed cross-tab coordination messages.
    }
  }

  private readonly handleVisibilityChange = () => {
    if (document.visibilityState === 'visible') {
      this.renewTab()
      this.checkLeader()
    }
  }

  private readonly handlePageHide = () => {
    const token = this.callbacks?.getToken() ?? ''
    this.removeTab()
    if (this.isLeader()) {
      this.clearLeader()
    }
    if (this.callbacks?.offlineOnPageLeave() && this.isLastActiveTab()) {
      sendLogoutKeepalive(token)
    }
  }

  private renewTab() {
    const tabs = this.activeTabs()
    tabs[this.tabId] = Date.now() + TAB_LEASE_MS
    localStorage.setItem(TABS_KEY, JSON.stringify(tabs))
  }

  private removeTab() {
    const tabs = readJSON<TabRegistry>(TABS_KEY, {})
    delete tabs[this.tabId]
    localStorage.setItem(TABS_KEY, JSON.stringify(tabs))
  }

  private activeTabs() {
    const now = Date.now()
    const tabs = readJSON<TabRegistry>(TABS_KEY, {})
    return Object.entries(tabs).reduce<TabRegistry>((acc, [tabId, expiresAt]) => {
      if (expiresAt > now) {
        acc[tabId] = expiresAt
      }
      return acc
    }, {})
  }

  private isLastActiveTab() {
    const tabs = this.activeTabs()
    return Object.keys(tabs).length === 0
  }

  private checkLeader() {
    if (!this.callbacks?.hasToken()) {
      this.stop()
      return
    }
    this.renewTab()
    if (this.isLeader()) {
      this.renewLeader()
      this.startHeartbeat()
      return
    }
    const lease = this.readLeader()
    if (!lease || lease.expiresAt <= Date.now()) {
      this.becomeLeader()
      return
    }
    this.stopHeartbeat()
  }

  private readLeader() {
    return readJSON<LeaderLease | null>(LEADER_KEY, null)
  }

  private isLeader() {
    const lease = this.readLeader()
    return lease?.tabId === this.tabId && lease.expiresAt > Date.now()
  }

  private renewLeader() {
    localStorage.setItem(
      LEADER_KEY,
      JSON.stringify({
        tabId: this.tabId,
        expiresAt: Date.now() + LEADER_LEASE_MS,
      }),
    )
  }

  private becomeLeader() {
    this.renewLeader()
    if (!this.isLeader()) {
      return
    }
    this.post('leader')
    this.startHeartbeat(true)
  }

  private clearLeader() {
    if (this.isLeader()) {
      localStorage.removeItem(LEADER_KEY)
    }
  }

  private startHeartbeat(immediate = false) {
    if (this.heartbeatTimer) {
      return
    }
    const delay = immediate ? 0 : HEARTBEAT_INTERVAL_MS
    this.heartbeatTimer = setTimeout(() => {
      this.heartbeatTimer = null
      void this.runHeartbeat()
    }, delay)
  }

  private stopHeartbeat() {
    if (this.heartbeatTimer) {
      clearTimeout(this.heartbeatTimer)
      this.heartbeatTimer = null
    }
  }

  private async runHeartbeat() {
    if (!this.callbacks?.hasToken() || !this.isLeader()) {
      this.stopHeartbeat()
      return
    }
    this.renewLeader()
    try {
      await this.callbacks.heartbeat()
      localStorage.setItem(LAST_BEAT_KEY, `${Date.now()}`)
    } finally {
      if (this.callbacks?.hasToken() && this.isLeader()) {
        this.startHeartbeat()
      }
    }
  }
}

const coordinator = new HeartbeatCoordinator()

export function startHeartbeat(callbacks: HeartbeatCallbacks) {
  coordinator.start(callbacks)
}

export function stopHeartbeat(options?: { broadcast?: boolean; clearLeader?: boolean }) {
  coordinator.stop(options)
}

export function broadcastHeartbeatLogout() {
  coordinator.broadcastLogout()
}

export function broadcastHeartbeatTokenUpdated() {
  coordinator.broadcastTokenUpdated()
}
