const commands = {
  help: "Available commands: [help] [status] [features] [download] [clear] [theme]",
  status: "focusd is ONLINE. Privacy protocols active. Zero data exfiltration.",
  features:
    "Features: [PRIVACY] [TUI DASHBOARD] [BROWSER TRACKING] [POMODORO] [APP LIMITS] [BREAK REMINDERS] [DATA CONTROL] [AUTO-START]",
  download: "Redirecting to GitHub releases...",
  clear: "CLEAR_ACTION",
  theme: "THEME_ACTION",
  focusd:
    "<span style='color:#0f0'>Launching focusd TUI...</span><br><span style='color:#888'>  ___  ___  ___  ___  ___  ___  ___<br> | F || O || C || U || S || D || ↑ |<br> |___||___||___||___||___||___||___|</span><br><span style='color:#0f0'>● Dashboard  ○ Stats  ○ Focus  ○ Limits  ○ Settings</span><br><span style='color:#555'>(simulation — install focusd to experience the real TUI)</span>",
};

let matrixInterval;
let matrixSpeed = 50;
let sfxEnabled = false;
let audioCtx;
let noiseBuffer = null;
let systemStarted = false;

document.addEventListener("DOMContentLoaded", () => {
  const prefersReducedMotion = window.matchMedia(
    "(prefers-reduced-motion: reduce)",
  ).matches;

  initTheme();
  initSFX();
  fetchLatestVersion();

  if (prefersReducedMotion) {
    document.getElementById("start-overlay").style.display = "none";
    revealSections();
    document.querySelector(".terminal-input-area").classList.remove("hidden");
  }

  initTerminal();
});

async function startSystem() {
  if (systemStarted) return;
  systemStarted = true;

  const overlay = document.getElementById("start-overlay");
  overlay.style.opacity = "0";
  setTimeout(() => overlay.remove(), 500);

  if (sfxEnabled || localStorage.getItem("sfx") !== "false") {
    initAudioContext();
    if (audioCtx.state === "suspended") await audioCtx.resume();
    sfxEnabled = true;
  }

  initMatrix();
  runBootSequence();
}

async function runBootSequence() {
  const history = document.getElementById("terminal-history");
  const inputArea = document.querySelector(".terminal-input-area");

  inputArea.classList.add("hidden");

  const wait = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

  async function typeLine(text, element) {
    element.innerHTML = 'C:\\Users\\You> <span class="typing-cursor"></span>';

    let currentText = "C:\\Users\\You> ";
    element.textContent = currentText;

    const span = document.createElement("span");
    span.className = "typing-cursor";
    element.appendChild(span);

    for (let char of text) {
      await wait(Math.random() * 30 + 10);
      if (sfxEnabled) playClick();
      currentText += char;
      element.firstChild.textContent = currentText;
    }

    await wait(100);
    span.remove();
    element.innerHTML += "<br>";
  }

  await wait(100);
  const bootLine = document.createElement("div");
  bootLine.className = "command-output";
  history.appendChild(bootLine);
  await typeLine("focusd.exe", bootLine);

  const logs = [
    "Initializing daemon core...",
    "Loading local SQLite database... [OK]",
    "Starting background tracker...",
    "<span style='color: #0f0'>SUCCESS: focusd installed. Type 'focusd' in Terminal to launch.</span>",
  ];

  for (let log of logs) {
    await wait(100);

    if (log.includes("SUCCESS")) {
      if (sfxEnabled) playSuccess();
    } else {
      if (sfxEnabled && Math.random() > 0.5) playClick();
    }

    addToHistory(log, true);
  }

  await wait(300);

  revealSections();

  await wait(200);
  inputArea.classList.remove("hidden");
  document.getElementById("cmd-input").focus();
  addToHistory(
    "<br>Interactive shell ready. Type 'help' for commands.<br>",
    true,
  );
}

function initSFX() {
  const btn = document.getElementById("sfx-btn");
  const saved = localStorage.getItem("sfx");

  if (saved === null || saved === "true") {
    sfxEnabled = true;
    btn.innerText = "[SFX: ON]";
  } else {
    sfxEnabled = false;
    btn.innerText = "[SFX: OFF]";
  }
}

function initAudioContext() {
  if (!audioCtx) {
    audioCtx = new (window.AudioContext || window.webkitAudioContext)({
      latencyHint: "interactive",
    });

    const bufferSize = audioCtx.sampleRate;
    noiseBuffer = audioCtx.createBuffer(1, bufferSize, audioCtx.sampleRate);
    const data = noiseBuffer.getChannelData(0);
    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }
  }
  if (audioCtx.state === "suspended") audioCtx.resume().catch(() => {});
}

