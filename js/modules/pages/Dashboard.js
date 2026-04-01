export default class Dashboard {
  constructor(app) {
    this.app = app;
    this.uploadSpeed = 5.2;
    this.downloadSpeed = 12.8;
    this.progress = 65;
    this.logs = [
      { time: '11:23:45', level: 'info', message: '开始扫描任务: D:\\Photos' },
      { time: '11:23:47', level: 'info', message: '发现 500 个文件需要上传' },
      { time: '11:23:48', level: 'info', message: '开始上传: photo_001.jpg (2.3 MB)' },
      { time: '11:23:50', level: 'debug', message: '上传进度: 50% (1.15 MB / 2.3 MB)' },
      { time: '11:23:52', level: 'info', message: '文件上传成功: photo_001.jpg' },
      { time: '11:23:55', level: 'error', message: '上传失败: document.pdf - 网盘空间不足' },
      { time: '11:24:01', level: 'info', message: '任务完成: D:\\Documents (320/321 成功)' }
    ];
    this.intervals = [];
  }

  render() {
    return `
      <div class="page-header">
        <h1 class="page-title">仪表盘</h1>
        <div style="display: flex; gap: 12px;">
          <button class="btn btn-secondary" id="onboardingBtn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <circle cx="12" cy="12" r="10"/>
              <path d="M9.09 9a3 3 0 0 1 5.83 1c0 2-3 3-3 3"/>
              <line x1="12" y1="17" x2="12.01" y2="17"/>
            </svg>
            初次进入
          </button>
          <button class="btn btn-primary" id="syncBtn">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M21.5 2v6h-6M2.5 22v-6h6M2 11.5a10 10 0 0 1 18.8-4.3M22 12.5a10 10 0 0 1-18.8 4.2"/>
            </svg>
            立即同步
          </button>
        </div>
      </div>

      <div class="cards-grid">
        <div class="card card-hover">
          <div class="card-title">实时上传速度</div>
          <div class="card-value" id="uploadSpeed">${this.uploadSpeed} <span style="font-size: 18px;">MB/s</span></div>
          <div class="card-label">实时监控中</div>
        </div>
        <div class="card card-hover">
          <div class="card-title">实时下载速度</div>
          <div class="card-value" id="downloadSpeed">${this.downloadSpeed} <span style="font-size: 18px;">MB/s</span></div>
          <div class="card-label">实时监控中</div>
        </div>
        <div class="card card-hover">
          <div class="card-title">队列概览</div>
          <div class="card-value">152</div>
          <div class="card-label">1.5 GB 待处理</div>
        </div>
        <div class="card card-hover">
          <div class="card-title">运行状态</div>
          <div class="card-value">2h 15m</div>
          <div class="card-label">8 个活跃线程</div>
        </div>
      </div>

      <div class="transfer-section">
        <h2 class="section-title">实时传输列表</h2>
        
        <div class="transfer-card">
          <div class="transfer-header">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
            <strong>[百度网盘][我的备份]</strong> D:\\Photos\\... → 云端: /备份/照片
          </div>
          <div class="transfer-status">
            <span class="status-text">⬆️ 正在上传 (Uploading)</span>
            <span class="progress-percent" id="progressPercent">${Math.floor(this.progress)}%</span>
          </div>
          <progress value="${this.progress}" max="100" id="progressBar"></progress>
          <div class="transfer-details">
            <div class="detail-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 16V8a2 2 0 0 0-1-1.73l-7-4a2 2 0 0 0-2 0l-7 4A2 2 0 0 0 3 8v8a2 2 0 0 0 1 1.73l7 4a2 2 0 0 0 2 0l7-4A2 2 0 0 0 21 16z"/>
              </svg>
              <span>数据: 1.5 GB / 2.3 GB (剩 800 MB)</span>
            </div>
            <div class="detail-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
              </svg>
              <span id="speedDisplay">速度: ${this.uploadSpeed} MB/s</span>
            </div>
            <div class="detail-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                <polyline points="14 2 14 8 20 8"/>
              </svg>
              <span>文件: 320 / 500 项 (剩 180 个)</span>
            </div>
            <div class="detail-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <polyline points="12 6 12 12 16 14"/>
              </svg>
              <span>预计: 约 2分30秒</span>
            </div>
          </div>
        </div>

        <div class="transfer-card">
          <div class="transfer-header">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
            <strong>[一刻相册][家庭相册]</strong> 云端: /家庭 → D:\\Downloads\\Family
          </div>
          <div class="transfer-status">
            <span class="status-text">🔍 扫描中 (Scanning)</span>
            <span class="progress-percent">—</span>
          </div>
          <progress></progress>
          <div class="transfer-details">
            <div class="detail-item">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="11" cy="11" r="8"/>
                <path d="m21 21-4.35-4.35"/>
              </svg>
              <span>正在计算文件数量与大小... (已发现 1,240 个文件)</span>
            </div>
          </div>
        </div>

        <div class="transfer-card completed">
          <div class="transfer-header">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
            </svg>
            <strong>[百度网盘][工作文档]</strong> D:\\Documents → 云端: /Documents
            <span style="color: var(--success); margin-left: 8px;">✅ 已是最新 - 11:24 AM</span>
          </div>
        </div>
      </div>

      <div class="transfer-section">
        <div class="terminal-header">
          <h2 class="terminal-title">终端日志</h2>
          <button class="btn btn-secondary" style="padding: 6px 12px; font-size: 12px;" id="clearLogsBtn">清空日志</button>
        </div>
        <div class="terminal">
          <div id="log-content">
            ${this.renderInitialLogs()}
          </div>
        </div>
      </div>
    `;
  }

  renderInitialLogs() {
    return this.logs.map(log => `
      <div class="log-entry log-${log.level}">
        <span class="log-time">[${log.time}]</span> <span class="log-level">[${log.level.toUpperCase()}]</span> <span class="log-msg">${log.message}</span>
      </div>
    `).join('');
  }

  // Optimized: Incremental rendering - no innerHTML full rewrite
  appendLog(log) {
    const logContainer = document.getElementById('log-content');
    if (!logContainer) return;

    // Create new log element
    const logDiv = document.createElement('div');
    logDiv.className = `log-entry log-${log.level}`;
    logDiv.innerHTML = `<span class="log-time">[${log.time}]</span> <span class="log-level">[${log.level.toUpperCase()}]</span> <span class="log-msg">${log.message}</span>`;

    // Insert at the bottom (or top if you prefer)
    logContainer.appendChild(logDiv);

    // Maintain max logs (remove oldest)
    const MAX_LOGS = 500;
    while (logContainer.children.length > MAX_LOGS) {
      logContainer.removeChild(logContainer.firstChild);
    }

    // Auto scroll to bottom
    const terminal = logContainer.parentElement;
    if (terminal) {
      terminal.scrollTop = terminal.scrollHeight;
    }
  }

  attachEventListeners() {
    // Onboarding button
    document.getElementById('onboardingBtn')?.addEventListener('click', () => {
      this.app.startOnboarding();
    });

    // Sync button
    document.getElementById('syncBtn')?.addEventListener('click', () => {
      window.showToast('手动同步已触发', 'success');
    });

    // Clear logs button
    document.getElementById('clearLogsBtn')?.addEventListener('click', () => {
      const logContainer = document.getElementById('log-content');
      if (logContainer) {
        logContainer.innerHTML = '';
        this.logs = [];
        window.showToast('日志已清空', 'success');
      }
    });

    // Start simulations
    this.startSimulations();
  }

  startSimulations() {
    // Clear any existing intervals
    this.intervals.forEach(id => clearInterval(id));
    this.intervals = [];

    // Update speeds with throttling
    let lastUpdate = Date.now();
    const speedInterval = setInterval(() => {
      const now = Date.now();
      if (now - lastUpdate < 500) return; // Throttle to max 2 updates per second
      lastUpdate = now;

      this.uploadSpeed = Number((Math.random() * 10 + 2).toFixed(1));
      this.downloadSpeed = Number((Math.random() * 15 + 5).toFixed(1));
      
      const uploadEl = document.getElementById('uploadSpeed');
      const downloadEl = document.getElementById('downloadSpeed');
      const speedEl = document.getElementById('speedDisplay');
      
      if (uploadEl) uploadEl.innerHTML = `${this.uploadSpeed} <span style="font-size: 18px;">MB/s</span>`;
      if (downloadEl) downloadEl.innerHTML = `${this.downloadSpeed} <span style="font-size: 18px;">MB/s</span>`;
      if (speedEl) speedEl.textContent = `速度: ${this.uploadSpeed} MB/s`;
    }, 2000);
    this.intervals.push(speedInterval);

    // Update progress
    const progressInterval = setInterval(() => {
      if (this.progress < 100) {
        this.progress = Math.min(100, this.progress + Math.random() * 2);
        const progressBar = document.getElementById('progressBar');
        const progressPercent = document.getElementById('progressPercent');
        if (progressBar) progressBar.value = this.progress;
        if (progressPercent) progressPercent.textContent = Math.floor(this.progress) + '%';
      }
    }, 1000);
    this.intervals.push(progressInterval);

    // Add new logs using optimized append method
    const logMessages = [
      '文件上传成功: report.docx',
      '检查文件变化...',
      '开始上传: presentation.pptx (15.2 MB)',
      '同步队列: 剩余 145 个文件'
    ];

    const logInterval = setInterval(() => {
      const time = new Date().toLocaleTimeString('zh-CN', { hour12: false });
      const message = logMessages[Math.floor(Math.random() * logMessages.length)];
      const log = { time, level: 'info', message };
      
      this.logs.push(log);
      if (this.logs.length > 500) this.logs.shift();
      
      this.appendLog(log);
    }, 5000);
    this.intervals.push(logInterval);
  }

  destroy() {
    // Clean up intervals when page is destroyed
    this.intervals.forEach(id => clearInterval(id));
    this.intervals = [];
  }
}
