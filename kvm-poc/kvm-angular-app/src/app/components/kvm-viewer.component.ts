import { Component, Input, OnInit, OnDestroy, OnChanges, SimpleChanges, ViewChild, ElementRef, ChangeDetectorRef, PLATFORM_ID, Inject } from '@angular/core';
import { CommonModule, isPlatformBrowser } from '@angular/common';
import { Subject } from 'rxjs';

interface PixelFormat {
  bitsPerPixel: number;
  bigEndian: boolean;
  trueColour: boolean;
  redMax: number;
  greenMax: number;
  blueMax: number;
  redShift: number;
  greenShift: number;
  blueShift: number;
}

@Component({
  selector: 'app-kvm-viewer',
  standalone: true,
  imports: [CommonModule],
  template: `
    <div class="kvm-viewer" (click)="focusCanvas()">
      <canvas #kvmCanvas tabindex="0"></canvas>
      <div class="stats" *ngIf="showStats">
        <div>State: {{ rfbState }}</div>
        <div>FPS: {{ fps }}</div>
        <div>Frames: {{ frameCount }}</div>
        <div>Size: {{ canvasWidth }}x{{ canvasHeight }}</div>
        <div>Buffer: {{ dataBuffer.length }} bytes</div>
      </div>
    </div>
  `,
  styles: [`
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
    
    .stats {
      position: absolute;
      top: 10px;
      right: 10px;
      background: rgba(0, 0, 0, 0.7);
      color: #0f0;
      padding: 10px;
      font-family: monospace;
      font-size: 12px;
      border-radius: 4px;
    }
  `]
})
export class KvmViewerComponent implements OnInit, OnDestroy, OnChanges {
  @ViewChild('kvmCanvas', { static: true }) canvasRef!: ElementRef<HTMLCanvasElement>;
  @Input() kvmData$!: Subject<ArrayBuffer>;
  @Input() showStats = true;
  /** Incremented by the parent each time a new KVM session starts — triggers RFB state reset */
  @Input() epoch: number | null = 0;

  private canvas!: HTMLCanvasElement;
  private ctx!: CanvasRenderingContext2D;
  private sendDataCallback?: (data: ArrayBuffer) => void;
  private boundMouseUp?: (e: MouseEvent) => void;

  canvasWidth = 0;
  canvasHeight = 0;
  frameCount = 0;
  fps = 0;
  rfbState = 'ProtocolVersion'; // Track RFB handshake state (public for template)

  private lastFrameTime = 0;
  dataBuffer = new Uint8Array(0); // public for stats display
  private buttonMask = 0;

  private pixelFormat: PixelFormat = {
    bitsPerPixel: 32,
    bigEndian: false,
    trueColour: true,
    redMax: 255,   greenMax: 255,   blueMax: 255,
    redShift: 16,  greenShift: 8,   blueShift: 0,
  };

  constructor(private cdr: ChangeDetectorRef, @Inject(PLATFORM_ID) private platformId: Object) {}

  ngOnInit() {
    if (!isPlatformBrowser(this.platformId)) {
      console.log('[KVM Viewer] Skipping initialization on server side');
      return;
    }
    this.canvas = this.canvasRef.nativeElement;
    this.ctx = this.canvas.getContext('2d')!;

    // Mouse events
    this.canvas.addEventListener('mousemove',   this.onMouseMove.bind(this));
    this.canvas.addEventListener('mousedown',   this.onMouseButton.bind(this));
    this.canvas.addEventListener('mouseenter',  this.onMouseEnter.bind(this));
    this.canvas.addEventListener('contextmenu', (e) => e.preventDefault());
    // mouseup on document so button release is always captured even if mouse leaves canvas
    this.boundMouseUp = this.onMouseButton.bind(this);
    document.addEventListener('mouseup', this.boundMouseUp);
    // Keyboard events
    this.canvas.addEventListener('keydown', this.onKeyEvent.bind(this));
    this.canvas.addEventListener('keyup',   this.onKeyEvent.bind(this));
    // Auto-focus so keyboard works immediately after canvas appears
    this.canvas.focus();
    
    // Subscribe to KVM data from service
    this.kvmData$.subscribe((data: ArrayBuffer) => {
      this.handleKvmData(data);
    });
    
    console.log('[KVM Viewer] Initialized, waiting for RFB handshake...');
  }
  
