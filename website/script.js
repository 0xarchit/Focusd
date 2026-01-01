const commands = {
    help: "Available commands: [help] [status] [features] [download] [clear] [theme] [matrix]",
    status: "focusd v1.0.0 is ONLINE. Privacy protocols active. Zero leaks detected.",
    features: "Features: [PRIVACY] [PERFORMANCE] [CONTROL] [INSIGHTS] [POMODORO] [LIMITS]",
    download: "Redirecting to GitHub releases...",
    clear: "CLEAR_ACTION",
    theme: "THEME_ACTION",
    matrix: "MATRIX_ACTION",
    focusd: "Usage: focusd [command]. Try 'focusd help'."
};

let matrixInterval;
let matrixSpeed = 50;
let sfxEnabled = false;
let audioCtx;
let noiseBuffer = null;

document.addEventListener('DOMContentLoaded', () => {
    
    const prefersReducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    
    initTheme();
    initSFX(); 
    initTiltCards();
    initKonamiCode();
    fetchLatestVersion();

    if (prefersReducedMotion) {
        
        document.getElementById('start-overlay').style.display = 'none';
        revealSections();
        document.querySelector('.terminal-input-area').classList.remove('hidden');
    } 
    
    
    initTerminal(); 
});

async function startSystem() {
    const overlay = document.getElementById('start-overlay');
    overlay.style.opacity = '0';
    setTimeout(() => overlay.remove(), 500);

    
    if (sfxEnabled || localStorage.getItem('sfx') !== 'false') {
        initAudioContext();
        if (audioCtx.state === 'suspended') await audioCtx.resume();
        sfxEnabled = true; 
    }

    initMatrix();
    runBootSequence(); 
}


