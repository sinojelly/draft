export default class AddAccountModal {
  constructor(onClose, onSubmit) {
    this.onClose = onClose;
    this.onSubmit = onSubmit;
    this.selectedType = '';
    this.showQR = false;
  }

  render() {
    return `
      <div class="modal-overlay active" id="accountModalOverlay">
        <div class="modal-content">
          <div class="modal-header">添加网盘账号</div>
          
          <form id="accountForm">
            <div class="form-group">
              <label class="form-label">账号别名</label>
              <input type="text" class="form-input" id="accountAlias" 
                placeholder="例如: 我的备份" required>
            </div>
            
            <div class="form-group">
              <label class="form-label">网盘类型</label>
              <select class="form-select" id="accountType" required>
                <option value="">请选择网盘类型</option>
                <option value="baidu">百度网盘</option>
                <option value="yike">一刻相册</option>
                <option value="other">其他网盘</option>
              </select>
            </div>

            <div id="qrCodeArea" class="hidden">
              <div class="qr-container">
                <div class="qr-code">
                  <svg width="160" height="160" viewBox="0 0 100 100">
                    <rect x="0" y="0" width="100" height="100" fill="white"/>
                    <rect x="5" y="5" width="10" height="10" fill="black"/>
                    <rect x="20" y="5" width="5" height="5" fill="black"/>
                    <rect x="30" y="5" width="10" height="5" fill="black"/>
                    <rect x="45" y="5" width="5" height="10" fill="black"/>
                    <rect x="55" y="5" width="5" height="5" fill="black"/>
                    <rect x="65" y="5" width="5" height="10" fill="black"/>
                    <rect x="75" y="5" width="5" height="5" fill="black"/>
                    <rect x="85" y="5" width="10" height="10" fill="black"/>
                    <rect x="5" y="20" width="5" height="5" fill="black"/>
                    <rect x="15" y="20" width="10" height="5" fill="black"/>
                    <rect x="30" y="20" width="5" height="10" fill="black"/>
                    <rect x="40" y="20" width="10" height="5" fill="black"/>
                    <rect x="55" y="20" width="5" height="10" fill="black"/>
                    <rect x="65" y="20" width="10" height="5" fill="black"/>
                    <rect x="80" y="20" width="5" height="5" fill="black"/>
                    <rect x="90" y="20" width="5" height="10" fill="black"/>
                  </svg>
                </div>
                <p class="qr-hint" id="qrHint">请使用百度网盘 App 扫描二维码登录</p>
              </div>
            </div>
            
            <div class="modal-actions">
              <button type="button" class="btn btn-secondary" id="cancelBtn">取消</button>
              <button type="submit" class="btn btn-primary">确认绑定</button>
            </div>
          </form>
        </div>
      </div>
    `;
  }

  attachEventListeners() {
    // Close on overlay click
    document.getElementById('accountModalOverlay')?.addEventListener('click', (e) => {
      if (e.target.id === 'accountModalOverlay') {
        this.onClose();
      }
    });

    // Cancel button
    document.getElementById('cancelBtn')?.addEventListener('click', () => {
      this.onClose();
    });

    // Account type change
    document.getElementById('accountType')?.addEventListener('change', (e) => {
      this.selectedType = e.target.value;
      this.showQR = this.selectedType === 'baidu' || this.selectedType === 'yike';
      
      const qrArea = document.getElementById('qrCodeArea');
      const qrHint = document.getElementById('qrHint');
      
      if (qrArea) {
        if (this.showQR) {
          qrArea.classList.remove('hidden');
          if (qrHint) {
            qrHint.textContent = `请使用 ${this.selectedType === 'baidu' ? '百度网盘' : '一刻相册'} App 扫描二维码登录`;
          }
        } else {
          qrArea.classList.add('hidden');
        }
      }
    });

    // Form submission
    document.getElementById('accountForm')?.addEventListener('submit', (e) => {
      e.preventDefault();
      
      const alias = document.getElementById('accountAlias').value;
      const type = document.getElementById('accountType').value;

      const accountData = {
        alias,
        type,
        name: type === 'baidu' ? '百度网盘' : type === 'yike' ? '一刻相册' : '其他网盘'
      };

      this.onSubmit(accountData);
    });
  }
}
