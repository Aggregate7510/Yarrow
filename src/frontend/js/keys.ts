// Desktop keyboard shortcuts. Ignored while typing in a field.

import {
  state,
  isSplit,
  writeLocal,
  openReader,
  closeReader,
  closeMenu,
  openNeighbor,
  toggleRead,
  toggleSaved,
  openExternal,
  setUnreadOnly,
  refreshFeeds,
  loadItems,
  loadFullText,
} from "./store";

function focusCard(index: number) {
  if (!state.items.length) return;
  const i = Math.max(0, Math.min(state.items.length - 1, index));
  state.focusedIndex = i;
  const el = document.querySelector(`.card[data-id="${state.items[i].id}"]`);
  el?.scrollIntoView({ block: "center", behavior: "smooth" });
  if (i >= state.items.length - 3) loadItems();
}

function current() {
  if (state.reader.open) return state.reader.item;
  return state.items[state.focusedIndex] ?? null;
}

export function setupKeys(openSearch: () => void) {
  document.addEventListener("keydown", e => {
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    const t = e.target as HTMLElement;
    if (t && (t.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(t.tagName))) return;

    if (e.key === "Escape") {
      if (state.menu) closeMenu();
      else if (state.sheet) state.sheet = null;
      else if (state.reader.open) closeReader();
      else return;
      e.preventDefault();
      return;
    }
    if (state.menu || state.sheet) return;

    const item = current();
    const split = isSplit.value;
    switch (e.key) {
      case "j":
      case "ArrowRight":
        if (state.reader.open) openNeighbor(1);
        else if (e.key === "j") {
          focusCard(state.focusedIndex + 1);
          // with the reading pane, moving through the list reads as you go
          if (split && state.items[state.focusedIndex]) openReader(state.items[state.focusedIndex].id);
        } else return;
        break;
      case "k":
      case "ArrowLeft":
        if (state.reader.open) openNeighbor(-1);
        else if (e.key === "k") {
          focusCard(state.focusedIndex - 1);
          if (split && state.items[state.focusedIndex]) openReader(state.items[state.focusedIndex].id);
        } else return;
        break;
      case " ": {
        // space pages through the article in the reading pane, like Mail
        const pane = document.querySelector<HTMLElement>(".reader");
        if (!split || !state.reader.open || !pane) return;
        pane.scrollBy({ top: (e.shiftKey ? -1 : 1) * pane.clientHeight * 0.85, behavior: "smooth" });
        break;
      }
      case "\\":
        state.sidebarCollapsed = !state.sidebarCollapsed;
        writeLocal("yarrow.sidebarCollapsed", state.sidebarCollapsed ? "1" : "0");
        break;
      case "o":
      case "Enter":
        if (!state.reader.open && item) openReader(item.id);
        else return;
        break;
      case "m":
        if (item) toggleRead(item);
        break;
      case "s":
        if (item) toggleSaved(item);
        break;
      case "v":
        if (item?.link) openExternal(item.link);
        break;
      case "f":
        if (state.reader.open) loadFullText();
        break;
      case "u":
        if (state.view.type !== "saved") setUnreadOnly(!state.unreadOnly);
        break;
      case "r":
        if (state.refreshAvailable && !state.refreshing) refreshFeeds();
        break;
      case "/":
        if (!state.reader.open) openSearch();
        break;
      case "?":
        state.sheet = "shortcuts";
        break;
      default:
        return;
    }
    e.preventDefault();
  });
}

export const shortcutList: [string, string][] = [
  ["j / k", "Next / previous article"],
  ["right-click", "Article actions"],
  ["o or Enter", "Open article"],
  ["Esc", "Close article or menu"],
  ["space", "Scroll the article (reading pane)"],
  ["\\", "Show or hide the sidebar"],
  ["m", "Mark read / unread"],
  ["s", "Save for later / remove"],
  ["v", "Open original website"],
  ["f", "Load full article (in reader)"],
  ["u", "Toggle unread only"],
  ["r", "Refresh feeds"],
  ["/", "Search"],
  ["?", "Show this list"],
];