function toggleSFX() {
  sfxEnabled = !sfxEnabled;
  const btn = document.getElementById("sfx-btn");
  btn.innerText = sfxEnabled ? "[SFX: ON]" : "[SFX: OFF]";
  localStorage.setItem("sfx", sfxEnabled);

  if (sfxEnabled) {
    initAudioContext();
    playClick();
  }
}

function playClick() {
  if (!sfxEnabled) return;
  initAudioContext();
  const t = audioCtx.currentTime;

  const osc = audioCtx.createOscillator();
  const gain = audioCtx.createGain();

  osc.type = "square";
  osc.frequency.setValueAtTime(600, t);
  osc.frequency.exponentialRampToValueAtTime(100, t + 0.015);

  gain.gain.setValueAtTime(0.08, t);
  gain.gain.exponentialRampToValueAtTime(0.001, t + 0.015);

  osc.connect(gain);
  gain.connect(audioCtx.destination);

  osc.start();
  osc.stop(t + 0.02);

  if (noiseBuffer) {
    const noise = audioCtx.createBufferSource();
    noise.buffer = noiseBuffer;

    const noiseFilter = audioCtx.createBiquadFilter();
    noiseFilter.type = "lowpass";
    noiseFilter.frequency.value = 2500;

    const noiseGain = audioCtx.createGain();
    noiseGain.gain.setValueAtTime(0.12, t);
    noiseGain.gain.exponentialRampToValueAtTime(0.001, t + 0.02);

    noise.connect(noiseFilter);
    noiseFilter.connect(noiseGain);
    noiseGain.connect(audioCtx.destination);

    noise.start();
    noise.stop(t + 0.025);
  }
}

function playSuccess() {
  if (!sfxEnabled) return;
  initAudioContext();
  const t = audioCtx.currentTime;

  const freqs = [523.25, 659.25, 783.99, 987.77];

  freqs.forEach((f, i) => {
    const osc = audioCtx.createOscillator();
    const gain = audioCtx.createGain();

    osc.type = "sine";
    osc.frequency.setValueAtTime(f, t);

    gain.gain.setValueAtTime(0, t);
    gain.gain.linearRampToValueAtTime(0.15, t + 0.05);
    gain.gain.exponentialRampToValueAtTime(0.001, t + 1.2);

    osc.connect(gain);
    gain.connect(audioCtx.destination);

    const delay = i * 0.04;
    osc.start(t + delay);
    osc.stop(t + delay + 1.3);
  });
}

function initMatrix() {
  const canvas = document.getElementById("matrix-canvas");
  if (!canvas) return;
  const ctx = canvas.getContext("2d");

  canvas.width = window.innerWidth;
  canvas.height = window.innerHeight;

  const katakana =
    "アァカサタナハマヤャラワガザダバパイィキシチニヒミリヰギジヂビピウゥクスツヌフムユュルグズブヅプエェケセテネヘメレヱゲゼデベペオォコソトノホモヨョロヲゴゾドボポヴッン";
  const latin = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";
  const nums = "0123456789";
  const alphabet = katakana + latin + nums;

  const fontSize = 16;
  const columns = canvas.width / fontSize;
  const drops = [];

  for (let x = 0; x < columns; x++) {
    drops[x] = 1;
  }

  function draw() {
    ctx.fillStyle = document.body.classList.contains("paper-mode")
      ? "rgba(240, 240, 240, 0.1)"
      : "rgba(12, 12, 12, 0.05)";
    ctx.fillRect(0, 0, canvas.width, canvas.height);

    if (document.body.classList.contains("god-mode")) {
      ctx.fillStyle = "#ff0000";
    } else if (document.body.classList.contains("paper-mode")) {
      ctx.fillStyle = "#000";
    } else {
      ctx.fillStyle = "#0F0";
    }

    ctx.font = fontSize + "px monospace";

    for (let i = 0; i < drops.length; i++) {
      const text = alphabet.charAt(Math.floor(Math.random() * alphabet.length));
      ctx.fillText(text, i * fontSize, drops[i] * fontSize);

      if (drops[i] * fontSize > canvas.height && Math.random() > 0.975) {
        drops[i] = 0;
      }
      drops[i]++;
    }
  }

  startMatrixLoop(draw);

  window.addEventListener("resize", () => {
    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;
  });
}

