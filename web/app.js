const app = document.getElementById("app");
const sidebar = document.getElementById("sidebar");

let hostPollInterval = null;
let gpioPollInterval = null;
let gpioGridBuilt = false;
let cpuHistory = [];
let charts = [];
let consoleSession = null;

function showToast(message) {
    const container = document.getElementById("toastContainer");
    const toast = document.createElement("div");
    toast.className = "toast";
    toast.innerHTML = `
        <span class="mdi mdi-check-circle-outline" style="color:var(--success)"></span>
        <span>${message}</span>`;
    container.appendChild(toast);
    setTimeout(() => {
        toast.style.opacity = "0";
        toast.style.transform = "translateY(10px)";
        setTimeout(() => toast.remove(), 200);
    }, 2500);
}

document.getElementById("menuButton").addEventListener("click", () => {
    if (window.innerWidth <= 700) {
        sidebar.classList.toggle("mobile-open");
    } else {
        sidebar.classList.toggle("collapsed");
        localStorage.setItem("sidebar-collapsed", sidebar.classList.contains("collapsed"));
    }
});

if (localStorage.getItem("sidebar-collapsed") === "true") {
    sidebar.classList.add("collapsed");
}

const savedTheme = localStorage.getItem("dashboard-theme");
if (savedTheme) {
    document.documentElement.dataset.theme = savedTheme;
}

document.getElementById("themeButton").addEventListener("click", () => {
    const next = document.documentElement.dataset.theme === "dark" ? "light" : "dark";
    document.documentElement.dataset.theme = next;
    localStorage.setItem("dashboard-theme", next);
    redrawCharts();
});

document.getElementById("refreshButton").addEventListener("click", () => {
    if (getPageFromHash() === "dashboard") {
        fetchHost(true);
    } else if (getPageFromHash() === "gpio") {
        fetchGPIO(true);
    } else {
        showToast("Обновлено");
    }
});

const pages = {
    dashboard: renderDashboard,
    console: renderConsole,
    gpio: renderGPIO,
};

function getPageFromHash() {
    const hash = location.hash.replace("#/", "");
    return pages[hash] ? hash : "dashboard";
}

function navigate() {
    stopHostPoll();
    stopGPIOPoll();
    teardownConsole();

    const page = getPageFromHash();
    document.querySelectorAll(".nav-item").forEach(item => {
        item.classList.toggle("active", item.dataset.page === page);
    });
    pages[page]();
    sidebar.classList.remove("mobile-open");
    window.scrollTo({ top: 0, behavior: "instant" });
}

window.addEventListener("hashchange", navigate);

