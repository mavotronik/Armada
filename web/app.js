const app = document.getElementById("app");
const sidebar = document.getElementById("sidebar");

let hostPollInterval = null;
let lastNodes = [];
let lastHostDevice = null;
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
    } else {
        showToast("Обновлено");
    }
});

const pages = {
    dashboard: renderDashboard,
    console: renderConsole,
};

function getPageFromHash() {
    const hash = location.hash.replace("#/", "");
    return pages[hash] ? hash : "dashboard";
}

function navigate() {
    stopHostPoll();
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
    await fetchNodes();
    renderDeviceTable();
}

async function fetchNodes() {
    try {
        const res = await fetch("/api/v1/nodes", { cache: "no-store" });
        if (!res.ok) return;
        const data = await res.json();
        lastNodes = data.nodes || [];
    } catch (e) {
        lastNodes = [];
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

    lastHostDevice = data.device || null;

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
    const statusText = status === "online" ? "Online" : status === "offline" ? "Offline" : status;
    const dot = status === "online" ? "online" : status === "offline" ? "offline" : "warning";
    return `<tr>
        <td><span class="mdi mdi-server"></span> ${esc(name)}</td>
        <td>${esc(type)}</td>
        <td>${esc(ip)}</td>
        <td><span class="status"><span class="status-dot status-${dot}"></span>${esc(statusText)}</span></td>
        <td>${esc(load)}</td>
        <td></td>
    </tr>`;
}

function esc(s) {
    return String(s ?? "").replace(/[&<>"']/g, (c) => ({
        "&": "&amp;",
        "<": "&lt;",
        ">": "&gt;",
        '"': "&quot;",
        "'": "&#39;",
    }[c]));
}

function renderDeviceTable() {
    const tbody = document.getElementById("deviceBody");
    if (!tbody) return;
    const rows = [];
    if (lastHostDevice) {
        const d = lastHostDevice;
        rows.push(deviceRowHtml(d.name, d.type, d.ip || "—", d.status, `${Math.round(d.loadPercent)}%`));
    }
    for (const n of lastNodes) {
        const host = n.host;
        const name = host?.hostname || `node ${n.id}`;
        const ip = host?.device?.ip || "—";
        const load = host ? `${Math.round(host.cpu?.usagePercent || 0)}%` : "—";
        rows.push(deviceRowHtml(name, "UART", ip, n.online ? "online" : "offline", load));
    }
    tbody.innerHTML = rows.join("");
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
