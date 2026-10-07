"use strict";
const theme = document.querySelector("#site-theme");
try {
  document.body.classList.toggle(
    "day",
    localStorage.getItem("nsf-site-theme") === "day",
  );
} catch {}
theme.onclick = () => {
  document.body.classList.toggle("day");
  try {
    localStorage.setItem(
      "nsf-site-theme",
      document.body.classList.contains("day") ? "day" : "night",
    );
  } catch {}
  theme.setAttribute('aria-pressed', String(document.body.classList.contains('day')));
};
theme.setAttribute('aria-pressed', String(document.body.classList.contains('day')));

const palettes = ['citrus', 'glacier', 'orchid'];
function setPalette(value) {
  document.body.dataset.palette = palettes.includes(value) ? value : 'citrus';
  for (const button of document.querySelectorAll('[data-palette]')) {
    if (button.tagName === 'BUTTON') button.setAttribute('aria-pressed', String(button.dataset.palette === document.body.dataset.palette));
  }
}
try { setPalette(localStorage.getItem('nsf-site-palette')); } catch { setPalette('citrus'); }
for (const button of document.querySelectorAll('button[data-palette]')) button.onclick = () => {
  setPalette(button.dataset.palette);
  try { localStorage.setItem('nsf-site-palette', button.dataset.palette); } catch {}
};
const motion = document.querySelector('#site-motion');
function setMotion(paused) {
  document.body.classList.toggle('motion-paused', paused);
  motion.setAttribute('aria-pressed', String(paused));
  motion.textContent = paused ? 'Motion paused' : 'Pause motion';
}
try { setMotion(localStorage.getItem('nsf-site-motion') === 'paused'); } catch {}
motion.onclick = () => {
  const paused = !document.body.classList.contains('motion-paused');
  setMotion(paused);
  try { localStorage.setItem('nsf-site-motion', paused ? 'paused' : 'playing'); } catch {}
};
const observer = new IntersectionObserver(entries => {
  for (const entry of entries) if (entry.isIntersecting) {
    entry.target.classList.add('revealed');
    observer.unobserve(entry.target);
  }
}, { threshold: 0.08 });
for (const section of document.querySelectorAll('main > section:not(.hero)')) observer.observe(section);
const progress = document.querySelector('.reading-progress');
let scrollQueued = false;
function updateProgress() {
  const height = document.documentElement.scrollHeight - innerHeight;
  progress.style.transform = `scaleX(${height > 0 ? Math.min(1, Math.max(0, scrollY / height)) : 0})`;
  scrollQueued = false;
}
addEventListener('scroll', () => {
  if (!scrollQueued) { scrollQueued = true; requestAnimationFrame(updateProgress); }
}, { passive: true });
addEventListener('resize', updateProgress);
updateProgress();
const descriptions = {
  "desktop-dark.png": "NotSoFast desktop overview in the dark Iris theme",
  "desktop-overview.png": "NotSoFast desktop overview in daylight",
  "desktop-account.png":
    "NotSoFast local account, insights and optional cloud setup",
};
for (const button of document.querySelectorAll("[data-screen]"))
  button.onclick = () => {
    for (const item of document.querySelectorAll("[data-screen]"))
      item.setAttribute("aria-pressed", String(item === button));
    const picture = document.querySelector("#product-screen");
    picture.src = "/assets/" + button.dataset.screen;
    picture.alt = descriptions[button.dataset.screen];
  };
for (const button of document.querySelectorAll("[data-scope]"))
  button.onclick = () => {
    const full = button.dataset.scope === "full";
    for (const item of document.querySelectorAll("[data-scope]"))
      item.setAttribute("aria-pressed", String(item === button));
    const badge = document.querySelector("#decision");
    badge.textContent = full ? "REFUTED" : "UNKNOWN";
    badge.classList.toggle("refuted", full);
    document.querySelector("#decision-title").textContent = full
      ? "There it is. Now you know."
      : "A partial search leaves a gap.";
    document.querySelector("#decision-body").textContent = full
      ? "The complete search finds config/database.yaml. The exact absence claim is false, so the duplicate is blocked."
      : "Checking src/ cannot establish absence everywhere. The unsearched config/ directory still matters.";
    document.querySelector("#coverage-bar").style.width = full ? "100%" : "35%";
    document.querySelector("#coverage-caption").textContent = full
      ? "Complete coverage · matching witness found"
      : "Partial coverage · publication is not justified";
    document.querySelector("#witness").classList.toggle("found", full);
  };

