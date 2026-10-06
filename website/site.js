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
