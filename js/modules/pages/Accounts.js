export default class Accounts {
  constructor() {
    this.accounts = [
      {
        id: 1,
        name: '百度网盘',
        alias: '我的备份',
        type: 'baidu',
        status: 'connected',
        storage: { used: 256, total: 2048, unit: 'GB' },
        gradient: 'linear-gradient(135deg, var(--primary), #00BCF2)'
      },
      {
        id: 2,
        name: '一刻相册',
        alias: '家庭相册',
        type: 'yike',
        status: 'connected',
        storage: { used: 128, total: 512, unit: 'GB' },
        gradient: 'linear-gradient(135deg, #FF6B6B, #FF8E53)'
      },
      {
        id: 3,
        name: '百度网盘',
        alias: '工作文档',
        type: 'baidu',
        status: 'connected',
        storage: { used: 89, total: 2048, unit: 'GB' },
        gradient: 'linear-gradient(135deg, #4ECDC4, #44A08D)'
      }
    ];
  }

  render() {
    return `
      <div class="page-header">
        <h1 class="page-title">网盘账号</h1>
        <button class="btn btn-primary" id="addAccountBtn">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          添加账号
        </button>
      </div>

      <div class="account-list">
        ${this.accounts.map(account => this.renderAccount(account)).join('')}
        ${this.renderAddCard()}
      </div>

      <div id="toastContainer"></div>
    `;
  }

  renderAccount(account) {
    return `
      <div class="account-card">
        <div class="account-header">
          <div class="account-icon" style="background: ${account.gradient}">
            <svg viewBox="0 0 24 24" fill="currentColor">
              <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/>
            </svg>
          </div>
          <div class="account-info">
            <h3>${account.name}</h3>
            <p>${account.alias}</p>
          </div>
        </div>
        <div class="account-status ${account.status}">
          ${account.status === 'connected' ? '已绑定' : '未绑定'}
        </div>
        ${account.storage ? `
          <div class="account-storage">
            已用 ${account.storage.used} ${account.storage.unit} / 共 ${account.storage.total} ${account.storage.unit}
          </div>
        ` : ''}
        <button class="btn btn-secondary" style="width: 100%;" data-action="unbind" data-id="${account.id}">
          解绑账号
        </button>
      </div>
    `;
  }

  renderAddCard() {
    return `
      <div class="account-card" style="border: 2px dashed var(--border-color); box-shadow: none;">
        <div style="text-align: center; padding: 40px 20px; color: var(--text-secondary); cursor: pointer;" id="addCardBtn">
          <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" style="margin: 0 auto 16px;">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          <p>添加新账号</p>
        </div>
      </div>
    `;
  }

  attachEventListeners() {
    // Add account button
    document.getElementById('addAccountBtn')?.addEventListener('click', () => {
      alert('添加账号功能');
    });

    document.getElementById('addCardBtn')?.addEventListener('click', () => {
      alert('添加账号功能');
    });

    // Unbind buttons
    document.querySelectorAll('[data-action="unbind"]').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = parseInt(btn.dataset.id);
        if (confirm('确定要解绑这个账号吗？')) {
          this.accounts = this.accounts.filter(a => a.id !== id);
          this.updateView();
          this.showToast('账号已解绑');
        }
      });
    });
  }

  updateView() {
    const accountList = document.querySelector('.account-list');
    if (accountList) {
      accountList.innerHTML = this.accounts.map(account => this.renderAccount(account)).join('') + this.renderAddCard();
      this.attachEventListeners();
    }
  }

  showToast(message) {
    const container = document.getElementById('toastContainer');
    if (container) {
      container.innerHTML = `
        <div class="toast show">
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="20 6 9 17 4 12"/>
          </svg>
          <span>${message}</span>
        </div>
      `;
      setTimeout(() => container.innerHTML = '', 3000);
    }
  }
}
