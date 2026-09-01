# Nostalro UI Reference

[nostalro-client](https://github.com/nmeylan/nostalro-client) is a Rust RO
client on wgpu, Apache 2.0, cloned to
`/Users/borisglebov/git/RagnarokClients/nostalro-client`.

**We reference it for the UI layer only.** Its packet code targets the classic
era (versions up to `20120307`); our stack is `PACKETVER 20211103`, so packet
layouts do not transfer. For protocol work, korangar stays the reference — see
[reference notes](KORANGAR_COMPARISON.md).

---

## Why this one, for UI

Both clients made the same structural choice we did: a small immediate-mode
core with no third-party UI toolkit, and a large layer of RO windows on top.
Their core is a custom framework the README describes as inspired by egui;
ours is `internal/engine/ui2d`, native since [#95](https://github.com/avatar29A/midgard-ro/pull/95)
retired the ImGui widgets.

That makes it the closest structural match we have. Korangar's UI is a
different shape and answers fewer of the questions we are actually hitting.

## The split is the same; the gap is all in the windows

| Layer | Midgard | Nostalro |
|---|---|---|
| Immediate-mode core | `internal/engine/ui2d` — 4,445 LOC | `lib/ui-core` — 3,770 LOC |
| RO windows | `internal/game/ui` — 7,831 LOC, ~26 files | `lib/ui-component` — 35,368 LOC, ~64 files |

The cores are within striking distance of each other in size, which suggests
we are not missing a foundational piece. The windows layer is roughly 4.5x
ours. That is where the reference value is: not "how do we build a UI
framework", but "how does *this window* behave".

## What maps onto what

| Midgard | Nostalro | Notes |
|---|---|---|
| `hud_basic_info.go` | `game/basic_info_window.rs` | |
| `win_items.go` | `game/inventory_window.rs` | |
| `win_skills.go` | `game/skill_tree_window.rs` | |
| `win_map.go` | `game/minimap_window.rs` | |
| `hud_chat.go` | `game/chat_window.rs` | |
| `hud_hotkeys.go` | `game/hotkey_bar.rs` | plus `hotkey_config_window.rs` |
| `hud_sound.go` | `game/sound_options.rs` | plus `graphic_options.rs` |
| `npcdialog.go`, `npcmenu.go`, `npctext.go` | `game/npc_dialog.rs`, `game/npc_shop.rs` | |
| `charselect_native.go` | `account/char_select_window.rs` | plus `char_create_window.rs` |
| `scrollbar.go` | `helper/scrollbar.rs` | |
| `window_skin.go`, `ui2d/windowframe.go` | `helper/window_chrome.rs` | |

## Windows they have that we do not

Roughly in the order we are likely to want them: `equipment_window`,
`item_info_window`, `context_menu`, `confirm_dialog`, `input_dialog`,
`drop_quantity_dialog`, `item_pickup_notification`, `levelup_notification_window`,
`monster_info_window`, `emotion_window`, `quest_window`, `party_friends_window`,
`guild_window`, `mailbox_window`, `my_shop_window` (vending), `card_insert_dialog`,
`make_item_window`, `cart_window`, `book_window`, and the companion set
(`pet_window`, `homun_window`, `mercenary_window` and their skill windows).

## Specific things worth reading before we build the equivalent

- **`ui-core/frame.rs` — `DragState` / `DragCancelledInfo`.** Drag and drop
  with a cancel path. We need this the moment inventory items can go to the
  hotkey bar, and cancel-mid-drag is the part that is easy to get wrong.
- **`ui-core/draw.rs` — `parse_color_codes` and `colored_word_wrap`.** RO chat
  colour codes wrapped across lines. Our chat does not do this yet, and
  wrapping *coloured spans* is meaningfully harder than wrapping text.
- **`ui-core/theme.rs` — the fallback palette.** `fallback_button`,
  `fallback_panel`, `fallback_glossy_panel`, `bevel`: widgets drawn
  procedurally when the skin assets are missing. We go straight to the GRF
  via `window_skin.go` and have no answer for an absent asset.
- **`ui-component/widget_id.rs` — `WidgetId(u32)`.** Stable widget identity,
  which is the classic immediate-mode failure mode once windows can be
  reordered or closed.
- **`ui-core/test_support.rs`**, behind a `test-support` feature. Headless UI
  testing. We already test widgets (`hud_basic_info_test.go`,
  `hud_window_state_test.go`); worth seeing what they made assertable.
- **`ui-component/examples/hot_reload.rs`.** Their UI component viewer reloads
  without a restart. Our loop is a full client relaunch.
- **`helper/dialog_container.rs`, `helper/dropdown.rs`, `helper/head_board.rs`.**
  Shared chrome we will otherwise reinvent per window.

## Licence

Apache 2.0. Reading it and following its design is unencumbered. If we ever
adapt code from it rather than re-derive an approach, that carries attribution
and NOTICE obligations — and this repo currently has no `LICENSE` file of its
own, which should be settled first.
