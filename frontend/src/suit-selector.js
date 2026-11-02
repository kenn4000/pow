function escapeHtml(value) {
  return String(value ?? "").replaceAll("&", "&amp;").replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;").replaceAll('"', "&quot;").replaceAll("'", "&#39;");
}

export function renderSuitCycleWidget(container, config) {
  const markerID = `${container.id}-arrowhead`;
  const locations = ["scissors", "paper", "rock"];
  container.innerHTML = `<svg class="suit-arrows" viewBox="0 0 400 300" aria-hidden="true">
      <defs><marker id="${markerID}" markerWidth="10" markerHeight="10" refX="8" refY="5" orient="auto">
        <path d="M0,0 L10,5 L0,10 z" fill="#c5cbc7"></path>
      </marker></defs>
      <path class="connector" d="M145 141 L90 232"></path>
      <path class="connector" d="M200 232 L310 232"></path>
      <path class="connector" d="M255 141 L200 56"></path>
      <path style="marker-end:url(#${markerID})" d="M178 96 L145 141"></path>
      <path style="marker-end:url(#${markerID})" d="M140 232 L200 232"></path>
      <path style="marker-end:url(#${markerID})" d="M288 187 L255 141"></path>
    </svg>${config.nodes.map((node, index) => {
      const isOpen = config.openIndex === index;
      const menu = isOpen && config.canChange
        ? `<div class="color-menu" role="menu" aria-label="${escapeHtml(config.menuLabel(index))}">
            ${config.options(index).map(option =>
              `<button type="button" class="color-option" role="menuitem" data-suit-option="${index}" data-suit-value="${escapeHtml(option.value)}">
                <span class="color-option-swatch" style="background:${escapeHtml(option.swatch)}"></span>${escapeHtml(option.label)}
              </button>`).join("")}
          </div>`
        : "";
      return `<div class="suit-node-wrap ${locations[index]}${isOpen ? " menu-open" : ""}">
        <button type="button" class="suit-node" data-suit-trigger="${index}"
          style="--suit-node-color:${escapeHtml(node.color)}"
          aria-label="${escapeHtml(node.ariaLabel)}" aria-haspopup="menu" aria-expanded="${isOpen}"
          ${config.canChange ? "" : "disabled"}>
          ${node.content}
        </button>${menu}
      </div>`;
    }).join("")}<button type="button" class="rotate-colors" data-rotate-colors
      aria-label="${escapeHtml(config.rotateLabel)}" title="${escapeHtml(config.rotateLabel)}"
      ${config.canChange ? "" : "disabled"}>
      <svg viewBox="0 0 24 24" aria-hidden="true">
        <path d="M7.11 8.53A5.99 5.99 0 0 1 12 6c3.31 0 6 2.69 6 6s-2.69 6-6 6a5.99 5.99 0 0 1-5.65-4H4.26A8 8 0 0 0 12 20c4.42 0 8-3.58 8-8s-3.58-8-8-8c-2.21 0-4.21.9-5.66 2.34L4 4v6h6L7.11 8.53z"></path>
      </svg>
    </button>`;
  container.onclick = async event => {
    event.stopPropagation();
    if (event.target.closest("[data-rotate-colors]")) {
      await config.onRotate();
      return;
    }
    const option = event.target.closest("[data-suit-option]");
    if (option) {
      await config.onSelect(Number(option.dataset.suitOption), option.dataset.suitValue);
      return;
    }
    const trigger = event.target.closest("[data-suit-trigger]");
    if (trigger && config.canChange) {
      const index = Number(trigger.dataset.suitTrigger);
      config.onOpen(config.openIndex === index ? -1 : index);
      return;
    }
    config.onOpen(-1);
  };
}
