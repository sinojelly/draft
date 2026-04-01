export default class SuccessModal {
  constructor(onClose, onConfigureTask) {
    this.onClose = onClose;
    this.onConfigureTask = onConfigureTask;
  }

  render() {
    return `
      <div class="modal-overlay active" id="successModalOverlay">
        <div class="modal-content">
          <div style="text-align: center; padding: 20px 0;">
            <div style="width: 64px; height: 64px; background: var(--success); border-radius: 50%; margin: 0 auto 20px; display: flex; align-items: center; justify-content: center;">
              <svg width="32" height="32" viewBox="0 0 24 24" fill="none" stroke="white" stroke-width="3">
                <polyline points="20 6 9 17 4 12"/>
              </svg>
            </div>
            <h2 style="margin-bottom: 12px;">🎉 百度网盘授权成功!</h2>
            <p style="color: var(--text-secondary); margin-bottom: 32px;">要立刻为这个网盘配置一个同步任务吗?</p>
          </div>
          <div class="modal-actions">
            <button type="button" class="btn btn-secondary" id="laterBtn">稍后再说</button>
            <button type="button" class="btn btn-primary" id="configureBtn">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M13 2L3 14h9l-1 8 10-12h-9l1-8z"/>
              </svg>
              立即配置任务
            </button>
          </div>
        </div>
      </div>
    `;
  }

  attachEventListeners() {
    // Close on overlay click
    document.getElementById('successModalOverlay')?.addEventListener('click', (e) => {
      if (e.target.id === 'successModalOverlay') {
        this.onClose();
      }
    });

    // Later button
    document.getElementById('laterBtn')?.addEventListener('click', () => {
      this.onClose();
    });

    // Configure button
    document.getElementById('configureBtn')?.addEventListener('click', () => {
      this.onConfigureTask();
    });
  }
}
