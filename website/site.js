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
};
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
