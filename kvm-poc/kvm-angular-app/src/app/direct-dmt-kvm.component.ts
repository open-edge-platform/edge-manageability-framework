import { Component, signal } from '@angular/core';
import { KVMComponent } from '@device-management-toolkit/ui-toolkit-angular';
import { FormsModule } from '@angular/forms';
import { CommonModule } from '@angular/common';

/**
 * Simple KVM Viewer using DMT UI Toolkit Component Directly
 * 
 * This connects browser → MPS directly (no Go server needed)
 * DMT component handles ALL protocol work internally:
 * - RFB protocol parsing
 * - Canvas rendering
 * - Mouse/keyboard input
 * - User consent (CCM mode)
 */
@Component({
  selector: 'app-root',
  standalone: true,
  imports: [KVMComponent, FormsModule, CommonModule],
  template: `
    <div class="app">
      <header>
        <h1>🖥️ DMT Direct KVM Viewer</h1>
        <p>Browser → MPS → AMT (No backend server!)</p>
      </header>

      <!-- Connection Form -->
      <div class="connection-form" *ngIf="!connected()">
        <div class="form-group">
          <label>MPS Server:</label>
          <input [(ngModel)]="mpsServer" placeholder="mps-wss.example.com" />
        </div>

        <div class="form-group">
          <label>Device GUID:</label>
          <input [(ngModel)]="deviceId" placeholder="00000000-0000-0000-0000-000000000000" />
        </div>

        <div class="form-group">
          <label>JWT Token:</label>
          <input type="password" [(ngModel)]="authToken" placeholder="Paste JWT token" />
        </div>

        <button (click)="connect()" [disabled]="!authToken">
          Connect to Device
        </button>
      </div>

      <!-- Consent Dialog (appears if device in CCM mode) -->
      <div class="consent-dialog" *ngIf="consentRequired()">
        <div class="consent-card">
          <h3>⚠️ User Consent Required</h3>
          <p>Look at the device screen for a 6-digit code:</p>
          <div class="consent-screen-mockup">
            <pre>
╔════════════════════════╗
║  Remote KVM Access    ║
║  Consent Code:        ║
║  <span class="code">1 2 3 4 5 6</span>        ║
║  (Example)            ║
╚════════════════════════╝
            </pre>
          </div>
          <input 
            [(ngModel)]="consentCode" 
            placeholder="Enter 6-digit code"
            maxlength="6"
            class="consent-input"
          />
          <button (click)="submitConsent()">Submit Code</button>
          <button (click)="cancelConsent()" class="secondary">Cancel</button>
        </div>
      </div>

      <!-- DMT KVM Component (handles everything internally) -->
      <div class="kvm-viewer" *ngIf="connected()">
        <div class="kvm-controls">
          <button (click)="disconnect()" class="disconnect-btn">
            Disconnect
          </button>
          <span class="status">
            Status: {{ kvmStatus() }}
          </span>
        </div>

        <!-- This component does ALL the work! -->
        <amt-kvm
          [deviceId]="deviceId"
          [mpsServer]="mpsServer"
          [authToken]="authToken"
          [deviceConnection]="true"
          [selectedEncoding]="1"
          (consentRequired)="handleConsentRequest($event)"
          (statusChange)="handleStatusChange($event)"
          (dataReceived)="handleDataReceived($event)">
        </amt-kvm>
      </div>
    </div>
  `,
  styles: [`
    .app {
      max-width: 1400px;
      margin: 0 auto;
      padding: 20px;
      font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
    }

    header {
      text-align: center;
      background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
      color: white;
      padding: 30px;
      border-radius: 8px;
      margin-bottom: 30px;
    }

    header h1 {
      margin: 0 0 10px 0;
    }

    header p {
      margin: 0;
      opacity: 0.9;
    }

    .connection-form {
      background: #f8f9fa;
      padding: 30px;
      border-radius: 8px;
      max-width: 600px;
      margin: 0 auto;
    }

    .form-group {
      margin-bottom: 20px;
    }

    .form-group label {
      display: block;
      margin-bottom: 8px;
      font-weight: 600;
      color: #333;
    }

    .form-group input {
      width: 100%;
      padding: 12px;
      border: 1px solid #ddd;
      border-radius: 4px;
      font-size: 14px;
      box-sizing: border-box;
    }

    button {
      padding: 12px 30px;
      background: #667eea;
      color: white;
      border: none;
      border-radius: 4px;
      font-size: 16px;
      cursor: pointer;
      font-weight: 600;
    }

    button:hover:not(:disabled) {
      background: #5568d3;
      transform: translateY(-2px);
      box-shadow: 0 4px 8px rgba(102, 126, 234, 0.4);
    }

    button:disabled {
      opacity: 0.5;
      cursor: not-allowed;
    }

    .consent-dialog {
      position: fixed;
      top: 0;
      left: 0;
      right: 0;
      bottom: 0;
      background: rgba(0, 0, 0, 0.7);
      display: flex;
      align-items: center;
      justify-content: center;
      z-index: 1000;
    }

    .consent-card {
      background: white;
      padding: 40px;
      border-radius: 8px;
      max-width: 500px;
      text-align: center;
    }

    .consent-card h3 {
      color: #ff9800;
      margin: 0 0 20px 0;
    }

    .consent-screen-mockup {
      background: #1a1a1a;
      color: #0f0;
      padding: 20px;
      border-radius: 4px;
      margin: 20px 0;
      font-family: 'Courier New', monospace;
    }

    .consent-screen-mockup .code {
      color: #ff0;
      font-size: 24px;
      font-weight: bold;
    }

    .consent-input {
      width: 200px;
      padding: 15px;
      font-size: 24px;
      text-align: center;
      letter-spacing: 10px;
      margin: 20px 0;
      border: 2px solid #667eea;
      border-radius: 4px;
    }

    .consent-card button {
      margin: 0 10px;
    }

    .consent-card button.secondary {
      background: #dc3545;
    }

    .kvm-viewer {
      margin-top: 20px;
    }

    .kvm-controls {
      display: flex;
      justify-content: space-between;
      align-items: center;
      padding: 15px;
      background: #f8f9fa;
      border-radius: 8px 8px 0 0;
    }

    .disconnect-btn {
      background: #dc3545;
    }

    .disconnect-btn:hover {
      background: #c82333;
    }

    .status {
      font-weight: 600;
      color: #28a745;
    }
  `]
})
export class DirectDmtKvmApp {
  // Configuration
  mpsServer = 'mps-wss.orch-10-139-218-43.pid.infra-host.com';
  deviceId = '94e00576-d750-3391-de61-48210b50d802';
  authToken = '';