// Scroll scrubs a local, seekable film. Manual controls work independently.
const filmSection = document.querySelector('#film');
const film = document.querySelector('#evidence-film');
const timeline = document.querySelector('#film-timeline');
const reduced = matchMedia('(prefers-reduced-motion: reduce)');
const portraitFilm = matchMedia('(max-width: 760px)');
let filmLoaded = false, filmTarget = 0, filmTick = false, lastSeek = -1, manualFilm = false;
function filmFormat() {
  film.poster = portraitFilm.matches ? '/assets/evidence-poster-mobile.jpg' : '/assets/evidence-poster.jpg';
  film.querySelector('source').src = portraitFilm.matches ? '/assets/evidence-film-mobile.mp4' : '/assets/evidence-film.mp4';
  lastSeek = -1;
  if (filmLoaded) film.load();
}
portraitFilm.addEventListener('change', filmFormat);
filmFormat();
const chapters = [
  ['01 / CHECK THE SCOPE', 'A partial search cannot prove that a filename is absent everywhere.'],
  ['02 / FIND THE MATCH', 'Search the missing coverage. Finding config/database.yaml refutes the absence claim.'],
  ['03 / BLOCK THE DUPLICATE', 'The exact-filename policy blocks the duplicate write. The source repository stays unchanged.']
];
function loadFilm() {
  if (!filmLoaded) { filmLoaded = true; film.load(); }
}
function seekFilm() {
  if (!film.seeking && Number.isFinite(film.duration) && film.duration > 0) {
    const target = filmTarget * Math.max(0, film.duration - 0.08);
    if (Math.abs(film.currentTime - target) > 0.045 && Math.abs(lastSeek - target) > .025) {
      lastSeek = target;
      film.currentTime = target;
    }
  }
}
function showFilm(value, manual = false) {
  filmTarget = Math.max(0, Math.min(1, value));
  timeline.value = String(Math.round(filmTarget * 100));
  document.querySelector('#film-percent').textContent = `${Math.round(filmTarget * 100)}%`;
  const chapter = filmTarget < 1/3 ? 0 : filmTarget < 2/3 ? 1 : 2;
  document.querySelector('#film-chapter').textContent = chapters[chapter][0];
  document.querySelector('#film-description').textContent = chapters[chapter][1];
  document.querySelectorAll('[data-frame]').forEach((button, index) => button.setAttribute('aria-pressed', String(index === chapter)));
  if (manual) { manualFilm = true; loadFilm(); }
  seekFilm();
}
film.addEventListener('loadedmetadata', seekFilm);
film.addEventListener('loadeddata', () => {
  filmSection.classList.remove('film-unavailable');
  document.querySelector('#film-hint').textContent = 'Scroll to reveal · or drag the timeline. Illustrated product walkthrough; no live repository is modified.';
});
film.addEventListener('seeked', seekFilm);
film.addEventListener('error', () => {
  filmSection.classList.add('film-unavailable');
  document.querySelector('#film-hint').textContent = 'Film unavailable. The still illustration and chapter descriptions remain available.';
});
for (const source of film.querySelectorAll('source')) source.addEventListener('error', () => film.dispatchEvent(new Event('error')));
function scrollFilm() {
  filmTick = false;
  if (manualFilm || reduced.matches || document.body.classList.contains('motion-paused') || innerHeight < 740) return;
  const bounds = filmSection.getBoundingClientRect();
  if (bounds.top > innerHeight || bounds.bottom < 0) return;
  loadFilm();
  showFilm(-bounds.top / Math.max(1, bounds.height - innerHeight));
}
addEventListener('scroll', () => {
  if (!filmTick) { filmTick = true; requestAnimationFrame(scrollFilm); }
}, { passive: true });
addEventListener('resize', scrollFilm);
addEventListener('wheel', () => { manualFilm = false; }, { passive: true });
addEventListener('touchmove', () => { manualFilm = false; }, { passive: true });
addEventListener('keydown', event => {
  if (event.target !== timeline && ['PageDown','PageUp','ArrowDown','ArrowUp','Home','End',' '].includes(event.key)) manualFilm = false;
});
reduced.addEventListener('change', scrollFilm);
timeline.addEventListener('input', () => showFilm(Number(timeline.value)/100, true));
for (const button of document.querySelectorAll('[data-frame]')) button.addEventListener('click', () => showFilm(Number(button.dataset.frame), true));
scrollFilm();