function formatBytes(n) {
    if (!n) return "0 B";
    const units = ["B", "KB", "MB", "GB", "TB"];
    let i = 0;
    let v = n;
    while (v >= 1024 && i < units.length - 1) {
        v /= 1024;
        i++;
    }
    return `${v.toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

function formatBps(bps) {
    if (bps >= 1_000_000) return `${(bps / 1_000_000).toFixed(1)} Mbps`;
    if (bps >= 1_000) return `${(bps / 1_000).toFixed(0)} Kbps`;
    return `${Math.round(bps)} B/s`;
}

function progressClass(pct) {
    if (pct >= 90) return "danger";
    if (pct >= 75) return "warning";
    return "";
}

async function fetchHost(manual) {
    try {
        const res = await fetch("/api/v1/host", { cache: "no-store" });
        if (!res.ok) throw new Error(res.statusText);
        const data = await res.json();
        applyHostData(data);
        if (manual) showToast("Данные обновлены");
    } catch (e) {
        if (manual) showToast("Ошибка загрузки метрик");
    }
}

function applyHostData(data) {
    cpuHistory.push(data.cpu.usagePercent);
    if (cpuHistory.length > 60) cpuHistory.shift();

    setText("cpuValue", `${Math.round(data.cpu.usagePercent)}%`);
    setWidth("cpuProgress", data.cpu.usagePercent);

    const tempEl = document.getElementById("tempValue");
    if (tempEl) {
        tempEl.textContent = data.temperatureC != null
            ? `${data.temperatureC.toFixed(1)}°C`
            : "—";
    }

    const netRx = (data.network || []).reduce((s, n) => s + (n.rxBps || 0), 0);
    const netTx = (data.network || []).reduce((s, n) => s + (n.txBps || 0), 0);
    setText("networkValue", formatBps(netRx + netTx));

    setText("diskValue", formatBytes(data.disk.usedBytes));
    setWidth("diskProgress", data.disk.usagePercent);
    const diskStatus = document.getElementById("diskStatus");
    if (diskStatus) {
        diskStatus.textContent = `${Math.round(data.disk.usagePercent)}%`;
    }

    setText("hostSubtitle", `${data.hostname} · ${data.kernel || "linux"} · uptime ${formatUptime(data.uptimeSeconds)}`);

    updateSystemProgress("progCpu", data.cpu.usagePercent);
    updateSystemProgress("progRam", data.ram.usagePercent);
    updateSystemProgress("progDisk", data.disk.usagePercent);

    setText("loadValue", `${data.cpu.load1.toFixed(2)} / ${data.cpu.load5.toFixed(2)} / ${data.cpu.load15.toFixed(2)}`);

    const tbody = document.getElementById("deviceBody");
    if (tbody && data.device) {
        const d = data.device;
        tbody.innerHTML = deviceRowHtml(d.name, d.type, d.ip || "—", d.status, `${Math.round(d.loadPercent)}%`);
    }

    const netList = document.getElementById("netList");
    if (netList && data.network) {
        netList.innerHTML = data.network.map(n => networkItemHtml(n)).join("");
    }

    updateCpuChart();
}

function setText(id, text) {
    const el = document.getElementById(id);
    if (el) el.textContent = text;
}

function setWidth(id, pct) {
    const el = document.getElementById(id);
    if (el) {
        el.style.width = `${Math.min(100, Math.max(0, pct))}%`;
        el.className = `progress-value ${progressClass(pct)}`;
    }
}

function updateSystemProgress(id, pct) {
    const bar = document.getElementById(id);
    const label = document.getElementById(id + "Label");
    if (bar) {
        bar.style.width = `${pct}%`;
        bar.className = `progress-value ${progressClass(pct)}`;
    }
    if (label) label.textContent = `${Math.round(pct)}%`;
}

function formatUptime(sec) {
    const d = Math.floor(sec / 86400);
    const h = Math.floor((sec % 86400) / 3600);
    const m = Math.floor((sec % 3600) / 60);
    if (d > 0) return `${d}d ${h}h`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m`;
}

function deviceRowHtml(name, type, ip, status, load) {
    const statusText = status === "online" ? "Online" : status;
    return `<tr>
        <td><span class="mdi mdi-server"></span> ${name}</td>
        <td>${type}</td>
        <td>${ip}</td>
        <td><span class="status"><span class="status-dot status-${status === "online" ? "online" : "warning"}"></span>${statusText}</span></td>
        <td>${load}</td>
        <td></td>
    </tr>`;
}

function networkItemHtml(n) {
    const addrs = (n.addresses || []).join(", ") || "—";
    const traffic = `${formatBps(n.rxBps || 0)} ↓ / ${formatBps(n.txBps || 0)} ↑`;
    return `<div class="list-item">
        <div class="list-icon"><span class="mdi mdi-ethernet"></span></div>
        <div class="list-content">
            <div class="list-title">${n.name}</div>
            <div class="list-description">${addrs}</div>
        </div>
        <div style="text-align:right;font-size:13px">${traffic}</div>
    </div>`;
}