  ngOnChanges(changes: SimpleChanges): void {
    // Reset the RFB state machine whenever the parent starts a new KVM session.
    // This prevents the stale 'Normal' state from a previous session mishandling
    // the new session's ProtocolVersion string and breaking the handshake.
    if (changes['epoch'] && !changes['epoch'].firstChange) {
      console.log('[KVM Viewer] New session epoch — resetting RFB state machine');
      this.resetRfbState();
    }
  }

  // Public method to reset internal RFB state (called on reconnect via epoch input)
  resetRfbState(): void {
    this.rfbState = 'ProtocolVersion';
    this.dataBuffer = new Uint8Array(0);
    this.canvasWidth = 0;
    this.canvasHeight = 0;
    this.frameCount = 0;
    this.fps = 0;
    this.buttonMask = 0;
    this.pixelFormat = {
      bitsPerPixel: 32, bigEndian: false, trueColour: true,
      redMax: 255, greenMax: 255, blueMax: 255,
      redShift: 16, greenShift: 8, blueShift: 0,
    };
    if (this.ctx && this.canvas) {
      this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
    }
    this.cdr.detectChanges();
    console.log('[KVM Viewer] RFB state reset — ready for new session');
  }

  // Public method to set the sendData callback (called by parent)
  setSendCallback(callback: (data: ArrayBuffer) => void) {
    this.sendDataCallback = callback;
  }
  
  ngOnDestroy() {
    if (this.boundMouseUp) {
      document.removeEventListener('mouseup', this.boundMouseUp);
    }
  }
  
  private handleKvmData(data: ArrayBuffer) {
    if (!isPlatformBrowser(this.platformId)) return;
    try {
      const uint8 = new Uint8Array(data);
      const hexPreview = Array.from(uint8.slice(0, 20)).map(b => b.toString(16).padStart(2,'0')).join(' ');
      console.log(`[RFB←Server] ${uint8.byteLength} bytes [${this.rfbState}]: ${hexPreview}`);
      
      // Append to buffer
      const newBuffer = new Uint8Array(this.dataBuffer.length + uint8.length);
      newBuffer.set(this.dataBuffer);
      newBuffer.set(uint8, this.dataBuffer.length);
      this.dataBuffer = newBuffer;
      console.log(`[RFB] Total buffer: ${this.dataBuffer.length} bytes, state: ${this.rfbState}`);
      
      // Process RFB messages based on state
      this.processRfbMessages();
      
    } catch (error) {
      console.error('[KVM Viewer] Error handling data:', error);
    }
  }
  
