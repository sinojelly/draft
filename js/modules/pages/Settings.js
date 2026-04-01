export default class Settings {
  constructor(app) {
    this.app = app;
    this.settings = {
      webPort: 8080,
      autoOpenBrowser: true,
      logPath: './logs',
      logLevel: 'INFO',
      maxFileSize: 100,
      maxFileCount: 5,
      retentionDays: 30,
      maxConcurrency: 5,
      maxBatchSize: 100,
      debounceDelay: 500
    };
  }

  render() {
    return `
      <div class="page-header">
        <h1 class="page-title">系统设置</h1>
      </div>

      <div class="settings-section">
        <h3>Web 服务</h3>
        <div class="setting-item">
          <div>
            <div class="setting-label">Web 端口</div>
            <div class="setting-description">Web UI 监听端口</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.webPort}" min="1" max="65535" data-setting="webPort">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">自动打开浏览器</div>
            <div class="setting-description">启动时自动打开 Web UI</div>
          </div>
          <div class="setting-control">
            <div class="toggle-switch ${this.settings.autoOpenBrowser ? 'active' : ''}" data-toggle="autoOpenBrowser"></div>
          </div>
        </div>
      </div>

      <div class="settings-section">
        <h3>日志配置</h3>
        <div class="setting-item">
          <div>
            <div class="setting-label">日志路径</div>
            <div class="setting-description">日志文件保存位置</div>
          </div>
          <div class="setting-control">
            <input type="text" value="${this.settings.logPath}" placeholder="日志文件路径" data-setting="logPath">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">日志级别</div>
            <div class="setting-description">记录的日志详细程度</div>
          </div>
          <div class="setting-control">
            <select data-setting="logLevel">
              <option ${this.settings.logLevel === 'ERROR' ? 'selected' : ''}>ERROR</option>
              <option ${this.settings.logLevel === 'WARNING' ? 'selected' : ''}>WARNING</option>
              <option ${this.settings.logLevel === 'INFO' ? 'selected' : ''}>INFO</option>
              <option ${this.settings.logLevel === 'DEBUG' ? 'selected' : ''}>DEBUG</option>
            </select>
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">单个文件大小 (MB)</div>
            <div class="setting-description">单个日志文件最大大小</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.maxFileSize}" min="1" data-setting="maxFileSize">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">最大文件数量</div>
            <div class="setting-description">保留的日志文件数量</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.maxFileCount}" min="1" data-setting="maxFileCount">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">保留天数</div>
            <div class="setting-description">日志文件保留时长</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.retentionDays}" min="1" data-setting="retentionDays">
          </div>
        </div>
      </div>

      <div class="settings-section">
        <h3>性能配置</h3>
        <div class="setting-item">
          <div>
            <div class="setting-label">最大并发数</div>
            <div class="setting-description">同时进行的传输任务数</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.maxConcurrency}" min="1" max="20" data-setting="maxConcurrency">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">最大批量数</div>
            <div class="setting-description">单次批量处理的文件数</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.maxBatchSize}" min="1" data-setting="maxBatchSize">
          </div>
        </div>
        <div class="setting-item">
          <div>
            <div class="setting-label">事件监听防抖延迟 (毫秒)</div>
            <div class="setting-description">文件变化事件的等待时间</div>
          </div>
          <div class="setting-control">
            <input type="number" value="${this.settings.debounceDelay}" min="0" data-setting="debounceDelay">
          </div>
        </div>
      </div>

      <div style="margin-top: 32px; display: flex; gap: 12px; justify-content: flex-end;">
        <button class="btn btn-secondary" id="resetBtn">重置为默认</button>
        <button class="btn btn-primary" id="saveBtn">保存设置</button>
      </div>
    `;
  }

  attachEventListeners() {
    // Input changes
    document.querySelectorAll('[data-setting]').forEach(input => {
      input.addEventListener('change', (e) => {
        const setting = e.target.dataset.setting;
        this.settings[setting] = e.target.type === 'number' ? Number(e.target.value) : e.target.value;
      });
    });

    // Toggle switches
    document.querySelectorAll('[data-toggle]').forEach(toggle => {
      toggle.addEventListener('click', () => {
        const setting = toggle.dataset.toggle;
        this.settings[setting] = !this.settings[setting];
        toggle.classList.toggle('active');
      });
    });

    // Save button
    document.getElementById('saveBtn')?.addEventListener('click', () => {
      window.showToast('设置已保存', 'success');
    });

    // Reset button
    document.getElementById('resetBtn')?.addEventListener('click', () => {
      const confirmed = confirm('确定要重置为默认设置吗？');
      if (confirmed) {
        this.settings = {
          webPort: 8080,
          autoOpenBrowser: true,
          logPath: './logs',
          logLevel: 'INFO',
          maxFileSize: 100,
          maxFileCount: 5,
          retentionDays: 30,
          maxConcurrency: 5,
          maxBatchSize: 100,
          debounceDelay: 500
        };
        this.updateView();
        window.showToast('已重置为默认设置', 'success');
      }
    });
  }

  updateView() {
    const mainContent = document.getElementById('main-content');
    if (mainContent) {
      mainContent.innerHTML = this.render();
      this.attachEventListeners();
    }
  }
}
