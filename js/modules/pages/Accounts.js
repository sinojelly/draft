import AddAccountModal from '../components/AddAccountModal.js';
import SuccessModal from '../components/SuccessModal.js';
import AddTaskModal from '../components/AddTaskModal.js';

export default class Accounts {
  constructor(app) {
    this.app = app;
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
    this.currentModal = null;
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

      ${this.accounts.length === 0 ? this.renderEmptyState() : `
        <div class="account-list">
          ${this.accounts.map(account => this.renderAccount(account)).join('')}
          ${this.renderAddCard()}
        </div>
      `}

      <div id="modalContainer"></div>
    `;
  }

  renderEmptyState() {
    return `
      <div class="empty-state">
        <svg width="80" height="80" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
          <path d="M20.59 13.41l-7.17 7.17a2 2 0 0 1-2.83 0L2 12V2h10l8.59 8.59a2 2 0 0 1 0 2.82z"/>
        </svg>
        <h3>暂无网盘账号</h3>
        <p>您还没有绑定任何网盘账号，立即添加开始同步吧</p>
        <button class="btn btn-primary" onclick="document.getElementById('addAccountBtn').click()">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          添加账号
        </button>
      </div>
    `;
  }

  renderAccount(account) {
    return `
      <div class="account-card card-hover">
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
      <div class="account-card card-hover" style="border: 2px dashed var(--border-color); box-shadow: none; cursor: pointer;" id="addCardBtn">
        <div style="text-align: center; padding: 40px 20px; color: var(--text-secondary);">
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
      this.showAddAccountModal();
    });

    document.getElementById('addCardBtn')?.addEventListener('click', () => {
      this.showAddAccountModal();
    });

    // Unbind buttons
    document.querySelectorAll('[data-action="unbind"]').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = parseInt(btn.dataset.id);
        this.unbindAccount(id);
      });
    });
  }

  showAddAccountModal() {
    this.currentModal = new AddAccountModal(
      () => this.closeModal('addAccount'),
      (accountData) => this.handleAccountAdded(accountData)
    );
    
    const modalContainer = document.getElementById('modalContainer');
    if (modalContainer) {
      modalContainer.innerHTML = this.currentModal.render();
      this.currentModal.attachEventListeners();
    }
  }

  handleAccountAdded(accountData) {
    // Close add account modal
    this.closeModal('addAccount');
    
    // Show success modal
    setTimeout(() => {
      this.currentModal = new SuccessModal(
        () => this.closeModal('success'),
        () => this.handleConfigureTask()
      );
      
      const modalContainer = document.getElementById('modalContainer');
      if (modalContainer) {
        modalContainer.innerHTML = this.currentModal.render();
        this.currentModal.attachEventListeners();
      }
    }, 300);
  }

  handleConfigureTask() {
    // Close success modal
    this.closeModal('success');
    
    // Navigate to tasks page and show add task modal
    this.app.navigate('tasks');
    
    // Wait for page to render, then show modal
    setTimeout(() => {
      const tasksPage = this.app.pages.tasks;
      if (tasksPage && typeof tasksPage.showAddTaskModal === 'function') {
        tasksPage.showAddTaskModal();
      }
    }, 100);
  }

  unbindAccount(id) {
    const confirmed = confirm('确定要解绑这个账号吗？');
    if (confirmed) {
      this.accounts = this.accounts.filter(a => a.id !== id);
      this.updateView();
      window.showToast('账号已解绑', 'success');
    }
  }

  closeModal(modalName) {
    const modalContainer = document.getElementById('modalContainer');
    if (modalContainer) {
      modalContainer.innerHTML = '';
    }
    this.currentModal = null;
  }

  updateView() {
    const mainContent = document.getElementById('main-content');
    if (mainContent) {
      mainContent.innerHTML = this.render();
      this.attachEventListeners();
    }
  }
}
