import App from './modules/App.js';
import { initToast } from './utils/toast.js';

// Initialize toast system
initToast();

// Initialize the application
document.addEventListener('DOMContentLoaded', () => {
  const app = new App();
  app.init();
});
