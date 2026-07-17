// focusd landing — hero panel animation + live GitHub numbers.
// no framework, no libs. respects prefers-reduced-motion.

(() => {
  const reduce = matchMedia('(prefers-reduced-motion: reduce)').matches;
  const GH_REPO = '0xarchit/focusd';
  const CACHE_KEY = 'focusd:gh:v1';
  const TTL = 6 * 60 * 60 * 1000; // 6 hours

  // ---- panel bars: staggered reveal ------------------------------------
  const bars = document.querySelectorAll('.panel .app');
  bars.forEach((el, i) => {
    el.style.setProperty('--i', i);
    requestAnimationFrame(() => requestAnimationFrame(() => el.classList.add('is-in')));
  });

  // ---- panel counter: ease 0 → target ----------------------------------
  const num = document.querySelector('.panel__number[data-count-to]');
  if (num) {
    const target = parseInt(num.dataset.countTo, 10) || 0;
    if (reduce) {
      num.textContent = target;
    } else {
      const start = performance.now();
      const dur = 1400;
      const ease = (t) => 1 - Math.pow(1 - t, 3);
      const step = (now) => {
        const p = Math.min(1, (now - start) / dur);
        num.textContent = Math.round(target * ease(p));
        if (p < 1) requestAnimationFrame(step);
      };
      requestAnimationFrame(step);
    }
  }

  // ---- FAQ: single-open enforcement ------------------------------------
  const items = document.querySelectorAll('.faq details');
  items.forEach((d) => {
    d.addEventListener('toggle', () => {
      if (d.open) items.forEach((o) => { if (o !== d) o.open = false; });
    });
  });

  // ---- Scroll reveal: cascade [data-reveal] elements into view ---------
  // ponytail: one observer for the whole page; each element unobserves
  //           once revealed so we don't pay for scroll frames forever.
  const targets = document.querySelectorAll('[data-reveal]');
  if (targets.length) {
    if (reduce || !('IntersectionObserver' in window)) {
      targets.forEach((el) => el.classList.add('is-visible'));
    } else {
      const io = new IntersectionObserver((entries) => {
        entries.forEach((e) => {
          if (e.isIntersecting) {
            e.target.classList.add('is-visible');
            io.unobserve(e.target);
          }
        });
      }, { rootMargin: '0px 0px -12% 0px', threshold: 0.12 });
      targets.forEach((el) => io.observe(el));
    }
  }

  // ---- GitHub API: version + downloads + stars -------------------------
  // ponytail: two unauthenticated calls, cached in localStorage 6h.
  //           silent fail keeps the static fallbacks visible.
  const setText = (sel, val) => {
    document.querySelectorAll(sel).forEach(el => { el.textContent = val; });
  };
  const compact = (n) => {
    if (n >= 1000) return (n / 1000).toFixed(n < 10000 ? 1 : 0).replace(/\.0$/, '') + 'k';
    return String(n);
  };
  const nf = new Intl.NumberFormat('en');

  function apply(d) {
    if (d.version) setText('[data-js="version"]', d.version);
    if (d.downloads > 0) {
      setText('[data-js="downloads"]', nf.format(d.downloads));
      document.querySelector('[data-js="downloads-wrap"]')?.removeAttribute('hidden');
    }
    revealStar(d.stars);
  }

  function revealStar(stars) {
    const el = document.querySelector('[data-js="starw"]');
    if (!el) return;
    if (typeof stars === 'number' && stars >= 0) {
      setText('[data-js="stars"]', compact(stars));
    } else {
      setText('[data-js="stars"]', '★');
    }
    el.hidden = false;
    // wait a frame so transition catches
    requestAnimationFrame(() => requestAnimationFrame(() => el.classList.add('is-in')));
  }

  function readCache() {
    try {
      const raw = localStorage.getItem(CACHE_KEY);
      if (!raw) return null;
      const d = JSON.parse(raw);
      if (!d || (Date.now() - d.ts) > TTL) return null;
      return d;
    } catch { return null; }
  }
  function writeCache(d) {
    try { localStorage.setItem(CACHE_KEY, JSON.stringify(d)); } catch {}
  }

  async function loadGitHub() {
    const cached = readCache();
    if (cached) { apply(cached); return; }

    // Ensure the widget shows even if the network is dead — fall back to
    // an unknown count after a short timeout.
    const fallback = setTimeout(() => revealStar(null), 1200);

    try {
      const [repoRes, relRes] = await Promise.all([
        fetch(`https://api.github.com/repos/${GH_REPO}`, { headers: { Accept: 'application/vnd.github+json' } }),
        fetch(`https://api.github.com/repos/${GH_REPO}/releases?per_page=100`, { headers: { Accept: 'application/vnd.github+json' } })
      ]);
      if (!repoRes.ok || !relRes.ok) throw new Error('gh api');

      const repo = await repoRes.json();
      const releases = await relRes.json();
      clearTimeout(fallback);

      const stars = repo.stargazers_count | 0;
      const latest = Array.isArray(releases) && releases.length ? releases[0] : null;
      const version = latest?.tag_name || null;
      const downloads = Array.isArray(releases)
        ? releases.reduce((sum, r) =>
            sum + (r.assets || []).reduce((s, a) => s + (a.download_count | 0), 0), 0)
        : 0;

      const data = { stars, version, downloads, ts: Date.now() };
      writeCache(data);
      apply(data);
    } catch {
      /* keep static fallbacks — widget still reveals via `fallback` */
    }
  }

  loadGitHub();
})();