  private processRfbMessages() {
    while (true) {
      if (this.rfbState === 'ProtocolVersion') {
        if (this.dataBuffer.length < 12) return;
        const version = new TextDecoder().decode(this.dataBuffer.slice(0, 12));
        console.log(`[RFB] Server version: ${version.trim()}`);
        this.sendToServer(new TextEncoder().encode('RFB 003.008\n'));
        this.dataBuffer = this.dataBuffer.slice(12);
        this.rfbState = 'Security';
        this.cdr.detectChanges();
        continue;
      }
      
      if (this.rfbState === 'Security') {
        console.log(`[RFB] Security: buffer=${this.dataBuffer.length} bytes, first bytes: ${Array.from(this.dataBuffer.slice(0,4)).map(b=>'0x'+b.toString(16)).join(' ')}`);
        if (this.dataBuffer.length < 1) return;
        const numSecTypes = this.dataBuffer[0];
        console.log(`[RFB] Security: numSecTypes=${numSecTypes}`);
        if (numSecTypes === 0) { console.error('[RFB] Server rejected connection'); return; }
        if (this.dataBuffer.length < 1 + numSecTypes) { console.log(`[RFB] Security: need ${1+numSecTypes} bytes, have ${this.dataBuffer.length}`); return; }
        const secTypes = Array.from(this.dataBuffer.slice(1, 1 + numSecTypes));
        console.log(`[RFB] Security types offered: [${secTypes.join(', ')}]`);
        // Prefer None(1), otherwise pick first offered
        const chosen = secTypes.includes(1) ? 1 : secTypes[0];
        this.sendToServer(new Uint8Array([chosen]));
        console.log(`[RFB] Selected security type: ${chosen} → next state: ${chosen === 2 ? 'VNCAuth' : 'SecurityResult'}`);
        this.dataBuffer = this.dataBuffer.slice(1 + numSecTypes);
        this.rfbState = chosen === 2 ? 'VNCAuth' : 'SecurityResult';
        this.cdr.detectChanges();
        continue;
      }

      if (this.rfbState === 'VNCAuth') {
        // VNC Authentication: server sends 16-byte challenge, client sends DES-encrypted response
        // In CCM mode the AMT password is typically empty, so send 16 zero bytes
        if (this.dataBuffer.length < 16) return;
        console.warn('[RFB] VNC Auth challenge - sending null response (CCM/no-password)');
        this.sendToServer(new Uint8Array(16));
        this.dataBuffer = this.dataBuffer.slice(16);
        this.rfbState = 'SecurityResult';
        this.cdr.detectChanges();
        continue;
      }
      
      if (this.rfbState === 'SecurityResult') {
        // AMT pads its Security types frame to a 4-byte boundary.
        // After consuming the 3-byte security list (1 count + 2 types), a stray 0x80
        // byte remains that is NOT part of SecurityResult. Skip it before reading.
        while (this.dataBuffer.length >= 1 && this.dataBuffer[0] === 0x80) {
          console.log('[RFB] Skipping AMT 0x80 padding byte before SecurityResult');
          this.dataBuffer = this.dataBuffer.slice(1);
        }
        console.log(`[RFB] SecurityResult: buffer=${this.dataBuffer.length} bytes, need 4`);
        if (this.dataBuffer.length < 4) { console.log(`[RFB] SecurityResult: waiting for more data...`); return; }
        const result = new DataView(this.dataBuffer.buffer, this.dataBuffer.byteOffset, 4).getUint32(0, false);
        console.log(`[RFB] SecurityResult value: ${result} (0x${result.toString(16)}) — ${result === 0 ? 'OK' : 'FAILED'}`);
        if (result !== 0) { console.error('[RFB] Security failed, code:', result); return; }
        console.log('[RFB] Security OK - sending ClientInit (shared=1)');
        this.sendToServer(new Uint8Array([1])); // ClientInit: shared=1
        this.dataBuffer = this.dataBuffer.slice(4);
        this.rfbState = 'ServerInit';
        this.cdr.detectChanges();
        continue;
      }
      
      if (this.rfbState === 'ServerInit') {
        // ServerInit: width(2) + height(2) + pixelformat(16) + namelength(4) + name
        if (this.dataBuffer.length < 24) return;
        
        const view = new DataView(this.dataBuffer.buffer, this.dataBuffer.byteOffset);
        this.canvasWidth  = view.getUint16(0, false);
        this.canvasHeight = view.getUint16(2, false);

        // Parse pixel format (16 bytes at offset 4)
        this.pixelFormat = {
          bitsPerPixel: this.dataBuffer[4],
          bigEndian:    this.dataBuffer[6] !== 0,
          trueColour:   this.dataBuffer[7] !== 0,
          redMax:   view.getUint16(8,  false),
          greenMax: view.getUint16(10, false),
          blueMax:  view.getUint16(12, false),
          redShift:   this.dataBuffer[14],
          greenShift: this.dataBuffer[15],
          blueShift:  this.dataBuffer[16],
        };

        const nameLength = view.getUint32(20, false);
        if (this.dataBuffer.length < 24 + nameLength) return;
        
        const serverName = new TextDecoder().decode(this.dataBuffer.slice(24, 24 + nameLength));
        const isRGB565 = this.pixelFormat.bitsPerPixel === 16 && this.pixelFormat.redMax === 31 && this.pixelFormat.greenMax === 63 && this.pixelFormat.blueMax === 31;
        console.log(`[RFB] AMT Server: "${serverName}" ${this.canvasWidth}x${this.canvasHeight} ${this.pixelFormat.bitsPerPixel}bpp ${isRGB565 ? '(RGB565)' : '(generic)'} bigEndian=${this.pixelFormat.bigEndian}`);
        console.log(`[RFB] PixelFormat rMax=${this.pixelFormat.redMax}@${this.pixelFormat.redShift} gMax=${this.pixelFormat.greenMax}@${this.pixelFormat.greenShift} bMax=${this.pixelFormat.blueMax}@${this.pixelFormat.blueShift}`);
        
        // Set canvas size
        this.canvas.width  = this.canvasWidth;
        this.canvas.height = this.canvasHeight;
        
        // Request framebuffer update (SetEncodings + FramebufferUpdateRequest)
        this.sendSetEncodings();
        this.requestFramebufferUpdate(false);
        
        this.dataBuffer = this.dataBuffer.slice(24 + nameLength);
        this.rfbState = 'Normal';
        this.cdr.detectChanges();
        console.log('[RFB] Handshake complete! Receiving framebuffer...');
        continue;
      }
      
      if (this.rfbState === 'Normal') {
        if (this.dataBuffer.length < 1) return;
        const msgType = this.dataBuffer[0];
        
        if (msgType === 0) {
          // FramebufferUpdate
          if (!this.handleFramebufferUpdate()) return; // wait for more data
          continue;
        } else if (msgType === 2) {
          // Bell - 1 byte total
          this.dataBuffer = this.dataBuffer.slice(1);
          continue;
        } else if (msgType === 3) {
          // ServerCutText: type(1) + padding(3) + length(4) + text
          if (this.dataBuffer.length < 8) return;
          const textLen = new DataView(this.dataBuffer.buffer, this.dataBuffer.byteOffset).getUint32(4, false);
          if (this.dataBuffer.length < 8 + textLen) return;
          this.dataBuffer = this.dataBuffer.slice(8 + textLen);
          continue;
        } else {
          console.warn('[RFB] Unknown server message type:', msgType, '— resyncing buffer');
          this.dataBuffer = new Uint8Array(0);
          return;
        }
      }
      
      break;
    }
  }

