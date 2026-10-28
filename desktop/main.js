// Xidex desktop shell: opens the hosted dashboard (or admin panel) in its own
// window, with its own icon. It follows Windows light or dark mode and has its own switch.
const { app, BrowserWindow, shell, Menu, nativeTheme } = require('electron');
const os = require('os');
const path = require('path');
const fs = require('fs');

let cfg = { serverUrl: 'https://techet-pay.onrender.com', app: 'merchant' };
try { cfg = { ...cfg, ...JSON.parse(fs.readFileSync(path.join(__dirname, 'config.json'), 'utf8')) }; } catch (e) {}
if (process.env.TECHET_SERVER_URL) cfg.serverUrl = process.env.TECHET_SERVER_URL;

const base = cfg.serverUrl.replace(/\/+$/, '');
const isAdmin = cfg.app === 'admin';
const startUrl = base + (isAdmin ? '/admin' : '/dashboard');
const title = isAdmin ? 'Xidex Admin' : 'Xidex';
const origin = new URL(base).origin;

const dark = nativeTheme.shouldUseDarkColors;
const WAKING = 'data:text/html;charset=utf-8,' + encodeURIComponent(`<!doctype html><meta charset=utf-8>
<body style="margin:0;height:100vh;display:grid;place-items:center;font:16px system-ui;color:${dark ? '#F2F4F1' : '#111311'};background:${dark ? '#0C0D0C' : '#EFF0EE'}">
<div style="text-align:center"><div style="display:inline-block;padding:6px 18px;border-radius:99px;background:#91DA33;color:#0F1210;font-weight:800;font-size:20px">${title}</div>
<p style="opacity:.7">Connecting to the server. The free server can take up to a minute to wake up&hellip;</p></div>`);

// Each app needs its own identity, otherwise the merchant and admin apps share one
// "single instance" lock and launching one just focuses the other.
app.setName(title);
app.setPath('userData', path.join(app.getPath('appData'), isAdmin ? 'Xidex Admin' : 'Xidex'));
if (process.platform === 'win32') app.setAppUserModelId(isAdmin ? 'et.techet.pay.admin' : 'et.techet.pay.desktop');

if (!app.requestSingleInstanceLock()) { app.quit(); }

let win;
function load() { win.loadURL(startUrl).catch(() => {}); }

function createWindow() {
  win = new BrowserWindow({
    width: 1360, height: 860, minWidth: 420, minHeight: 600,
    title, show: false, autoHideMenuBar: true,
    icon: path.join(__dirname, 'build', 'icon.ico'),
    backgroundColor: dark ? '#0C0D0C' : '#EFF0EE',
    webPreferences: { contextIsolation: true, sandbox: true, nodeIntegration: false, spellcheck: false },
  });
  Menu.setApplicationMenu(null);
  win.once('ready-to-show', () => win.show());

  // Server asleep or offline: show a friendly page and keep retrying.
  win.webContents.on('did-fail-load', (_e, code, _d, url, isMain) => {
    if (!isMain || code === -3) return;
    if (!url.startsWith('data:')) win.loadURL(WAKING).catch(() => {});
    setTimeout(load, 6000);
  });
  // Anything that is not our own server (checkout links, docs) opens in the normal browser.
  win.webContents.setWindowOpenHandler(({ url }) => { shell.openExternal(url); return { action: 'deny' }; });
  win.webContents.on('will-navigate', (e, url) => {
    if (url.startsWith('data:') || url.startsWith(origin)) return;
    e.preventDefault(); shell.openExternal(url);
  });
  load();
}

app.on('second-instance', () => { if (win) { if (win.isMinimized()) win.restore(); win.focus(); } });
app.whenReady().then(createWindow);
app.on('window-all-closed', () => app.quit());
