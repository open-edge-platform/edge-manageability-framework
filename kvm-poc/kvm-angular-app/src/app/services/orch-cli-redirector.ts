import { KvmService } from './kvm.service'
import { Subscription } from 'rxjs'

/**
 * Adapts KvmService (Go orch-cli WebSocket) to the ICommunicator/IKvmDataCommunicator
 * interface expected by DMT's AMTDesktop + DataProcessor.
 *
 * The Go server handles the full AMT protocol handshake (0x10→0x41) and becomes
 * a pure RFB byte relay once active.  This redirector skips the AMT handshake and
 * fires onStart() as soon as the KVM session is active (deviceState$ === 3),
 * letting DMT's DataProcessor drive the RFB state machine from byte zero.
 */
export class OrchCliRedirector {
  // Callbacks wired by the caller (same interface as AMTKvmDataRedirector)
  onProcessData: (data: string) => void = () => {}
  onStart: () => void = () => {}
  onNewState: (state: number) => void = () => {}
  onStateChanged: (redirector: OrchCliRedirector, state: number) => void = () => {}
  onError: () => void = () => {}
  onSendKvmData: (data: string) => void = () => {}

  private dataSub: Subscription | null = null
  private stateSub: Subscription | null = null
  private started = false

  constructor(private readonly kvmService: KvmService) {}

  /**
   * Start listening.  Fires onStart() once deviceState$ reaches 3 (active).
   * The WebSocket constructor argument is accepted for interface compatibility
   * but is unused — KvmService owns the WebSocket connection.
   */
  start(_WebSocketClass: unknown): void {
    this.started = false

    // Subscribe to binary RFB data from Go server → convert to binary string → DataProcessor
    this.dataSub = this.kvmService.kvmData$.subscribe((buf: ArrayBuffer) => {
      const bytes = new Uint8Array(buf)
      let str = ''
      for (let i = 0; i < bytes.length; i++) str += String.fromCharCode(bytes[i])
      this.onProcessData(str)
    })

    // Subscribe to session state: fire onStart() exactly once when KVM becomes active.
    // BehaviorSubject emits current value on subscribe, so if state is already 3
    // (e.g. reconnect path), onStart() fires immediately — which is correct.
    this.stateSub = this.kvmService.deviceState$.subscribe((state) => {
      if (state === 3 && !this.started) {
        this.started = true
        this.onNewState(3)
        this.onStateChanged(this, 3)
        this.onStart()
      } else if (state === 0 && this.started) {
        this.started = false
        this.onNewState(0)
        this.onStateChanged(this, 0)
      }
    })
  }

  /** Send RFB binary string upstream: binary-string → Uint8Array → WebSocket */
  send(data: string): void {
    const buf = new Uint8Array(data.length)
    for (let i = 0; i < data.length; i++) buf[i] = data.charCodeAt(i)
    this.kvmService.sendData(buf.buffer as ArrayBuffer)
  }

  /** Alias of send() — KVM path has no extra AMT framing */
  socketSend(data: string): void {
    this.send(data)
  }

  stop(): void {
    this.dataSub?.unsubscribe()
    this.stateSub?.unsubscribe()
    this.dataSub = null
    this.stateSub = null
    this.started = false
  }
}