  // Returns true when the full update was consumed; false when more data is needed.
  private handleFramebufferUpdate(): boolean {
    // type(1) + padding(1) + numRects(2) = 4 bytes header
    if (this.dataBuffer.length < 4) return false;
    const view = new DataView(this.dataBuffer.buffer, this.dataBuffer.byteOffset);
    const numRects = view.getUint16(2, false);
    let offset = 4;

    for (let i = 0; i < numRects; i++) {
      // Each rect header: x(2) y(2) w(2) h(2) encoding(4) = 12 bytes
      if (this.dataBuffer.length < offset + 12) return false;
      
      const x        = view.getUint16(offset,     false);
      const y        = view.getUint16(offset + 2, false);
      const w        = view.getUint16(offset + 4, false);
      const h        = view.getUint16(offset + 6, false);
      const encoding = view.getInt32 (offset + 8, false);
      offset += 12;

      if (w === 0 || h === 0) continue; // empty rect — skip

      const bpp = this.pixelFormat.bitsPerPixel >>> 3;

      if (encoding === -223) {
        // Desktop Size pseudo-encoding: AMT is telling us the screen was resized.
        // w/h in the rect header are the new dimensions — no pixel payload.
        console.log(`[RFB] Desktop Size update: ${w}x${h}`);
        this.canvasWidth  = w;
        this.canvasHeight = h;
        this.canvas.width  = w;
        this.canvas.height = h;
        // Don't increment offset — no payload

      } else if (encoding === 0) {
        // Raw encoding — RGB565 (16bpp, little-endian) is AMT's default
        const pixelLen = w * h * bpp;
        if (this.dataBuffer.length < offset + pixelLen) return false;
        this.renderRaw(x, y, w, h, this.dataBuffer.slice(offset, offset + pixelLen));
        offset += pixelLen;

      } else if (encoding === 16) {
        // ZRLE: 4-byte length prefix + zlib-compressed data
        if (this.dataBuffer.length < offset + 4) return false;
        const zrleLen = view.getUint32(offset, false);
        if (this.dataBuffer.length < offset + 4 + zrleLen) return false;
        // TODO: implement ZRLE inflate+decode for better performance
        // For now skip the ZRLE block — AMT should fall back to Raw after we re-request
        console.warn(`[RFB] ZRLE rect ${w}x${h} (${zrleLen}B) — skipping (not yet implemented)`);
        offset += 4 + zrleLen;

      } else if (encoding === 1092) {
        // KVM Data Channel — carried in rect payload as raw bytes
        // (bidirectional channel for AMT metadata/ack, typically 16-17 bytes)
        if (this.dataBuffer.length < offset + w) return false; // w = payload length for this encoding
        offset += w; // consume payload

      } else {
        console.warn(`[RFB] Unsupported encoding ${encoding} at (${x},${y}) ${w}x${h} — resyncing`);
        this.dataBuffer = new Uint8Array(0);
        return true;
      }
    }

    this.dataBuffer = this.dataBuffer.slice(offset);

    this.frameCount++;
    const now = Date.now();
    if (this.lastFrameTime > 0) this.fps = Math.round(1000 / (now - this.lastFrameTime));
    this.lastFrameTime = now;

    // Request next incremental update
    this.requestFramebufferUpdate(true);
    return true;
  }

