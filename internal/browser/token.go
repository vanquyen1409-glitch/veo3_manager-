package browser

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
)

// tokenProbeJS is a function expression that rod calls via Input.Eval. It
// probes several known locations for the Google Labs access token and logs
// which keys are present in pageProps when the token is missing.
const tokenProbeJS = `
() => {
  try {
    const el = document.getElementById('__NEXT_DATA__');
    if (!el) return { ok: false, reason: 'no_next_data' };
    const data = JSON.parse(el.textContent);
    const pp = (data && data.props && data.props.pageProps) || {};
    const candidates = [
      pp && pp.session && pp.session.access_token,
      pp && pp.session && pp.session.accessToken,
      pp && pp.token,
      pp && pp.user && pp.user.accessToken,
      pp && pp.account && pp.account.accessToken,
    ];
    const tok = candidates.find((t) => typeof t === 'string' && t.length > 20);
    if (tok) return { ok: true, token: tok };
    return { ok: false, reason: 'token_field_missing', keys: Object.keys(pp) };
  } catch (e) { return { ok: false, reason: String(e) }; }
}
`

type tokenResult struct {
	OK     bool     `json:"ok"`
	Token  string   `json:"token,omitempty"`
	Reason string   `json:"reason,omitempty"`
	Keys   []string `json:"keys,omitempty"`
}

// extractToken evaluates tokenProbeJS on the given page. Returns the access
// token string or a typed error.
func extractToken(p *rod.Page) (string, error) {
	res, err := p.Eval(tokenProbeJS)
	if err != nil {
		return "", fmt.Errorf("eval token probe: %w", err)
	}
	var out tokenResult
	if err := res.Value.Unmarshal(&out); err != nil {
		return "", fmt.Errorf("decode token result: %w", err)
	}
	if !out.OK {
		if out.Reason == "no_next_data" {
			return "", ErrLabsTabUnavailable
		}
		// Surface keys we found so debug card can show them.
		if len(out.Keys) > 0 {
			return "", fmt.Errorf("%w (keys: %s)", ErrTokenMissing, strings.Join(out.Keys, ","))
		}
		return "", fmt.Errorf("%w: %s", ErrTokenMissing, out.Reason)
	}
	return out.Token, nil
}
