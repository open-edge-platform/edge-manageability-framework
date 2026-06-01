/*********************************************************************
 * KVM Service — connects Angular app to Go KVM server
 * Go server handles: JWT → redirect token → MPS WebSocket → Binary relay
 * This service: REST (connect/disconnect/status) + WebSocket (KVM binary data)
 *********************************************************************/
import { Injectable } from '@angular/core'
import { Subject, BehaviorSubject } from 'rxjs'

export interface KvmConfig {
  mpsHost: string
  deviceGuid: string
  port: number
  mode: string
  jwtToken: string
}

/**
 * Device states matching toolkit convention:
 *   0 = disconnected, 1 = connecting, 2 = connected, 3 = active/ready
 */
export type DeviceState = 0 | 1 | 2 | 3

@Injectable({ providedIn: 'root' })
export class KvmService {
  /** Binary KVM data (RFB frames) from AMT device */
  readonly kvmData$ = new Subject<ArrayBuffer>()

  /** Log messages for display */
  readonly logMessage$ = new Subject<string>()

  /** Device state (0=off, 1=connecting, 2=connected, 3=active) */
  readonly deviceState$ = new BehaviorSubject<DeviceState>(0)

  /**
   * Increments each time a new KVM connection starts.
   * Viewer components subscribe/watch this to reset their RFB state machine.
   */
  readonly connectionId$ = new BehaviorSubject<number>(0)

  /** Go server base URL (empty = same origin via proxy) */
  private serverUrl = ''

  private socket: WebSocket | null = null
  private lastConfig: KvmConfig | null = null   // cached for auto-reconnect
  private userDisconnected = false              // true only when user calls disconnect()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private reconnectAttempts = 0
  private readonly maxReconnectAttempts = 10
  private readonly minReconnectDelayMs = 3000  // AMT needs ~3s to free its KVM session

  /** Connect to AMT device via the Go KVM server */
  async connect(config: KvmConfig): Promise<void> {
    this.lastConfig = config
    this.userDisconnected = false
    this.reconnectAttempts = 0   // only reset on explicit user-initiated connect
    // Signal viewer components to reset their RFB state machine
    this.connectionId$.next(this.connectionId$.value + 1)
    this.deviceState$.next(1) // connecting

    try {
      await this.startMpsSession(config)
      await this.openWebSocket()
    } catch (err: any) {
      this.log(`[ERROR] ${err.message || err}`)
      this.deviceState$.next(0)
      throw err
    }
  }

  /** Disconnect from the AMT device */
  async disconnect(): Promise<void> {
    this.userDisconnected = true
    this.cancelReconnect()
    if (this.socket) {
      this.socket.close()
      this.socket = null
    }

    try {
      await fetch(`${this.serverUrl}/api/disconnect`, { method: 'POST' })
    } catch (_) {}

    this.deviceState$.next(0)
    this.log('[*] Disconnected')
  }