  private pixelToRGB(buf: Uint8Array, offset: number): [number, number, number] {
    const pf = this.pixelFormat;
    const bpp = pf.bitsPerPixel >>> 3;
    let v = 0;
    if      (bpp === 4) v = new DataView(buf.buffer, buf.byteOffset + offset, 4).getUint32(0, !pf.bigEndian);
    else if (bpp === 2) v = new DataView(buf.buffer, buf.byteOffset + offset, 2).getUint16(0, !pf.bigEndian);
    else                v = buf[offset];
    const r = Math.round(((v >>> pf.redShift)   & pf.redMax)   * 255 / pf.redMax);
    const g = Math.round(((v >>> pf.greenShift) & pf.greenMax) * 255 / pf.greenMax);
    const b = Math.round(((v >>> pf.blueShift)  & pf.blueMax)  * 255 / pf.blueMax);
    return [r, g, b];
  }

  private renderRaw(x: number, y: number, w: number, h: number, data: Uint8Array): void {
    const pf  = this.pixelFormat;
    const bpp = pf.bitsPerPixel >>> 3;
    const n   = w * h;
    const imageData = this.ctx.createImageData(w, h);
    const out = imageData.data;

    if (bpp === 2) {
      // RGB565 fast path (AMT default) — matches DMT ImageHelper.setPixel() logic
      const leFlag = !pf.bigEndian; // true = little-endian
      const dv = new DataView(data.buffer, data.byteOffset);
      for (let i = 0; i < n; i++) {
        const v = dv.getUint16(i << 1, leFlag);
        const outOff = i << 2;
        out[outOff]     = (v >> 8) & 0xF8;  // R: bits 15-11
        out[outOff + 1] = (v >> 3) & 0xFC;  // G: bits 10-5
        out[outOff + 2] = (v << 3) & 0xF8;  // B: bits 4-0
        out[outOff + 3] = 255;
      }
    } else if (bpp === 4) {
      // 32bpp path
      const leFlag = !pf.bigEndian;
      const dv = new DataView(data.buffer, data.byteOffset);
      const rDiv = 255 / pf.redMax || 1;
      const gDiv = 255 / pf.greenMax || 1;
      const bDiv = 255 / pf.blueMax || 1;
      for (let i = 0; i < n; i++) {
        const v = dv.getUint32(i << 2, leFlag);
        const outOff = i << 2;
        out[outOff]     = Math.round(((v >>> pf.redShift)   & pf.redMax)   * rDiv);
        out[outOff + 1] = Math.round(((v >>> pf.greenShift) & pf.greenMax) * gDiv);
        out[outOff + 2] = Math.round(((v >>> pf.blueShift)  & pf.blueMax)  * bDiv);
        out[outOff + 3] = 255;
      }
    } else {
      // 8bpp RGB332
      for (let i = 0; i < n; i++) {
        const v = data[i];
        const outOff = i << 2;
        out[outOff]     = v & 0xE0;
        out[outOff + 1] = (v & 0x1C) << 3;
        out[outOff + 2] = (v & 0x03) << 6;
        out[outOff + 3] = 255;
      }
    }
    this.ctx.putImageData(imageData, x, y);
  }

  // Called from the wrapper div click and canvas mouseenter to ensure keyboard focus
  focusCanvas(): void {
    this.canvas?.focus();
  }

  // ---- Input events -------------------------------------------------------

  private onMouseEnter(e: MouseEvent): void {
    this.canvas.focus();
  }

  private onMouseMove(e: MouseEvent): void {
    if (this.rfbState !== 'Normal') return;
    const [x, y] = this.canvasCoords(e);
    this.sendPointerEvent(x, y, this.buttonMask);
  }

  private onMouseButton(e: MouseEvent): void {
    if (this.rfbState !== 'Normal') return;
    e.preventDefault();
    // For document-level mouseup, compute coords clamped to canvas
    const [x, y] = this.canvasCoordsFromPage(e);
    const btnBit = [1, 2, 4][e.button] ?? 1; // left=1, middle=2, right=4
    this.buttonMask = e.type === 'mousedown' ? this.buttonMask | btnBit : this.buttonMask & ~btnBit;
    this.sendPointerEvent(x, y, this.buttonMask);
  }

