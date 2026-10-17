// TechET Pay desktop shell: opens the hosted dashboard (or admin panel) in its own
// window. On Windows 11 the window uses real acrylic (frosted glass) material.
const { app, BrowserWindow, shell, Menu } = require('electron');
const os = require('os');
const path = require('path');
const fs = require('fs');

let cfg = { serverUrl: 'https://techet-pay.onrender.com', app: 'merchant' };
try { cfg = { ...cfg, ...JSON.parse(fs.readFileSync(path.join(__dirname, 'config.json'), 'utf8')) }; } catch (e) {}
if (process.env.TECHET_SERVER_URL) cfg.serverUrl = process.env.TECHET_SERVER_URL;

const base = cfg.serverUrl.replace(/\/+$/, '');
const isAdmin = cfg.app === 'admin';
const startUrl = base + (isAdmin ? '/admin' : '/dashboard');
const title = isAdmin ? 'TechET Pay Admin' : 'TechET Pay';
const origin = new URL(base).origin;

// Windows 11 = build 22000+. Acrylic only exists there; Windows 10 gets a solid window.
const build = parseInt((os.release().split('.')[2] || '0'), 10);
const glass = process.platform === 'win32' && build >= 22000;

// Make the page background see-through so the acrylic shows behind the glass cards.
const GLASS_CSS = `
html,body{background:transparent!important}
body{background:
  radial-gradient(55% 38% at 6% 4%,rgba(47,134,224,.45) 0%,transparent 70%),
  radial-gradient(50% 34% at 98% 22%,rgba(142,193,242,.55) 0%,transparent 72%),
  radial-gradient(60% 40% at 10% 76%,rgba(20,58,143,.35) 0%,transparent 70%),
  linear-gradient(180deg,rgba(242,246,252,.50) 0%,rgba(187,217,243,.40) 50%,rgba(134,179,234,.45) 100%)!important}
`;

const WAKING = 'data:text/html;charset=utf-8,' + encodeURIComponent(`<!doctype html><meta charset=utf-8>
<body style="margin:0;height:100vh;display:grid;place-items:center;font:16px system-ui;color:#0B1B4D;
background:linear-gradient(160deg,#F2F6FC,#BBD9F3 55%,#5FA8F5)"><div style="text-align:center">
<div style="font-size:22px;font-weight:800">${title}</div>
<p>Connecting to the server. The free server can take up to a minute to wake up&hellip;</p></div>`);

if (!app.requestSingleInstanceLock()) { app.quit(); }

let win;
function load() { win.loadURL(startUrl).catch(() => {}); }

function createWindow() {
  win = new BrowserWindow({
    width: 1360, height: 860, minWidth: 420, minHeight: 600,
    title, show: false, autoHideMenuBar: true,
    icon: path.join(__dirname, 'build', 'icon.ico'),
    backgroundColor: glass ? '#00000000' : '#BBD9F3',
    ...(glass ? { backgroundMaterial: 'acrylic' } : {}),
    webPreferences: { contextIsolation: true, sandbox: true, nodeIntegration: false, spellcheck: false },
  });
  Menu.setApplicationMenu(null);
  win.once('ready-to-show', () => win.show());

  win.webContents.on('did-finish-load', () => {
    if (glass && win.webContents.getURL().startsWith(origin)) win.webContents.insertCSS(GLASS_CSS).catch(() => {});
  });
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
