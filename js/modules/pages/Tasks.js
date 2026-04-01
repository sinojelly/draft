export default class Tasks {
  constructor() {
    this.tasks = [
      {
        id: 1,
        localPath: 'D:\\Photos',
        remotePath: '/备份/照片',
        direction: 'upload',
        account: '我的备份',
        accountType: '百度网盘',
        status: 'active',
        pollInterval: 60,
        conflictStrategy: '自动更名'
      },
      {
        id: 2,
        localPath: 'D:\\Downloads\\Music',
        remotePath: '/音乐',
        direction: 'download',
        account: '家庭相册',
        accountType: '一刻相册',
        status: 'paused',
        pollInterval: 120,
        conflictStrategy: '覆盖'
      },
      {
        id: 3,
        localPath: 'D:\\Documents',
        remotePath: '/Documents',
        direction: 'bidirectional',
        account: '工作文档',
        accountType: '百度网盘',
        status: 'active',
        pollInterval: 30,
        conflictStrategy: '跳过'
      }
    ];
  }

  render() {
    return `
      <div class="page-header">
        <h1 class="page-title">同步任务</h1>
        <button class="btn btn-primary" id="addTaskBtn">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="12" y1="5" x2="12" y2="19"/>
            <line x1="5" y1="12" x2="19" y2="12"/>
          </svg>
          添加任务
        </button>
      </div>

      <div class="task-list">
        ${this.tasks.map(task => this.renderTask(task)).join('')}
      </div>

      <div id="toastContainer"></div>
    `;
  }

  renderTask(task) {
    const directionIcon = this.getDirectionIcon(task.direction);
    const directionColor = this.getDirectionColor(task.direction);
    const statusIcon = task.status === 'active' ? `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <rect x="6" y="4" width="4" height="16"/>
        <rect x="14" y="4" width="4" height="16"/>
      </svg>
    ` : `
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
        <polygon points="5 3 19 12 5 21 5 3"/>
      </svg>
    `;

    return `
      <div class="task-card">
        <div class="task-info">
          <div class="task-paths">
            <span class="task-path">${task.localPath}</span>
            <div class="task-arrow" style="color: ${directionColor}">
              ${directionIcon}
            </div>
            <span class="task-path">${task.remotePath}</span>
          </div>
          <div class="task-meta">
            <strong>${task.accountType}</strong> - ${task.account} • 每${task.pollInterval}秒轮询 • ${task.conflictStrategy}冲突文件
          </div>
        </div>
        <div class="task-actions">
          <button class="icon-btn" title="编辑">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/>
              <path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>
            </svg>
          </button>
          <button class="icon-btn" data-action="toggle" data-id="${task.id}" 
            title="${task.status === 'active' ? '暂停' : '启动'}"
            style="color: ${task.status === 'active' ? 'var(--text-primary)' : 'var(--success)'}">
            ${statusIcon}
          </button>
          <button class="icon-btn" data-action="delete" data-id="${task.id}" 
            title="删除" style="color: var(--danger)">
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
              <polyline points="3 6 5 6 21 6"/>
              <path d="M19 6v14a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2V6m3 0V4a2 2 0 0 1 2-2h4a2 2 0 0 1 2 2v2"/>
            </svg>
          </button>
        </div>
      </div>
    `;
  }

  getDirectionIcon(direction) {
    switch (direction) {
      case 'upload':
        return `
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="5" y1="12" x2="19" y2="12"/>
            <polyline points="12 5 19 12 12 19"/>
          </svg>
          备份
        `;
      case 'download':
        return `
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <line x1="19" y1="12" x2="5" y2="12"/>
            <polyline points="12 19 5 12 12 5"/>
          </svg>
          下载
        `;
      case 'bidirectional':
        return `
          <svg width="20" height="20" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <polyline points="17 1 21 5 17 9"/>
            <path d="M3 11V9a4 4 0 0 1 4-4h14"/>
            <polyline points="7 23 3 19 7 15"/>
            <path d="M21 13v2a4 4 0 0 1-4 4H3"/>
          </svg>
          双向
        `;
    }
  }

  getDirectionColor(direction) {
    switch (direction) {
      case 'upload': return 'var(--primary)';
      case 'download': return 'var(--success)';
      case 'bidirectional': return 'var(--warning)';
    }
  }

  attachEventListeners() {
    // Add task button
    document.getElementById('addTaskBtn')?.addEventListener('click', () => {
      alert('添加任务功能');
    });

    // Task action buttons
    document.querySelectorAll('[data-action="delete"]').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = parseInt(btn.dataset.id);
        if (confirm('确定要删除这个任务吗？')) {
          this.tasks = this.tasks.filter(t => t.id !== id);
          this.updateView();
          this.showToast('任务已删除');
        }
      });
    });

    document.querySelectorAll('[data-action="toggle"]').forEach(btn => {
      btn.addEventListener('click', () => {
        const id = parseInt(btn.dataset.id);
        const task = this.tasks.find(t => t.id === id);
        if (task) {
          task.status = task.status === 'active' ? 'paused' : 'active';
          this.updateView();
          this.showToast('任务状态已更新');
        }
      });
    });
  }

  updateView() {
    const taskList = document.querySelector('.task-list');
    if (taskList) {
      taskList.innerHTML = this.tasks.map(task => this.renderTask(task)).join('');
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