  /** Send mouse/keyboard events to AMT via Go server */
  sendData(data: ArrayBuffer): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(data)
    }
  }

  // ── Private ──────────────────────────────────────────────────────

  private scheduleReconnect(): void {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      this.log('[ERROR] Max reconnect attempts reached — giving up')
      return
    }
    // Exponential backoff with a hard minimum of minReconnectDelayMs.
    // AMT KVM module needs ~3 seconds to fully tear down the previous session
    // before it will accept a new connection on the same channel.
    const backoff = 1000 * Math.pow(2, this.reconnectAttempts)
    const delayMs = Math.max(this.minReconnectDelayMs, Math.min(backoff, 30000))
    this.reconnectAttempts++
    this.log(`[*] MPS session dropped — reconnecting in ${delayMs / 1000}s (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})...`)
    this.reconnectTimer = setTimeout(async () => {
      if (this.userDisconnected || !this.lastConfig) return
      try {
        // Check if the Go server session is still alive before doing a full
        // POST /api/connect (which creates a new MPS WS and new AMT handshake).
        // If the session is already active, just reopen the browser WebSocket.
        const sessionState = await this.fetchSessionState()
        if (sessionState === 'error' || sessionState === 'disconnected') {
          // Terminal state — the Go session is dead and cannot self-recover.
          // Stop retrying; operator must start a new session.
          this.log('[*] Go server session ended — not reconnecting')
          this.deviceState$.next(0)
          return
        }
        if (sessionState === 'active') {
          this.log('[*] Go server session still active — reopening WebSocket only')
          this.connectionId$.next(this.connectionId$.value + 1)
          this.deviceState$.next(3)
          await this.openWebSocket()
        } else {
          // Session is still in progress (connecting/authenticating) — wait for it
          this.connectionId$.next(this.connectionId$.value + 1)
          this.deviceState$.next(1)
          await this.startMpsSession(this.lastConfig)
          await this.openWebSocket()
        }
      } catch (_) {
        if (!this.userDisconnected) {
          this.scheduleReconnect()
        }
      }
    }, delayMs)
  }

  private cancelReconnect(): void {
    if (this.reconnectTimer !== null) {
      clearTimeout(this.reconnectTimer)
      this.reconnectTimer = null
    }
    this.reconnectAttempts = 0
  }

  /** POST /api/connect and wait for Go server to reach active state */
  private async startMpsSession(config: KvmConfig): Promise<void> {
    this.log('[*] Sending connect request to Go server...')
    const resp = await fetch(`${this.serverUrl}/api/connect`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        mpsHost: config.mpsHost,
        deviceGuid: config.deviceGuid,
        port: config.port,
        mode: config.mode,
        jwtToken: config.jwtToken,
      }),
    })
    if (!resp.ok) {
      const body = await resp.text()
      throw new Error(`Connect failed (${resp.status}): ${body}`)
    }
    this.log('[OK] Go server connecting to MPS/AMT...')
    this.deviceState$.next(2)
    await this.waitForActive(20000)
  }

  /** Open the /ws/kvm WebSocket and attach handlers */
  private async openWebSocket(): Promise<void> {
    this.log('[*] Opening KVM WebSocket...')
    const wsProto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    const wsUrl = `${wsProto}//${window.location.host}/ws/kvm`
    this.socket = new WebSocket(wsUrl)
    this.socket.binaryType = 'arraybuffer'

    await new Promise<void>((resolve, reject) => {
      this.socket!.onopen = () => {
        this.log('[OK] KVM WebSocket connected')
        this.deviceState$.next(3)
        resolve()
      }
      this.socket!.onerror = () => {
        this.log('[ERROR] KVM WebSocket error')
        reject(new Error('WebSocket error'))
      }
    })

    this.socket.onmessage = (event: MessageEvent) => {
      this.kvmData$.next(event.data as ArrayBuffer)
    }

    this.socket.onclose = (event: CloseEvent) => {
      this.log(`[*] KVM WebSocket closed (code=${event.code})`)
      const wasActive = this.deviceState$.value === 3
      if (this.deviceState$.value !== 0) {
        this.deviceState$.next(0)
      }
      const isCleanClose = event.code === 1000 || event.code === 1001
      if (!this.userDisconnected && wasActive && this.lastConfig && !isCleanClose) {
        this.scheduleReconnect()
      }
    }
  }

  /** Fetch current session state from Go server without throwing */
  private async fetchSessionState(): Promise<string> {
    try {
      const resp = await fetch(`${this.serverUrl}/api/status`)
      const status = await resp.json()
      return status.state ?? 'disconnected'
    } catch {
      return 'disconnected'
    }
  }

  private async waitForActive(timeoutMs: number): Promise<void> {
    const start = Date.now()
    while (Date.now() - start < timeoutMs) {
      try {
        const resp = await fetch(`${this.serverUrl}/api/status`)
        const status = await resp.json()

        if (status.state === 'active') {
          this.log('[OK] KVM session is active!')
          return
        }
        if (status.state === 'connecting' || status.state === 'authenticating') {
          this.log(`[*] Waiting for KVM session to become active... (state=${status.state})`)
        }
        if (status.state === 'error' || status.state === 'disconnected') {
          throw new Error(`KVM session failed on server (state=${status.state})`)
        }
      } catch (err: any) {
        if (err.message?.includes('failed')) throw err
      }
      await new Promise(r => setTimeout(r, 500))
    }
    this.log('[WARN] Timeout waiting for active, opening WebSocket anyway...')
  }

  private log(msg: string): void {
    console.log(msg)
    this.logMessage$.next(msg)
  }
}
