export default class AddTaskModal {
  constructor(onClose, onSubmit, existingTask = null) {
    this.onClose = onClose;
    this.onSubmit = onSubmit;
    this.existingTask = existingTask;
    this.showAdvanced = false;
    this.formData = existingTask || {
      localPath: '',
      remotePath: '',
      account: '百度网盘 - 我的备份',
      direction: 'upload',
      conflictStrategy: '自动更名',
      pollInterval: 60,
      syncDelete: false
    };
  }

  render() {
    const isEdit = !!this.existingTask;
    return `
      <div class="modal-overlay active" id="taskModalOverlay">
        <div class="modal-content">
          <div class="modal-header">${isEdit ? '编辑' : '添加'}同步任务</div>
          
          <form id="taskForm">
            <div class="form-group">
              <label class="form-label">本地路径</label>
              <input type="text" class="form-input" id="localPath" 
                placeholder="D:\\Documents" value="${this.formData.localPath}" required>
            </div>
            
            <div class="form-group">
              <label class="form-label">选择网盘账号</label>
              <select class="form-select" id="account">
                <option ${this.formData.account === '百度网盘 - 我的备份' ? 'selected' : ''}>百度网盘 - 我的备份</option>
                <option ${this.formData.account === '一刻相册 - 家庭相册' ? 'selected' : ''}>一刻相册 - 家庭相册</option>
                <option ${this.formData.account === '百度网盘 - 工作文档' ? 'selected' : ''}>百度网盘 - 工作文档</option>
              </select>
            </div>
            
            <div class="form-group">
              <label class="form-label">远程路径</label>
              <input type="text" class="form-input" id="remotePath" 
                placeholder="/备份/文档" value="${this.formData.remotePath}" required>
            </div>
            
            <div class="form-group">
              <label class="form-label">同步方向</label>
              <select class="form-select" id="direction">
                <option value="upload" ${this.formData.direction === 'upload' ? 'selected' : ''}>⬆️ 备份 (本地 → 云端)</option>
                <option value="download" ${this.formData.direction === 'download' ? 'selected' : ''}>⬇️ 下载 (云端 → 本地)</option>
                <option value="bidirectional" ${this.formData.direction === 'bidirectional' ? 'selected' : ''}>🔄 双向同步</option>
              </select>
            </div>

            <div class="collapsible-header ${this.showAdvanced ? 'open' : ''}" id="advancedToggle">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <polyline points="9 18 15 12 9 6"/>
              </svg>
              高级设置
            </div>
            
            <div class="collapsible-content ${this.showAdvanced ? 'open' : ''}" id="advancedContent">
              <div class="form-group">
                <label class="form-label">冲突策略</label>
                <select class="form-select" id="conflictStrategy">
                  <option ${this.formData.conflictStrategy === '自动更名' ? 'selected' : ''}>自动更名</option>
                  <option ${this.formData.conflictStrategy === '覆盖旧文件' ? 'selected' : ''}>覆盖旧文件</option>
                  <option ${this.formData.conflictStrategy === '跳过' ? 'selected' : ''}>跳过</option>
                </select>
              </div>
              
              <div class="form-group">
                <label class="form-label">轮询间隔 (秒)</label>
                <input type="number" class="form-input" id="pollInterval" 
                  value="${this.formData.pollInterval}" min="10" required>
              </div>
              
              <div class="form-group" style="display: flex; justify-content: space-between; align-items: center;">
                <label class="form-label" style="margin: 0;">同步删除操作</label>
                <div class="toggle-switch ${this.formData.syncDelete ? 'active' : ''}" id="syncDeleteToggle"></div>
              </div>
            </div>
            
            <div class="modal-actions">
              <button type="button" class="btn btn-secondary" id="cancelBtn">取消</button>
              <button type="submit" class="btn btn-primary">${isEdit ? '保存' : '创建'}任务</button>
            </div>
          </form>
        </div>
      </div>
    `;
  }

  attachEventListeners() {
    // Close on overlay click
    document.getElementById('taskModalOverlay')?.addEventListener('click', (e) => {
      if (e.target.id === 'taskModalOverlay') {
        this.onClose();
      }
    });

    // Cancel button
    document.getElementById('cancelBtn')?.addEventListener('click', () => {
      this.onClose();
    });

    // Advanced toggle
    document.getElementById('advancedToggle')?.addEventListener('click', () => {
      this.showAdvanced = !this.showAdvanced;
      const toggle = document.getElementById('advancedToggle');
      const content = document.getElementById('advancedContent');
      if (toggle && content) {
        toggle.classList.toggle('open');
        content.classList.toggle('open');
      }
    });

    // Sync delete toggle
    document.getElementById('syncDeleteToggle')?.addEventListener('click', (e) => {
      this.formData.syncDelete = !this.formData.syncDelete;
      e.target.classList.toggle('active');
    });

    // Form submission
    document.getElementById('taskForm')?.addEventListener('submit', (e) => {
      e.preventDefault();
      
      const localPath = document.getElementById('localPath').value;
      const remotePath = document.getElementById('remotePath').value;
      const account = document.getElementById('account').value;
      const direction = document.getElementById('direction').value;
      const conflictStrategy = document.getElementById('conflictStrategy').value;
      const pollInterval = parseInt(document.getElementById('pollInterval').value);

      const [accountType, alias] = account.split(' - ');

      const taskData = {
        localPath,
        remotePath,
        account: alias,
        accountType,
        direction,
        conflictStrategy,
        pollInterval,
        syncDelete: this.formData.syncDelete
      };

      this.onSubmit(taskData);
    });
  }
}
