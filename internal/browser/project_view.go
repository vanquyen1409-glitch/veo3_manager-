package browser

import (
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/go-rod/rod"
)

// EnsureProjectView is the public wrapper of ensureProjectView so other
// services (queue worker) can restore the Flow UI after submits/edits.
func (s *Service) EnsureProjectView() error {
	s.mu.RLock()
	page := s.page
	s.mu.RUnlock()
	if page == nil {
		return ErrNotConnected
	}
	return s.ensureProjectView(page)
}

// ensureProjectView normalises the page URL so the user-facing Flow UI keeps
// its prompt input + sidebar:
//
//   - /tools/flow root             → pick the first project tile, navigate.
//   - /tools/flow/project/<id>/edit/<wf>  → strip /edit/<wf>, stay on root project.
//   - /tools/flow/project/<id>     → keep.
//
// If the page is on the project list and no projects exist yet, click the
// "Dự án mới" tile to create one.
func (s *Service) ensureProjectView(page *rod.Page) error {
	info, err := page.Info()
	if err != nil || info == nil {
		return err
	}
	currentURL := info.URL

	if hasProject(currentURL) && !hasEditMode(currentURL) {
		return nil
	}

	if hasProject(currentURL) && hasEditMode(currentURL) {
		root := stripEditMode(currentURL)
		slog.Info("ensureProjectView: trim edit mode", "from", currentURL, "to", root)
		if err := page.Navigate(root); err != nil {
			return err
		}
		_ = page.WaitLoad()
		return nil
	}

	// On the project list (or anywhere else): retry the search up to 5
	// times with 600ms gaps because Flow's grid renders client-side and
	// can be empty on first paint right after navigation.
	for attempt := 1; attempt <= 5; attempt++ {
		if href, ok := s.findFirstProjectHref(page); ok {
			slog.Info("ensureProjectView: navigating to existing project", "attempt", attempt, "href", href)
			if err := page.Navigate(href); err != nil {
				return err
			}
			_ = page.WaitLoad()
			return nil
		}
		if s.clickNewProjectTile(page) {
			slog.Info("ensureProjectView: clicked Dự án mới", "attempt", attempt)
			_ = page.WaitLoad()
			time.Sleep(600 * time.Millisecond)
			return nil
		}
		time.Sleep(600 * time.Millisecond)
	}
	return errors.New("project tile / 'Dự án mới' button not visible after retries — page DOM not ready")
}

func (s *Service) findFirstProjectHref(page *rod.Page) (string, bool) {
	res, err := page.Eval(`() => {
		const a = document.querySelector('a[href*="/tools/flow/project/"]');
		return a ? a.href : '';
	}`)
	if err != nil || res == nil {
		return "", false
	}
	var href string
	_ = res.Value.Unmarshal(&href)
	if href == "" {
		return "", false
	}
	return stripEditMode(href), true
}

func (s *Service) clickNewProjectTile(page *rod.Page) bool {
	res, err := page.Eval(`() => {
		const cands = Array.from(document.querySelectorAll('button, [role="button"], a, div'));
		const m = cands.find((el) => /Dự án mới|New project|Tạo dự án/i.test(el.innerText || ''));
		if (m) { m.click(); return true; }
		return false;
	}`)
	if err != nil || res == nil {
		return false
	}
	var ok bool
	_ = res.Value.Unmarshal(&ok)
	return ok
}

func hasProject(rawURL string) bool {
	return strings.Contains(rawURL, "/tools/flow/project/")
}

func hasEditMode(rawURL string) bool {
	return strings.Contains(rawURL, "/edit/")
}

// stripEditMode returns the project root URL by removing the trailing
// /edit/<workflow-id> segment. No-op when not in edit mode.
func stripEditMode(rawURL string) string {
	idx := strings.Index(rawURL, "/edit/")
	if idx < 0 {
		return rawURL
	}
	return rawURL[:idx]
}
