package automation

// All DOM selectors used by the UI automation helpers live here so a single
// file fixes them whenever Google ships a UI tweak. Verify each against the
// real labs.google DOM via DevTools before relying.
const (
	// PromptEditor matches the Slate.js editable.
	PromptEditor = `[role="textbox"][contenteditable="true"]`

	// CreateButtonText is the visible text on the submit button. There are
	// multiple buttons on the page with the same text — we further filter
	// matches by Y position via the helper.
	CreateButtonText = "Create"
	// CreateButtonMinY is the minimum y-coordinate for the visible bottom-row
	// "Create" button. Anything above this row is a sidebar / hidden duplicate.
	CreateButtonMinY = 680.0

	// SettingsBtnXPath opens the cog dropdown.
	SettingsBtnXPath = `//button[@aria-haspopup="menu" and contains(., "crop_")]`

	// AspectTabXPath / OutputCountTabXPath: tab role with the visible label.
	AspectTabXPath      = `//*[@role="tab" and normalize-space()=$LABEL]`
	OutputCountTabXPath = `//*[@role="tab" and normalize-space()=$LABEL]`

	// ModelMenuItemXPath: menuitem role for nested model picker.
	ModelMenuItemXPath = `//*[@role="menuitem" and contains(., $LABEL)]`
)