  private canvasCoords(e: MouseEvent): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    return [
      Math.round((e.clientX - rect.left) * this.canvasWidth  / rect.width),
      Math.round((e.clientY - rect.top)  * this.canvasHeight / rect.height),
    ];
  }

  // Like canvasCoords but clamps to canvas bounds (used for document-level mouseup)
  private canvasCoordsFromPage(e: MouseEvent): [number, number] {
    const rect = this.canvas.getBoundingClientRect();
    const cx = Math.max(0, Math.min(rect.width,  e.clientX - rect.left));
    const cy = Math.max(0, Math.min(rect.height, e.clientY - rect.top));
    return [
      Math.round(cx * this.canvasWidth  / rect.width),
      Math.round(cy * this.canvasHeight / rect.height),
    ];
  }

  private sendPointerEvent(x: number, y: number, buttonMask: number): void {
    const msg = new Uint8Array(6);
    const v = new DataView(msg.buffer);
    v.setUint8 (0, 5);
    v.setUint8 (1, buttonMask);
    v.setUint16(2, x, false);
    v.setUint16(4, y, false);
    this.sendToServer(msg);
  }

  private onKeyEvent(e: KeyboardEvent): void {
    if (this.rfbState !== 'Normal') return;
    e.preventDefault();
    const keysym = this.jsKeyToKeysym(e.key);
    if (keysym) this.sendKeyEvent(keysym, e.type === 'keydown');
  }

  private sendKeyEvent(keysym: number, down: boolean): void {
    const msg = new Uint8Array(8);
    const v = new DataView(msg.buffer);
    v.setUint8 (0, 4);           // KeyEvent
    v.setUint8 (1, down ? 1 : 0);
    v.setUint16(2, 0, false);    // padding
    v.setUint32(4, keysym, false); // big-endian keysym
    this.sendToServer(msg);
  }

  private jsKeyToKeysym(key: string): number {
    const map: Record<string, number> = {
      'Enter':     0xff0d, 'Escape':    0xff1b, 'Backspace': 0xff08,
      'Tab':       0xff09, 'Delete':    0xffff, 'Insert':    0xff63,
      'Home':      0xff50, 'End':       0xff57,
      'ArrowLeft': 0xff51, 'ArrowUp':   0xff52,
      'ArrowRight':0xff53, 'ArrowDown': 0xff54,
      'PageUp':    0xff55, 'PageDown':  0xff56,
      'F1': 0xffbe, 'F2': 0xffbf, 'F3': 0xffc0, 'F4': 0xffc1,
      'F5': 0xffc2, 'F6': 0xffc3, 'F7': 0xffc4, 'F8': 0xffc5,
      'F9': 0xffc6,'F10': 0xffc7,'F11': 0xffc8,'F12': 0xffc9,
      'Shift':    0xffe1, 'Control':  0xffe3, 'Alt': 0xffe9,
      'Meta':     0xffeb, 'CapsLock': 0xffe5,
    };
    if (map[key]) return map[key];
    if (key.length === 1) return key.charCodeAt(0);
    return 0;
  }

  // ---- Helpers -------------------------------------------------------------

  private sendSetEncodings() {
    // Intel AMT encodings — advertise only Raw(0), KVM Data Channel(1092), Desktop Size(-223).
    // ZRLE(16) is intentionally NOT advertised so AMT falls back to Raw which we can decode.
    // ZRLE requires zlib inflate which is not implemented here.
    const msg = new Uint8Array([
      2, 0,                         // SetEncodings + padding
      0, 3,                         // 3 encodings
      0, 0, 0, 0,                   // Raw
      0, 0, 0x04, 0x44,             // KVM Data Channel (1092)
      0xFF, 0xFF, 0xFF, 0x21,       // Desktop Size (-223)
    ]);
    this.sendToServer(msg);
    console.log('[RFB] Sent SetEncodings: Raw(0) KvmDataChannel(1092) DesktopSize(-223)');
  }
  
  private requestFramebufferUpdate(incremental: boolean) {
    const msg = new Uint8Array(10);
    const v = new DataView(msg.buffer);
    v.setUint8 (0, 3);                            // FramebufferUpdateRequest
    v.setUint8 (1, incremental ? 1 : 0);
    v.setUint16(2, 0, false);                     // x
    v.setUint16(4, 0, false);                     // y
    v.setUint16(6, this.canvasWidth  || 1024, false);
    v.setUint16(8, this.canvasHeight || 768,  false);
    this.sendToServer(msg);
  }
  
  private sendToServer(data: Uint8Array) {
    if (this.sendDataCallback) {
      this.sendDataCallback(data.buffer as ArrayBuffer);
    } else {
      console.warn('[RFB] sendData callback not set!');
    }
  }
}
