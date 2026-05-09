package automation

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// Helper drives the labs.google UI when the API path needs a fallback or
// when settings must be sync'd in-page.
type Helper struct{}

// New returns a fresh helper. Stateless — safe to share across goroutines.
func New() *Helper { return &Helper{} }

// FillPrompt clears the Slate.js editor and inserts text via CDP InsertText.
// Slate.js intercepts keydown events, so type-emulation fails — InsertText
// dispatches a native input event the React tree will pick up.
func (h *Helper) FillPrompt(p *rod.Page, text string) error {
	el, err := p.Element(PromptEditor)
	if err != nil {
		return ErrPromptEditorMissing
	}
	if err := el.Focus(); err != nil {
		return fmt.Errorf("focus editor: %w", err)
	}
	// Select-all + delete to clear existing content.
	if err := el.SelectAllText(); err != nil {
		return fmt.Errorf("select all: %w", err)
	}
	keyDown := proto.InputDispatchKeyEvent{
		Type: proto.InputDispatchKeyEventTypeRawKeyDown,
		Key:  "Delete",
	}
	if err := keyDown.Call(p); err != nil {
		return fmt.Errorf("clear: %w", err)
	}
	keyUp := proto.InputDispatchKeyEvent{
		Type: proto.InputDispatchKeyEventTypeKeyUp,
		Key:  "Delete",
	}
	if err := keyUp.Call(p); err != nil {
		return fmt.Errorf("clear keyup: %w", err)
	}

	// CDP InsertText: bypasses Slate's onKeyDown via native input/beforeinput.
	if err := (proto.InputInsertText{Text: text}).Call(p); err != nil {
		return fmt.Errorf("insert text: %w", err)
	}
	return nil
}

// ClickCreate clicks the visible bottom-row "Create" button. Filters by Y
// position because the page renders multiple buttons with the same label.
func (h *Helper) ClickCreate(p *rod.Page) error {
	// Some pages embed the label inside spans; query all buttons and read inner text.
	btns, err := p.Elements("button")
	if err != nil {
		return err
	}
	for _, b := range btns {
		txt, _ := b.Text()
		if !strings.Contains(strings.TrimSpace(txt), CreateButtonText) {
			continue
		}
		shape, err := b.Shape()
		if err != nil || shape == nil || len(shape.Quads) == 0 {
			continue
		}
		box := shape.Box()
		if box == nil {
			continue
		}
		if box.Y >= CreateButtonMinY {
			return b.Click(proto.InputMouseButtonLeft, 1)
		}
	}
	return ErrCreateButtonNotFound
}

// openSettings clicks the cog button and waits briefly for the menu to mount.
func (h *Helper) openSettings(p *rod.Page) error {
	btn, err := p.ElementX(SettingsBtnXPath)
	if err != nil {
		return ErrSettingsButtonMissing
	}
	if err := btn.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return err
	}
	// Settle animations.
	time.Sleep(200 * time.Millisecond)
	return nil
}

// SetAspectRatio selects the aspect-ratio tab matching ratio. Idempotent —
// returns nil if the tab already shows data-state="active".
func (h *Helper) SetAspectRatio(p *rod.Page, ratio string) error {
	if err := h.openSettings(p); err != nil {
		return err
	}
	xpath := strings.Replace(AspectTabXPath, "$LABEL", `"`+ratio+`"`, 1)
	tab, err := p.ElementX(xpath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrTabNotFound, ratio)
	}
	state, _ := tab.Attribute("data-state")
	if state != nil && *state == "active" {
		return nil
	}
	return tab.Click(proto.InputMouseButtonLeft, 1)
}

// SetOutputCount selects the count tab (1..4).
func (h *Helper) SetOutputCount(p *rod.Page, count int) error {
	if count < 1 || count > 4 {
		return fmt.Errorf("count out of range: %d", count)
	}
	if err := h.openSettings(p); err != nil {
		return err
	}
	xpath := strings.Replace(OutputCountTabXPath, "$LABEL", `"`+strconv.Itoa(count)+`"`, 1)
	tab, err := p.ElementX(xpath)
	if err != nil {
		return fmt.Errorf("%w: count=%d", ErrTabNotFound, count)
	}
	state, _ := tab.Attribute("data-state")
	if state != nil && *state == "active" {
		return nil
	}
	return tab.Click(proto.InputMouseButtonLeft, 1)
}

// SetModel picks a menuitem in the model sub-dropdown.
func (h *Helper) SetModel(p *rod.Page, model string) error {
	if err := h.openSettings(p); err != nil {
		return err
	}
	xpath := strings.Replace(ModelMenuItemXPath, "$LABEL", `"`+model+`"`, 1)
	item, err := p.ElementX(xpath)
	if err != nil {
		return fmt.Errorf("%w: %s", ErrModelMenuItemMissing, model)
	}
	return item.Click(proto.InputMouseButtonLeft, 1)
}