function renderDashboard() {
    cpuHistory = [];
    clearCharts();

    app.innerHTML = `
        <div class="page-header">
            <div>
                <h1 class="page-title">Обзор</h1>
                <div class="page-subtitle" id="hostSubtitle">Загрузка…</div>
            </div>
        </div>
        <section class="metrics">
            <div class="card metric">
                <div class="metric-top">
                    <div class="metric-icon"><span class="mdi mdi-chip"></span></div>
                    <span class="status"><span class="status-dot status-online pulse"></span>Live</span>
                </div>
                <div class="metric-label">CPU</div>
                <div class="metric-value" id="cpuValue">—</div>
                <div class="progress"><div id="cpuProgress" class="progress-value" style="width:0"></div></div>
            </div>
            <div class="card metric">
                <div class="metric-top">
                    <div class="metric-icon"><span class="mdi mdi-thermometer"></span></div>
                    <span class="status"><span class="status-dot status-online"></span>SoC</span>
                </div>
                <div class="metric-label">Температура</div>
                <div class="metric-value" id="tempValue">—</div>
            </div>
            <div class="card metric">
                <div class="metric-top">
                    <div class="metric-icon"><div class="network-animation">
                        <div class="network-bar"></div><div class="network-bar"></div><div class="network-bar"></div>
                        <div class="network-bar"></div><div class="network-bar"></div>
                    </div></div>
                    <span class="status"><span class="status-dot status-online"></span>Net</span>
                </div>
                <div class="metric-label">Сеть</div>
                <div class="metric-value"><span id="networkValue">—</span></div>
            </div>
            <div class="card metric">
                <div class="metric-top">
                    <div class="metric-icon"><span class="mdi mdi-harddisk disk-icon"></span></div>
                    <span class="status"><span class="status-dot status-warning" id="diskStatus">—</span></span>
                </div>
                <div class="metric-label">Диск /</div>
                <div class="metric-value" id="diskValue">—</div>
                <div class="progress"><div id="diskProgress" class="progress-value" style="width:0"></div></div>
            </div>
        </section>
        <section class="dashboard-grid">
            <div class="card">
                <div class="card-header">
                    <div class="card-title">CPU Load</div>
                    <span style="margin-left:auto" class="status"><span class="status-dot status-online pulse"></span>Live</span>
                </div>
                <div class="card-body"><div class="chart-container"><canvas id="cpuChart"></canvas></div></div>
            </div>
            <div class="card">
                <div class="card-header"><div class="card-title">Состояние системы</div></div>
                <div class="card-body">
                    ${systemProgressBlock("CPU", "progCpu")}
                    ${systemProgressBlock("RAM", "progRam")}
                    ${systemProgressBlock("Disk", "progDisk")}
                    <div style="margin-top:16px;font-size:13px;color:var(--text-secondary)">
                        Load avg: <span id="loadValue">—</span>
                    </div>
                </div>
            </div>
        </section>
        <div class="grid-2">
            <div class="card">
                <div class="card-header"><div class="card-title">Сетевые интерфейсы</div></div>
                <div class="card-body"><div class="list" id="netList"></div></div>
            </div>
            <div class="card">
                <div class="card-header"><div class="card-title">Устройства</div></div>
                <div class="table-wrapper">
                    <table>
                        <thead><tr><th>Устройство</th><th>Тип</th><th>IP</th><th>Статус</th><th>Нагрузка</th><th></th></tr></thead>
                        <tbody id="deviceBody"></tbody>
                    </table>
                </div>
            </div>
        </div>`;

    initCpuChartCanvas();
    fetchHost(false);
    hostPollInterval = setInterval(() => {
        if (getPageFromHash() !== "dashboard") {
            stopHostPoll();
            return;
        }
        fetchHost(false);
    }, 1000);
}

function systemProgressBlock(label, id) {
    return `<div style="margin-bottom:18px">
        <div style="display:flex;justify-content:space-between"><span>${label}</span><span id="${id}Label" style="color:var(--text-secondary)">—</span></div>
        <div class="progress"><div id="${id}" class="progress-value" style="width:0"></div></div>
    </div>`;
}

