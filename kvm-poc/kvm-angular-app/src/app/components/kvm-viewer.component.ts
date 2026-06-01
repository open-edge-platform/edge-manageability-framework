import {
  Component,
  Input,
  OnDestroy,
  OnChanges,
  SimpleChanges,
  ViewChild,
  ElementRef,
  AfterViewInit,
  NgZone,
  PLATFORM_ID,
  Inject,
} from '@angular/core'
import { CommonModule, isPlatformBrowser } from '@angular/common'
import {
  AMTDesktop,
  DataProcessor,
  MouseHelper,
  KeyBoardHelper,
  CommsHelper,
} from '@device-management-toolkit/ui-toolkit/core'
import { KvmService } from '../services/kvm.service'
import { OrchCliRedirector } from '../services/orch-cli-redirector'

@Component({
  selector: 'app-kvm-viewer',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="kvm-viewer" (click)="focusCanvas()">
      <canvas
        #kvmCanvas
        tabindex="0"
        (mousemove)="onMouseMove($event)"
        (mousedown)="onMouseDown($event)"
        (mouseup)="onMouseUp($event)"
        (contextmenu)="$event.preventDefault(); $event.stopPropagation()"
      ></canvas>

      <div class="kvm-toolbar" *ngIf="isActive">
        <button
          class="btn-cad"
          (click)="sendCtrlAltDel(); $event.stopPropagation()"
          title="Send Ctrl+Alt+Del to remote device"
        >
          Ctrl+Alt+Del
        </button>
      </div>

      <div class="connecting" *ngIf="!isActive">
        <div class="connecting-text">{{ statusText }}</div>
      </div>

      <div class="stats" *ngIf="showStats">
        <div>State: {{ connectionState }}</div>
        <div>Size: {{ canvasWidth }}x{{ canvasHeight }}</div>
      </div>
    </div>
  `,
  styles: [
    `
      .kvm-viewer {
        position: relative;
        display: flex;
        flex-direction: column;
        justify-content: center;
        align-items: center;
        background: #000;
        min-height: 400px;
      }

      canvas {
        border: 2px solid #667eea;
        max-width: 100%;
        height: auto;
        cursor: crosshair;
        outline: none;
        display: block;
      }

      canvas:focus {
        border-color: #00ff88;
        box-shadow: 0 0 0 2px rgba(0, 255, 136, 0.4);
      }

      .kvm-toolbar {
        position: absolute;
        top: 8px;
        left: 8px;
        z-index: 10;
      }

      .btn-cad {
        background: rgba(30, 30, 30, 0.85);
        color: #e0e0e0;
        border: 1px solid #555;
        border-radius: 4px;
        padding: 4px 10px;
        font-size: 11px;
        cursor: pointer;
      }

      .btn-cad:hover {
        background: rgba(60, 60, 60, 0.95);
        border-color: #888;
      }

      .connecting {
        position: absolute;
        top: 50%;
        left: 50%;
        transform: translate(-50%, -50%);
        color: #0f0;
        font-family: monospace;
        font-size: 14px;
        text-align: center;
        pointer-events: none;
      }

      .stats {
        position: absolute;
        bottom: 8px;
        right: 8px;
        background: rgba(0, 0, 0, 0.7);
        color: #0f0;
        font-family: monospace;
        font-size: 11px;
        padding: 4px 8px;
        border-radius: 3px;
        pointer-events: none;
      }
    `,
  ],
})
export class KvmViewerComponent implements AfterViewInit, OnDestroy, OnChanges {
  @ViewChild('kvmCanvas', { static: true }) canvasRef!: ElementRef<HTMLCanvasElement>

  /**
   * Incremented by the parent whenever a new KVM connection starts.
   * The viewer tears down and re-initialises DMT on each change.
   */
  @Input() epoch: number | null = 0

  @Input() showStats = false

  // View state
  isActive = false
  statusText = 'Connecting...'
  connectionState = 'Disconnected'
  canvasWidth = 1024
  canvasHeight = 768

  private desktop: AMTDesktop | null = null
  private redirector: OrchCliRedirector | null = null
  private dataProcessor: DataProcessor | null = null
  private mouseHelper: MouseHelper | null = null
  private keyboardHelper: KeyBoardHelper | null = null

  /** Coarse throttle for mousemove - matches DMT's 200 ms throttleTime */
  private lastMoveTime = 0

  constructor(
    private readonly kvmService: KvmService,
    private readonly ngZone: NgZone,
    @Inject(PLATFORM_ID) private readonly platformId: object,
  ) {}

  ngAfterViewInit(): void {
    if (!isPlatformBrowser(this.platformId)) return
    this.init()
  }

  ngOnChanges(changes: SimpleChanges): void {
    if (
      changes['epoch'] &&
      !changes['epoch'].firstChange &&
      isPlatformBrowser(this.platformId)
    ) {
      this.teardown()
      this.init()
    }
  }

  ngOnDestroy(): void {
    this.teardown()
  }

  focusCanvas(): void {
    this.canvasRef?.nativeElement.focus()
  }

  onMouseMove(e: MouseEvent): void {
    const now = Date.now()
    if (now - this.lastMoveTime < 50) return
    this.lastMoveTime = now
    this.mouseHelper?.mousemove(e)
  }

  onMouseDown(e: MouseEvent): void {
    this.mouseHelper?.mousedown(e)
  }

  onMouseUp(e: MouseEvent): void {
    this.mouseHelper?.mouseup(e)
  }

  sendCtrlAltDel(): void {
    if (this.redirector) {
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      CommsHelper.sendCtrlAltDelMsg(this.redirector as any)
    }
  }

  // ── Private ──────────────────────────────────────────────────────────

  private init(): void {
    if (!isPlatformBrowser(this.platformId)) return

    const canvas = this.canvasRef.nativeElement
    const ctx = canvas.getContext('2d')
    if (!ctx) return

    // Run DMT initialisation outside Angular zone so that the very
    // frequent putImageData / onProcessData calls do not trigger
    // Angular change detection on every RFB frame.
    this.ngZone.runOutsideAngular(() => {
      // eslint-disable-next-line @typescript-eslint/no-non-null-assertion
      const redirector = new OrchCliRedirector(this.kvmService)
      const desktop = new AMTDesktop(ctx)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const dataProcessor = new DataProcessor(redirector as any, desktop)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const mouseHelper = new MouseHelper(desktop, redirector as any, 200)
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const keyboardHelper = new KeyBoardHelper(desktop, redirector as any)

      this.redirector = redirector
      this.desktop = desktop
      this.dataProcessor = dataProcessor
      this.mouseHelper = mouseHelper
      this.keyboardHelper = keyboardHelper

      // ── Wire callbacks (identical order to DMT kvm.component.ts) ─────
      redirector.onProcessData = desktop.processData.bind(desktop)
      redirector.onStart = desktop.start.bind(desktop)
      redirector.onNewState = desktop.onStateChange.bind(desktop)
      redirector.onSendKvmData = desktop.onSendKvmData.bind(desktop)

      redirector.onStateChanged = (_r: OrchCliRedirector, state: number) => {
        // Re-enter Angular zone for UI state updates only
        this.ngZone.run(() => {
          this.isActive = state === 3
          this.connectionState = state === 3 ? 'Active' : 'Disconnected'
          this.statusText = state === 3 ? '' : 'Connecting...'
        })
      }

      redirector.onError = () => {
        this.ngZone.run(() => {
          this.isActive = false
          this.statusText = 'Connection error'
        })
      }

      desktop.onSend = redirector.send.bind(redirector)
      desktop.onProcessData = dataProcessor.processData.bind(dataProcessor)

      // 8-bit RGB332 — DMT default (bpp=1); switch to 2 for 16-bit RGB565
      desktop.bpp = 1

      // Canvas resize: update DOM directly (avoids Angular binding clearing the canvas)
      desktop.onScreenSizeChange = (width: number, height: number) => {
        canvas.width = width
        canvas.height = height
        this.canvasWidth = width
        this.canvasHeight = height
        mouseHelper.resetOffsets()
      }

      // Start: redirector subscribes to KvmService state/data streams.
      // onStart() fires once deviceState$ reaches 3 (active).
      redirector.start(WebSocket)
      keyboardHelper.GrabKeyInput()
    })
  }

  private teardown(): void {
    this.keyboardHelper?.UnGrabKeyInput()
    this.redirector?.stop()
    this.desktop = null
    this.redirector = null
    this.dataProcessor = null
    this.mouseHelper = null
    this.keyboardHelper = null
    this.isActive = false
    this.statusText = 'Connecting...'
  }
}