  // State
  connected = signal(false);
  consentRequired = signal(false);
  consentCode = '';
  kvmStatus = signal('Disconnected');

  connect() {
    if (!this.authToken) {
      alert('Please enter JWT token');
      return;
    }
    
    console.log('[App] Connecting to device via DMT component...');
    console.log(`[App] MPS: ${this.mpsServer}`);
    console.log(`[App] Device: ${this.deviceId}`);
    
    this.connected.set(true);
    this.kvmStatus.set('Connecting...');
  }

  disconnect() {
    console.log('[App] Disconnecting...');
    this.connected.set(false);
    this.consentRequired.set(false);
    this.kvmStatus.set('Disconnected');
  }

  handleConsentRequest(event: any) {
    console.log('[App] Consent required!', event);
    this.consentRequired.set(true);
    this.kvmStatus.set('Waiting for consent...');
  }

  submitConsent() {
    if (this.consentCode.length !== 6) {
      alert('Please enter 6-digit code');
      return;
    }
    
    console.log('[App] Submitting consent code:', this.consentCode);
    
    // DMT component will handle sending this to AMT
    // For now, just close dialog (DMT will emit event when consent is processed)
    this.consentRequired.set(false);
    this.consentCode = '';
    this.kvmStatus.set('Consent submitted...');
  }

  cancelConsent() {
    console.log('[App] Consent cancelled');
    this.consentRequired.set(false);
    this.disconnect();
  }

  handleStatusChange(status: any) {
    console.log('[App] KVM status changed:', status);
    
    if (status.connected) {
      this.kvmStatus.set('Connected');
    } else if (status.active) {
      this.kvmStatus.set('Active - Receiving frames');
    } else if (status.error) {
      this.kvmStatus.set(`Error: ${status.error}`);
    }
  }

  handleDataReceived(event: any) {
    console.log('[App] Data received:', event.byteLength, 'bytes');
  }
}