function stopHostPoll() {
    if (hostPollInterval) {
        clearInterval(hostPollInterval);
        hostPollInterval = null;
    }
}

function stopGPIOPoll() {
    if (gpioPollInterval) {
        clearInterval(gpioPollInterval);
        gpioPollInterval = null;
    }
    gpioGridBuilt = false;
}

function renderGPIO() {
    gpioGridBuilt = false;
    app.innerHTML = `
        <div class="page-header">
            <div>
                <h1 class="page-title">GPIO</h1>
                <div class="page-subtitle">Luckfox Pico Max · in/out, PWM, сохранение состояния</div>
            </div>
        </div>
        <div class="gpio-alert" id="gpioStubBanner" style="display:none">
            <span class="mdi mdi-information-outline"></span>
            Режим заглушки: sysfs GPIO недоступен (Docker / x86). Настройки сохраняются в файл, но пины не переключаются.
        </div>
        <div class="gpio-grid" id="gpioGrid">
            <div class="card"><div class="card-body">Загрузка…</div></div>
        </div>`;

    fetchGPIO(false);
    gpioPollInterval = setInterval(() => {
        if (getPageFromHash() !== "gpio") {
            stopGPIOPoll();
            return;
        }
        fetchGPIO(false);
    }, 1000);
}

async function fetchGPIO(manual) {
    try {
        const res = await fetch("/api/v1/gpio", { cache: "no-store" });
        if (!res.ok) throw new Error(res.statusText);
        const data = await res.json();
        applyGPIOData(data);
        if (manual) showToast("GPIO обновлено");
    } catch (e) {
        if (manual) showToast("Ошибка загрузки GPIO");
    }
}

function applyGPIOData(data) {
    const banner = document.getElementById("gpioStubBanner");
    if (banner) {
        banner.style.display = data.hardware ? "none" : "block";
    }
    const grid = document.getElementById("gpioGrid");
    if (!grid) return;

    if (!gpioGridBuilt) {
        grid.innerHTML = (data.pins || []).map(pinCardHtml).join("");
        grid.addEventListener("change", onGPIOChange);
        grid.addEventListener("click", onGPIOClicks);
        gpioGridBuilt = true;
    } else {
        for (const pin of data.pins || []) {
            updatePinLiveFields(pin);
        }
    }
}