async function runBootSequence() {
    const history = document.getElementById('terminal-history');
    const inputArea = document.querySelector('.terminal-input-area');
    
    
    inputArea.classList.add('hidden');

    const wait = (ms) => new Promise(resolve => setTimeout(resolve, ms));

    async function typeLine(text, element) {
        element.innerHTML = 'C:\\Users\\You> <span class="typing-cursor"></span>';
        
        let currentText = "C:\\Users\\You> ";
        element.textContent = currentText; 
        
        const span = document.createElement('span');
        span.className = 'typing-cursor';
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
    const bootLine = document.createElement('div');
    bootLine.className = 'command-output';
    history.appendChild(bootLine);
    await typeLine("focusd.exe", bootLine);

    
    const logs = [
        "Initializing core protocols...",
        "Verifying local database integrity... [OK]",
        "Loading UI modules...",
        "<span style='color: #0f0'>SUCCESS: focusd initialized successfully.</span>"
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
    inputArea.classList.remove('hidden');
    document.getElementById('cmd-input').focus();
    addToHistory("<br>Interactive shell ready. Type 'help' for commands.<br>", true);
}


function initSFX() {
    const btn = document.getElementById('sfx-btn');
    const saved = localStorage.getItem('sfx');
    
    if (saved === null || saved === 'true') {
        sfxEnabled = true;
        btn.innerText = "[SFX: ON]";
    } else {
        sfxEnabled = false;
        btn.innerText = "[SFX: OFF]";
    }
}

function initAudioContext() {
    if (!audioCtx) {
        
        audioCtx = new (window.AudioContext || window.webkitAudioContext)({ latencyHint: 'interactive' });
        
        
        const bufferSize = audioCtx.sampleRate;
        noiseBuffer = audioCtx.createBuffer(1, bufferSize, audioCtx.sampleRate);
        const data = noiseBuffer.getChannelData(0);
        for (let i = 0; i < bufferSize; i++) {
            data[i] = Math.random() * 2 - 1;
        }
    }
    if (audioCtx.state === 'suspended') audioCtx.resume().catch(() => {});
}

function toggleSFX() {
    sfxEnabled = !sfxEnabled;
    const btn = document.getElementById('sfx-btn');
    btn.innerText = sfxEnabled ? "[SFX: ON]" : "[SFX: OFF]";
    localStorage.setItem('sfx', sfxEnabled);
    
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
    
    osc.type = 'square';
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
        noiseFilter.type = 'lowpass'; 
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
        
        osc.type = 'sine';
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
    const canvas = document.getElementById('matrix-canvas');
    if (!canvas) return;
    const ctx = canvas.getContext('2d');

    canvas.width = window.innerWidth;
    canvas.height = window.innerHeight;

    const katakana = 'アァカサタナハマヤャラワガザダバパイィキシチニヒミリヰギジヂビピウゥクスツヌフムユュルグズブヅプエェケセテネヘメレヱゲゼデベペオォコソトノホモヨョロヲゴゾドボポヴッン';
    const latin = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789';
    const nums = '0123456789';
    const alphabet = katakana + latin + nums;

    const fontSize = 16;
    const columns = canvas.width / fontSize;
    const drops = [];

    for (let x = 0; x < columns; x++) {
        drops[x] = 1;
    }

    function draw() {
        ctx.fillStyle = document.body.classList.contains('paper-mode') ? 'rgba(240, 240, 240, 0.1)' : 'rgba(12, 12, 12, 0.05)';
        ctx.fillRect(0, 0, canvas.width, canvas.height);

        if (document.body.classList.contains('god-mode')) {
            ctx.fillStyle = '#ff0000'; 
        } else if (document.body.classList.contains('paper-mode')) {
            ctx.fillStyle = '#000';
        } else {
            ctx.fillStyle = '#0F0';
        }
        
        ctx.font = fontSize + 'px monospace';

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

    window.addEventListener('resize', () => {
        canvas.width = window.innerWidth;
        canvas.height = window.innerHeight;
    });
}

function startMatrixLoop(drawFn) {
    if (matrixInterval) clearInterval(matrixInterval);
    matrixInterval = setInterval(drawFn, matrixSpeed);
}


function initTerminal() {
    const input = document.getElementById('cmd-input');
    const history = document.getElementById('terminal-history');

    input.addEventListener('keydown', function(e) {
        if (sfxEnabled) playClick();

        if (e.key === 'Enter') {
            const cmd = this.value.trim().toLowerCase();
            this.value = '';

            addToHistory(`C:\\Users\\You> ${cmd}`);

            if (cmd === '') return;

            if (commands[cmd]) {
                const response = commands[cmd];
                if (response === 'CLEAR_ACTION') {
                    history.innerHTML = '';
                } else if (response === 'THEME_ACTION') {
                    toggleTheme();
                    addToHistory("Theme toggled.");
                } else if (response === 'MATRIX_ACTION') {
                    matrixSpeed = (matrixSpeed === 50) ? 20 : 50;
                    addToHistory(matrixSpeed === 20 ? "Matrix intensity: HIGH" : "Matrix intensity: NORMAL");
                } else {
                    addToHistory(response);
                    if (cmd === 'download') {
                        setTimeout(() => {
                            window.open('https://github.com/0xarchit/Focusd/releases', '_blank');
                        }, 1000);
                    }
                }
            } else if (cmd.startsWith('focusd')) {
                 addToHistory("Executing focusd daemon... (simulation)");
                 addToHistory(commands.help);
            } else {
                addToHistory(`'${cmd}' is not recognized as an internal or external command.`);
            }
            
            const offset = input.parentElement.offsetTop;
            window.scrollTo({ top: offset - 200, behavior: 'smooth' });
        }
    });

    document.addEventListener('click', (e) => {
        if (!e.target.closest('button') && !e.target.closest('a')) {
            const input = document.getElementById('cmd-input');
            if (input && !input.parentElement.classList.contains('hidden')) {
                input.focus();
            }
        }
    });
}

function addToHistory(text, isHTML = false) {
    const history = document.getElementById('terminal-history');
    if (!history) return;
    const p = document.createElement('div');
    p.className = 'command-output';
    if (isHTML) {
        p.innerHTML = text;
    } else {
        p.textContent = text;
    }
    history.appendChild(p);
}


function initTiltCards() {
    const cards = document.querySelectorAll('.card');
    cards.forEach(card => {
        card.addEventListener('mousemove', (e) => {
            const rect = card.getBoundingClientRect();
            const x = e.clientX - rect.left;
            const y = e.clientY - rect.top;
            
            const centerX = rect.width / 2;
            const centerY = rect.height / 2;
            
            const rotateX = ((y - centerY) / centerY) * -10; 
            const rotateY = ((x - centerX) / centerX) * 10;
            
            card.style.transform = `perspective(1000px) rotateX(${rotateX}deg) rotateY(${rotateY}deg) scale3d(1.02, 1.02, 1.02)`;
        });
        
        card.addEventListener('mouseleave', () => {
            card.style.transform = 'perspective(1000px) rotateX(0) rotateY(0)';
        });
    });
}


function initKonamiCode() {
    const code = ['ArrowUp', 'ArrowUp', 'ArrowDown', 'ArrowDown', 'ArrowLeft', 'ArrowRight', 'ArrowLeft', 'ArrowRight', 'b', 'a'];
    let index = 0;
    
    document.addEventListener('keydown', (e) => {
        if (e.key === code[index]) {
            index++;
            if (index === code.length) {
                activateGodMode();
                index = 0;
            }
        } else {
            index = 0;
        }
    });
}

function activateGodMode() {
    showToast("GOD MODE ACTIVATED");
    document.body.classList.add('god-mode');
    document.documentElement.style.setProperty('--accent-color', '#ff0000');
    document.documentElement.style.setProperty('--text-color', '#ffaaaa');
}


function fetchLatestVersion() {
    const badge = document.getElementById('github-badge');
    fetch('https://api.github.com/repos/0xarchit/Focusd/releases/latest')
        .then(response => response.json())
        .then(data => {
            if (data.tag_name) {
                const ver = data.tag_name;
                badge.textContent = `Latest: ${ver}`;
                commands.status = `focusd ${ver} is ONLINE. Privacy protocols active. Zero leaks detected.`;
            }
        })
        .catch(err => {
            console.error('Failed to fetch release:', err);
            badge.textContent = 'Latest: v1.0.0 (offline)';
        });
}

function copyToClipboard(text, successMsg) {
    if (navigator.clipboard) {
        navigator.clipboard.writeText(text).then(() => {
            showToast(successMsg);
        }).catch(err => {
            console.error('Async: Could not copy text: ', err);
            fallbackCopyText(text, successMsg);
        });
    } else {
        fallbackCopyText(text, successMsg);
    }
}

function fallbackCopyText(text, successMsg) {
    var textArea = document.createElement("textarea");
    textArea.value = text;
    textArea.style.top = "0";
    textArea.style.left = "0";
    textArea.style.position = "fixed";
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();

    try {
        var successful = document.execCommand('copy');
        if (successful) {
            showToast(successMsg);
        } else {
            showToast("Copy failed. Please select manually.");
        }
    } catch (err) {
        console.error('Fallback: Oops, unable to copy', err);
        showToast("Copy error.");
    }

    document.body.removeChild(textArea);
}

function copyCommand() {
    const cmd = `iwr "https://github.com/0xarchit/Focusd/releases/latest/download/focusd.exe" -OutFile focusd.exe; ./focusd.exe init`;
    copyToClipboard(cmd, "PowerShell command copied");
}

function copyCurl() {
    const cmd = `curl -L -o focusd.exe "https://github.com/0xarchit/focusd/releases/latest/download/focusd.exe" && focusd.exe init`;
    copyToClipboard(cmd, "CMD command copied");
}

function showToast(msg) {
    const x = document.getElementById("toast");
    x.innerText = "[SYSTEM] " + msg;
    x.className = "show";
    setTimeout(function(){ x.className = x.className.replace("show", ""); }, 3000);
}

function initTheme() {
    const saved = localStorage.getItem('theme');
    const btn = document.getElementById('theme-btn');
    if (saved === 'paper') {
        document.body.classList.add('paper-mode');
        btn.innerText = "[DARK MODE]";
    }
}

function toggleTheme() {
    const body = document.body;
    const btn = document.getElementById('theme-btn');
    body.classList.toggle('paper-mode');

    if (body.classList.contains('paper-mode')) {
        localStorage.setItem('theme', 'paper');
        btn.innerText = "[DARK MODE]";
    } else {
        localStorage.setItem('theme', 'dark');
        btn.innerText = "[LIGHT MODE]";
    }
}

function revealSections() {
    const sections = document.querySelectorAll('.delayed-reveal');
    sections.forEach((sec, index) => {
        sec.classList.add('visible');
    });
}
