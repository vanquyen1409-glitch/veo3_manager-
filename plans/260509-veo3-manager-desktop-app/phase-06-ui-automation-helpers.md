# Phase 06 — UI Automation Helpers (Slate.js + Settings Dropdowns)

## Context
- Parent: [plan.md](plan.md)
- Depends on: [Phase 03](phase-03-browser-service.md)

## Overview
- **Date:** 2026-05-09
- **Description:** Thin helpers that drive the labs.google Flow UI when needed: filling the Slate.js prompt editor, clicking the "Create" submit button (with the `y > 680` filter), and selecting aspect ratio / output count / model from the settings dropdowns. Used as a **fallback** when the API path fails, and for one-time in-page sync (e.g. ensuring page settings match user's stored config so the API request inherits the right state).
- **Priority:** Should-have — API path is primary; UI fallback raises reliability.
- **Implementation status:** Pending
- **Review status:** Pending

## Key insights
- Slate.js intercepts `keydown`; only `Input.insertText` (CDP) lands text in the editor.
- Page renders multiple buttons with the same label — `y > 680` distinguishes the visible bottom-row "Create" from sidebar entries.
- Dropdowns: open by clicking a `button[aria-haspopup="menu"]` whose text contains `crop_` (a Material Symbols ligature). After open, items differ by selector:
  - **Aspect ratio** + **output count**: `[role="tab"][data-state="inactive|active"]`. Active state mutates after click.
  - **Model**: `[role="menuitem"]`, may live in a nested submenu.
- Always wait for animations (use `page.WaitStable(300ms)`).

## Requirements
1. `automation.Helper` exposes:
   - `FillPrompt(ctx, page *rod.Page, text string) error`
   - `ClickCreate(ctx, page *rod.Page) error`
   - `SetAspectRatio(ctx, page, ratio string) error` (`"16:9" | "9:16" | "1:1"`)
   - `SetOutputCount(ctx, page, count int) error` (1–4)
   - `SetModel(ctx, page, model string) error`
2. Idempotent — querying current `data-state="active"` skips the click if already correct.
3. Each helper logs which selector matched + time taken; raises typed errors on miss.
4. All selectors centralized in a `selectors.go` package-level var so a single file fixes them when Google ships a UI tweak.

## Architecture
```
internal/automation/
├── helper.go        // Helper struct, public methods
├── prompt.go        // FillPrompt + ClickCreate
├── settings.go      // SetAspectRatio, SetOutputCount, SetModel
├── selectors.go     // Centralized CSS / XPath strings
└── errors.go
```

### Selectors (initial — verify in real DOM)
```go
const (
    PromptEditor       = `[role="textbox"][contenteditable="true"]`
    SettingsBtnXPath   = `//button[@aria-haspopup="menu" and contains(., "crop_")]`
    AspectTabXPath     = `//*[@role="tab" and normalize-space()=$ratio]`
    OutputCountTabXPath= `//*[@role="tab" and normalize-space()=string($count)]`
    ModelMenuItemXPath = `//*[@role="menuitem" and contains(., $model)]`
)
```

### FillPrompt (CDP insertText)
```go
func (h *Helper) FillPrompt(p *rod.Page, text string) error {
    el, err := p.Element(PromptEditor)
    if err != nil { return err }
    if err := el.Focus(); err != nil { return err }
    // Clear existing
    _ = el.SelectAllText()
    _ = proto.InputDispatchKeyEvent{Type: proto.InputDispatchKeyEventTypeRawKeyDown, WindowsVirtualKeyCode: 46}.Call(p) // Delete
    return proto.InputInsertText{Text: text}.Call(p)
}
```

### ClickCreate (with y filter)
```go
func (h *Helper) ClickCreate(p *rod.Page) error {
    btns, err := p.Elements(`button:has-text("Create")`)
    if err != nil { return err }
    for _, b := range btns {
        box, err := b.Shape()
        if err != nil { continue }
        if box.Box().Y > 680 {
            return b.Click(proto.InputMouseButtonLeft, 1)
        }
    }
    return ErrCreateButtonNotFound
}
```

### Open-settings + select tab
```go
func (h *Helper) openSettings(p *rod.Page) (*rod.Element, error) {
    btn, err := p.ElementX(SettingsBtnXPath)
    if err != nil { return nil, err }
    if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil { return nil, err }
    p.MustWaitStable()
    return btn, nil
}

func (h *Helper) SetAspectRatio(p *rod.Page, ratio string) error {
    if _, err := h.openSettings(p); err != nil { return err }
    tab, err := p.ElementX(strings.NewReplacer("$ratio", ratio).Replace(AspectTabXPath))
    if err != nil { return err }
    state, _ := tab.Attribute("data-state")
    if state != nil && *state == "active" { return nil }
    return tab.Click(proto.InputMouseButtonLeft, 1)
}
```

## Related code files
- `internal/automation/*`
- `internal/queue/runner.go` (Phase 08) — optional fallback path.

## Implementation steps
1. Stub the package + types; selectors centralized.
2. Implement `FillPrompt` first; iterate against a real labs.google page.
3. Implement `ClickCreate` with y-filter.
4. Implement settings helpers; reuse `openSettings`.
5. Add `SetModel` last (nested menu — may need extra wait for submenu mount).
6. Document the manual probing flow in a brief `internal/automation/README.md`: open DevTools → inspect DOM → tweak selectors → re-run.

## Todo list
- [ ] Centralize selectors.
- [ ] Implement `FillPrompt` via CDP insertText.
- [ ] Implement `ClickCreate` with y > 680 filter.
- [ ] Implement aspect/output/model setters; idempotent.
- [ ] Manual smoke against real page.

## Success criteria
- `FillPrompt` reliably injects 100+ char prompts even with line breaks (`\n`).
- `ClickCreate` always picks the correct visible button.
- `SetAspectRatio("9:16")` flips active tab without re-opening menu unnecessarily.

## Risk assessment
- **DOM changes** — keep all selectors in one file; document the rebuild flow.
- **Race with React re-render** — use `WaitStable`/`WaitElementsStable` after mutations.
- **Unicode prompts** — `Input.insertText` handles UTF-8 fine; test with Vietnamese characters.

## Security considerations
- No new attack surface; we only manipulate the user's own browser session.

## Next steps
Phase 07 — Video Download Pipeline.