function pinCardHtml(pin) {
    const cfg = pin.config || { mode: "in" };
    const isOut = cfg.mode === "out";
    const isPwm = isOut && cfg.output === "pwm";
    const level = pin.level != null ? (pin.level ? "1 (HIGH)" : "0 (LOW)") : "—";
    const uartWarn = pin.uart
        ? `<div class="gpio-alert">
            <span class="mdi mdi-alert-outline"></span>
            Контакт может быть занят ${escapeHtml(pin.uart)} (${escapeHtml(pin.uartLine || "")}).
            Перевод в GPIO нарушит работу UART на этих пинах.
           </div>`
        : "";
    const muxWarn = pin.muxNote
        ? `<div class="gpio-alert">
            <span class="mdi mdi-alert-outline"></span>
            ${escapeHtml(pin.muxNote)}
           </div>`
        : "";
    const volt = pin.voltageNote
        ? `<div class="gpio-meta">Логика ${escapeHtml(pin.voltageNote)}</div>`
        : "";
    const err = pin.lastError
        ? `<div class="gpio-error pin-error">${escapeHtml(pin.lastError)}</div>`
        : `<div class="gpio-error pin-error" style="display:none"></div>`;
    const pwmBackend = pin.pwmBackend
        ? `<div class="gpio-meta pin-pwm-backend">PWM: ${escapeHtml(pin.pwmBackend)}</div>`
        : `<div class="gpio-meta pin-pwm-backend" style="display:none"></div>`;

    return `<div class="card" data-gpio-id="${escapeHtml(pin.id)}">
        <div class="card-header">
            <div class="card-title">${escapeHtml(pin.label)}</div>
            <span style="margin-left:auto;font-size:12px;color:var(--text-secondary)">GPIO ${pin.linuxGpio}</span>
        </div>
        <div class="card-body">
            ${uartWarn}
            ${muxWarn}
            ${volt}
            <div class="form-group">
                <label class="form-label">Режим</label>
                <select class="select" name="mode" data-field="mode">
                    <option value="in" ${cfg.mode === "in" ? "selected" : ""}>Вход</option>
                    <option value="out" ${cfg.mode === "out" ? "selected" : ""}>Выход</option>
                </select>
            </div>
            <div class="form-group gpio-out-fields" style="display:${isOut ? "block" : "none"}">
                <label class="form-label">Тип выхода</label>
                <select class="select" name="output" data-field="output">
                    <option value="digital" ${cfg.output !== "pwm" ? "selected" : ""}>Цифровой</option>
                    <option value="pwm" ${cfg.output === "pwm" ? "selected" : ""}>PWM</option>
                </select>
            </div>
            <div class="switch-row gpio-digital-row" style="display:${isOut && !isPwm ? "flex" : "none"}">
                <span>Уровень</span>
                <label class="switch">
                    <input type="checkbox" name="value" data-field="value" ${cfg.value ? "checked" : ""}>
                    <span class="slider"></span>
                </label>
            </div>
            <div class="gpio-pwm-fields" style="display:${isPwm ? "block" : "none"}">
                <div class="form-group">
                    <label class="form-label">Частота, Гц</label>
                    <input class="input" type="number" min="1" max="2000" step="1" name="frequencyHz"
                        data-field="frequencyHz" value="${Number(cfg.frequencyHz || 1000)}">
                </div>
                <div class="form-group">
                    <label class="form-label">Скважность, %</label>
                    <input class="input" type="number" min="0" max="100" step="1" name="dutyPercent"
                        data-field="dutyPercent" value="${Number(cfg.dutyPercent || 0)}">
                </div>
            </div>
            <div class="gpio-meta">Уровень / вход: <span class="pin-level">${level}</span></div>
            ${pwmBackend}
            ${err}
            <div style="display:flex;gap:8px;margin-top:14px;flex-wrap:wrap">
                <button type="button" class="button" data-action="default">Сделать умолчанием</button>
                <button type="button" class="button" data-action="reset">Вернуть умолчание</button>
            </div>
        </div>
    </div>`;
}

function updatePinLiveFields(pin) {
    const root = document.querySelector(`[data-gpio-id="${pin.id}"]`);
    if (!root) return;
    const levelEl = root.querySelector(".pin-level");
    if (levelEl) {
        levelEl.textContent = pin.level != null ? (pin.level ? "1 (HIGH)" : "0 (LOW)") : "—";
    }
    const errEl = root.querySelector(".pin-error");
    if (errEl) {
        if (pin.lastError) {
            errEl.style.display = "block";
            errEl.textContent = pin.lastError;
        } else {
            errEl.style.display = "none";
            errEl.textContent = "";
        }
    }
    const pwmEl = root.querySelector(".pin-pwm-backend");
    if (pwmEl) {
        if (pin.pwmBackend) {
            pwmEl.style.display = "block";
            pwmEl.textContent = `PWM: ${pin.pwmBackend}`;
        } else {
            pwmEl.style.display = "none";
        }
    }
}

function onGPIOChange(ev) {
    const root = ev.target.closest("[data-gpio-id]");
    if (!root || !ev.target.dataset.field) return;
    refreshPinVisibility(root);
    applyPinConfig(root);
}

function onGPIOClicks(ev) {
    const btn = ev.target.closest("button[data-action]");
    if (!btn) return;
    const root = btn.closest("[data-gpio-id]");
    if (!root) return;
    const id = root.dataset.gpioId;
    const action = btn.dataset.action;
    if (action === "default") {
        postGPIOAction(id, "default");
    } else if (action === "reset") {
        postGPIOAction(id, "reset");
    }
}