function startMatrixLoop(drawFn) {
  if (matrixInterval) clearInterval(matrixInterval);
  matrixInterval = setInterval(drawFn, matrixSpeed);
}

function initTerminal() {
  const input = document.getElementById("cmd-input");
  const history = document.getElementById("terminal-history");

  input.addEventListener("keydown", function (e) {
    if (sfxEnabled) playClick();

    if (e.key === "Enter") {
      const cmd = this.value.trim().toLowerCase();
      this.value = "";

      addToHistory(`C:\\Users\\You> ${cmd}`);

      if (cmd === "") return;

      if (commands[cmd]) {
        const response = commands[cmd];
        if (response === "CLEAR_ACTION") {
          history.innerHTML = "";
        } else if (response === "THEME_ACTION") {
          toggleTheme();
          addToHistory("Theme toggled.");
        } else {
          const allowedHTMLKeys = ["focusd"];
          const isAllowedHTML = allowedHTMLKeys.includes(cmd);
          addToHistory(response, isAllowedHTML);
          if (cmd === "download") {
            setTimeout(() => {
              window.open(
                "https://github.com/0xarchit/focusd/releases",
                "_blank",
              );
            }, 1000);
          }
        }
      } else if (cmd.startsWith("focusd")) {
        addToHistory(commands.focusd, true);
      } else {
        addToHistory(
          `'${cmd}' is not recognized as an internal or external command.`,
        );
      }

      const offset = input.parentElement.offsetTop;
      window.scrollTo({ top: offset - 200, behavior: "smooth" });
    }
  });

  document.addEventListener("click", (e) => {
    if (!e.target.closest("button") && !e.target.closest("a")) {
      const input = document.getElementById("cmd-input");
      if (input && !input.parentElement.classList.contains("hidden")) {
        input.focus();
      }
    }
  });
}

function addToHistory(text, isHTML = false) {
  const history = document.getElementById("terminal-history");
  if (!history) return;
  const p = document.createElement("div");
  p.className = "command-output";
  if (isHTML) {
    p.innerHTML = text;
  } else {
    p.textContent = text;
  }
  history.appendChild(p);
}

function fetchLatestVersion() {
  const badge = document.getElementById("github-badge");
  fetch("https://api.github.com/repos/0xarchit/focusd/releases/latest")
    .then((response) => response.json())
    .then((data) => {
      if (data.tag_name) {
        const ver = data.tag_name;
        badge.textContent = `Latest: ${ver}`;
        commands.status = `focusd ${ver} is ONLINE. Privacy protocols active. Zero data exfiltration.`;
      }
    })
    .catch((err) => {
      console.error("Failed to fetch release:", err);
      badge.textContent = "Latest: (offline)";
    });
}

function copyToClipboard(text, successMsg) {
  if (!navigator.clipboard || !navigator.clipboard.writeText) {
    showToast("Failed to copy: Clipboard API unavailable");
    return;
  }
  navigator.clipboard
    .writeText(text)
    .then(() => {
      showToast(successMsg);
    })
    .catch((err) => {
      console.error("Could not copy text: ", err);
      showToast("Failed to copy command");
    });
}

function copyCommand() {
  const cmd = `iwr "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" -OutFile focusd_setup.exe; ./focusd_setup.exe`;
  copyToClipboard(cmd, "PowerShell command copied");
}

function copyCurl() {
  const cmd = `curl -L -o focusd_setup.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd_setup.exe" && focusd_setup.exe`;
  copyToClipboard(cmd, "CMD command copied");
}

function showToast(msg) {
  const x = document.getElementById("toast");
  x.innerText = "[SYSTEM] " + msg;
  x.className = "show";
  setTimeout(function () {
    x.className = x.className.replace("show", "");
  }, 3000);
}

function initTheme() {
  const saved = localStorage.getItem("theme");
  const btn = document.getElementById("theme-btn");
  if (saved === "paper") {
    document.body.classList.add("paper-mode");
    btn.innerText = "[DARK MODE]";
  }
}

function toggleTheme() {
  const body = document.body;
  const btn = document.getElementById("theme-btn");
  body.classList.toggle("paper-mode");

  if (body.classList.contains("paper-mode")) {
    localStorage.setItem("theme", "paper");
    btn.innerText = "[DARK MODE]";
  } else {
    localStorage.setItem("theme", "dark");
    btn.innerText = "[LIGHT MODE]";
  }
}

function revealSections() {
  const sections = document.querySelectorAll(".delayed-reveal");
  sections.forEach((sec, index) => {
    sec.classList.add("visible");
  });
}
