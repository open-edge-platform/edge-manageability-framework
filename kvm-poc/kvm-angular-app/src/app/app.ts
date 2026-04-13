import { Component, signal, computed, OnInit, ViewChild } from '@angular/core'
import { FormsModule } from '@angular/forms'
import { CommonModule, AsyncPipe } from '@angular/common'
import { KvmService } from './services/kvm.service'
import { KvmViewerComponent } from './components/kvm-viewer.component'

@Component({
  selector: 'app-root',
  imports: [FormsModule, CommonModule, AsyncPipe, KvmViewerComponent],
  templateUrl: './app.html',
  styleUrl: './app.css',
})
export class App implements OnInit {
  @ViewChild(KvmViewerComponent)
  set kvmViewer(component: KvmViewerComponent | undefined) {
    if (!component) {
      return
    }

    component.setSendCallback((data: ArrayBuffer) => {
      this.kvmService.sendData(data)
    })
  }
  // Config
  mpsHost = 'mps-wss.orch-10-139-218-43.pid.infra-host.com'
  deviceGuid = '94e00576-d750-3391-de61-48210b50d802'
  port = 16994
  jwtToken = ''

  // State
  deviceState = signal(0)
  logs = signal<string[]>([])
  showViewer = signal(false)
  consentCode = ''
  consentMessage = ''
  consentRequested = false
  requestingConsent = false
  submittingConsent = false
  consentApproved = false

  // Computed
  readonly isActive = computed(() => this.deviceState() === 3)
  readonly isConnecting = computed(() => this.deviceState() === 1 || this.deviceState() === 2)
  readonly stateLabel = computed(() => {
    const labels = {
      0: 'Disconnected',
      1: 'Connecting...',
      2: 'Connected',
      3: 'Active'
    }
    return labels[this.deviceState() as keyof typeof labels]
  })

  constructor(public kvmService: KvmService) {}

  ngOnInit() {
    // Subscribe to service state
    this.kvmService.deviceState$.subscribe(state => {
      this.deviceState.set(state)
    })

    this.kvmService.logMessage$.subscribe(msg => {
      this.logs.update(l => [...l, msg].slice(-50))
    })
  }

  async requestConsentCode() {
    if (!this.deviceGuid || !this.jwtToken) {
      this.consentMessage = 'Device GUID and JWT token are required'
      return
    }

    this.requestingConsent = true
    this.consentRequested = false
    this.consentApproved = false
    this.consentCode = ''
    this.consentMessage = 'Requesting consent code from the device...'

    try {
      const resp = await fetch(`/api/consent/${this.deviceGuid}`, {
        method: 'GET',
        headers: {
          Authorization: `Bearer ${this.jwtToken}`,
        },
      })

      if (!resp.ok) {
        throw new Error(await resp.text())
      }

      this.consentRequested = true
      this.consentMessage = 'Consent code displayed on the device screen. Enter the 6-digit code below.'
      this.logs.update(l => [...l, '[*] Consent code requested successfully'].slice(-50))
    } catch (error: any) {
      this.consentMessage = `Failed to request consent code: ${error?.message || error}`
      this.logs.update(l => [...l, `[ERROR] ${this.consentMessage}`].slice(-50))
    } finally {
      this.requestingConsent = false
    }
  }

  async submitConsentCode() {
    if (!this.consentCode || this.consentCode.length !== 6) {
      this.consentMessage = 'Enter the 6-digit consent code'
      return
    }

    this.submittingConsent = true
    this.consentMessage = 'Submitting consent code...'

    try {
      const resp = await fetch(`/api/consent/${this.deviceGuid}`, {
        method: 'POST',
        headers: {
          Authorization: `Bearer ${this.jwtToken}`,
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ consentCode: this.consentCode }),
      })

      if (!resp.ok) {
        throw new Error(await resp.text())
      }

      this.consentApproved = true
      this.consentRequested = false
      this.consentMessage = 'Consent approved. You can now connect to KVM.'
      this.logs.update(l => [...l, '[OK] Consent approved'].slice(-50))
    } catch (error: any) {
      this.consentMessage = `Consent submit failed: ${error?.message || error}`
      this.logs.update(l => [...l, `[ERROR] ${this.consentMessage}`].slice(-50))
    } finally {
      this.submittingConsent = false
    }
  }

  resetConsentFlow() {
    this.consentCode = ''
    this.consentMessage = ''
    this.consentRequested = false
    this.requestingConsent = false
    this.submittingConsent = false
    this.consentApproved = false
  }
  
  async connect() {
    if (!this.consentApproved) {
      this.consentMessage = 'Complete user consent before connecting to KVM'
      return
    }

    try {
      this.showViewer.set(true)
      await this.kvmService.connect({
        mpsHost: this.mpsHost,
        deviceGuid: this.deviceGuid,
        port: this.port,
        mode: 'kvm',
        jwtToken: this.jwtToken
      })
    } catch (error: any) {
      console.error('Connection error:', error)
      this.showViewer.set(false)
      if (this.deviceState() === 0) {
        this.logs.update(l => [...l, '[WARN] Connection attempt ended before viewer became active'].slice(-50))
      }
    }
  }

  async disconnect() {
    await this.kvmService.disconnect()
    this.showViewer.set(false)
  }
}