function refreshPinVisibility(root) {
    const mode = root.querySelector('[name="mode"]').value;
    const outWrap = root.querySelector(".gpio-out-fields");
    const out = root.querySelector('[name="output"]')?.value || "digital";
    const isOut = mode === "out";
    if (outWrap) outWrap.style.display = isOut ? "block" : "none";
    const digital = root.querySelector(".gpio-digital-row");
    const pwm = root.querySelector(".gpio-pwm-fields");
    if (digital) digital.style.display = isOut && out !== "pwm" ? "flex" : "none";
    if (pwm) pwm.style.display = isOut && out === "pwm" ? "block" : "none";
}

function configFromCard(root) {
    const mode = root.querySelector('[name="mode"]').value;
    const cfg = { mode };
    if (mode === "out") {
        cfg.output = root.querySelector('[name="output"]').value;
        if (cfg.output === "pwm") {
            cfg.frequencyHz = Number(root.querySelector('[name="frequencyHz"]').value) || 1000;
            cfg.dutyPercent = Number(root.querySelector('[name="dutyPercent"]').value) || 0;
        } else {
            cfg.value = root.querySelector('[name="value"]').checked;
        }
    }
    return cfg;
}

async function applyPinConfig(root) {
    const id = root.dataset.gpioId;
    const cfg = configFromCard(root);
    try {
        const res = await fetch(`/api/v1/gpio/${encodeURIComponent(id)}`, {
            method: "PUT",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify(cfg),
        });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || res.statusText);
        }
        fetchGPIO(false);
    } catch (e) {
        showToast(`Ошибка ${id}: ${e.message}`);
    }
}

async function postGPIOAction(id, action) {
    try {
        const res = await fetch(`/api/v1/gpio/${encodeURIComponent(id)}/${action}`, { method: "POST" });
        if (!res.ok) {
            const text = await res.text();
            throw new Error(text || res.statusText);
        }
        showToast(action === "default" ? "Умолчание сохранено" : "Применено умолчание");
        gpioGridBuilt = false;
        fetchGPIO(false);
    } catch (e) {
        showToast(e.message);
    }
}

function escapeHtml(s) {
    return String(s)
        .replace(/&/g, "&amp;")
        .replace(/</g, "&lt;")
        .replace(/>/g, "&gt;")
        .replace(/"/g, "&quot;");
}

function renderConsole() {
    app.innerHTML = `
        <div class="page-header">
            <div>
                <h1 class="page-title">Консоль</h1>
                <div class="page-subtitle">Локальный shell на этом узле (/bin/sh)</div>
            </div>
        </div>
        <div class="card">
            <div class="card-body" style="padding:0">
                <div id="terminal" style="height:min(70vh,560px);padding:8px;background:#0d1117"></div>
            </div>
        </div>`;

    if (typeof Terminal === "undefined") {
        document.getElementById("terminal").textContent = "xterm.js не загружен";
        return;
    }

    const term = new Terminal({
        cursorBlink: true,
        fontFamily: "ui-monospace, monospace",
        fontSize: 14,
        theme: { background: "#0d1117" },
    });
    const FitAddonClass =
        typeof FitAddon === "function"
            ? FitAddon
            : FitAddon?.FitAddon;
    if (!FitAddonClass) {
        document.getElementById("terminal").textContent =
            "FitAddon не загружен (xterm addon-fit)";
        return;
    }
    const fitAddon = new FitAddonClass();
    term.loadAddon(fitAddon);
    term.open(document.getElementById("terminal"));
    fitAddon.fit();

    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    const ws = new WebSocket(`${proto}//${location.host}/ws/console`);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
        sendResize(ws, term, fitAddon);
    };
    ws.onmessage = (ev) => {
        if (ev.data instanceof ArrayBuffer) {
            term.write(new Uint8Array(ev.data));
        }
    };
    ws.onclose = () => term.writeln("\r\n\r\n[соединение закрыто]");
    term.onData(data => {
        if (ws.readyState === WebSocket.OPEN) {
            ws.send(new TextEncoder().encode(data));
        }
    });

    const onResize = () => {
        fitAddon.fit();
        sendResize(ws, term, fitAddon);
    };
    window.addEventListener("resize", onResize);

    consoleSession = {
        teardown: () => {
            window.removeEventListener("resize", onResize);
            try { ws.close(); } catch (_) {}
            term.dispose();
        },
    };
}

function sendResize(ws, term, fitAddon) {
    if (ws.readyState !== WebSocket.OPEN) return;
    fitAddon.fit();
    ws.send(JSON.stringify({
        type: "resize",
        cols: term.cols,
        rows: term.rows,
    }));
}

function teardownConsole() {
    if (consoleSession) {
        consoleSession.teardown();
        consoleSession = null;
    }
}

let cpuChartDraw = null;

function initCpuChartCanvas() {
    const canvas = document.getElementById("cpuChart");
    if (!cpuChartDraw) {
        cpuChartDraw = createChartRenderer(canvas, 0, 100);
        charts.push(cpuChartDraw);
    } else {
        cpuChartDraw.setCanvas(canvas);
    }
}

function updateCpuChart() {
    if (cpuChartDraw) {
        cpuChartDraw.setValues(cpuHistory.length ? [...cpuHistory] : [0]);
    }
}

function clearCharts() {
    charts.forEach(c => c.destroy());
    charts = [];
    cpuChartDraw = null;
}

function redrawCharts() {
    charts.forEach(c => c.draw());
}

function createChartRenderer(canvas, min, max) {
    if (!canvas) return { setCanvas() {}, setValues() {}, draw() {}, destroy() {} };

    let values = [];
    let ctx = canvas.getContext("2d");
    let interval = null;

    function resize() {
        if (!canvas) return;
        const rect = canvas.getBoundingClientRect();
        const ratio = window.devicePixelRatio || 1;
        canvas.width = rect.width * ratio;
        canvas.height = rect.height * ratio;
        ctx.setTransform(ratio, 0, 0, ratio, 0, 0);
    }

    function draw() {
        if (!canvas) return;
        const width = canvas.clientWidth;
        const height = canvas.clientHeight;
        ctx.clearRect(0, 0, width, height);
        const styles = getComputedStyle(document.documentElement);
        const border = styles.getPropertyValue("--border");
        const primary = styles.getPropertyValue("--primary");
        ctx.strokeStyle = border;
        ctx.lineWidth = 1;
        for (let i = 1; i < 5; i++) {
            const y = (height * i) / 5;
            ctx.beginPath();
            ctx.moveTo(0, y);
            ctx.lineTo(width, y);
            ctx.stroke();
        }
        if (values.length < 2) return;
        ctx.beginPath();
        values.forEach((value, index) => {
            const x = (index * width) / (values.length - 1);
            const y = height - ((value - min) / (max - min)) * height;
            if (index === 0) ctx.moveTo(x, y);
            else ctx.lineTo(x, y);
        });
        ctx.strokeStyle = primary;
        ctx.lineWidth = 2;
        ctx.stroke();
        ctx.lineTo(width, height);
        ctx.lineTo(0, height);
        ctx.closePath();
        ctx.globalAlpha = 0.08;
        ctx.fillStyle = primary;
        ctx.fill();
        ctx.globalAlpha = 1;
    }

    function onResize() {
        resize();
        draw();
    }

    resize();
    draw();
    window.addEventListener("resize", onResize);
    interval = setInterval(draw, 700);

    return {
        setCanvas(c) {
            canvas = c;
            ctx = canvas.getContext("2d");
            resize();
            draw();
        },
        setValues(v) {
            values = v;
            draw();
        },
        draw,
        destroy() {
            window.removeEventListener("resize", onResize);
            if (interval) clearInterval(interval);
        },
    };
}

if (!location.hash) {
    history.replaceState(null, "", "#/");
}
navigate();
