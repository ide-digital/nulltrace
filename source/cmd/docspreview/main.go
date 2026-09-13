// Command docspreview writes docs/media/preview.html from the real TUI views.
package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/4x3/nulltrace/internal/tui"
)

func main() {
	lipgloss.SetColorProfile(termenv.ANSI256)

	root, err := repoRoot()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	home := tui.RenderHome(tui.HomeInfo{
		Identity:  "Ada Lovelace",
		Vault:     tui.OK("unlocked"),
		Mail:      tui.OK("Gmail") + "  " + tui.Name("ada@example.com"),
		EmailHint: "linked  ada@example.com",
		Listings:  24,
		Queued:    8,
		Verified:  3,
		Manual:    5,
		Hint:      "First time?  [5] Identity  →  [1] Scan  →  [14] Leaks  →  [4] Listings",
	})
	login := tui.RenderLoginDemo("WORKSTATION", 8)

	out := filepath.Join(root, "docs", "media", "preview.html")
	if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	page := wrapPage(ansiHTML(home), ansiHTML(padFrame(login, 102, 3)))
	if err := os.WriteFile(out, []byte(page), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("wrote", out)
}

func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "source", "go.mod")); err == nil && filepath.Base(dir) != "source" {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("repo root not found from %s", wd)
		}
		dir = parent
	}
}

func trimBlank(s string) string {
	lines := strings.Split(s, "\n")
	for len(lines) > 0 && strings.TrimSpace(stripANSI(lines[0])) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(stripANSI(lines[len(lines)-1])) == "" {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

// padFrame makes every line the same visible width so the screenshot
// window cannot shrink-wrap around the banner and leave the form off-center.
func padFrame(s string, width, vpad int) string {
	lines := strings.Split(trimBlank(s), "\n")
	blank := strings.Repeat(" ", width)
	out := make([]string, 0, len(lines)+2*vpad)
	for i := 0; i < vpad; i++ {
		out = append(out, blank)
	}
	for _, line := range lines {
		w := lipgloss.Width(stripANSI(line))
		if w < width {
			line += strings.Repeat(" ", width-w)
		}
		out = append(out, line)
	}
	for i := 0; i < vpad; i++ {
		out = append(out, blank)
	}
	return strings.Join(out, "\n")
}

func stripANSI(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) {
				i = j + 1
				continue
			}
		}
		b.WriteByte(s[i])
		i++
	}
	return b.String()
}

func wrapPage(home, login string) string {
	return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<title>NullTrace preview</title>
<style>
  @font-face {
    font-family: "NT Mono";
    src: url("CascadiaMono.ttf") format("truetype");
    font-weight: 400;
    font-style: normal;
    font-display: block;
  }
  html, body { margin: 0; background: #161616; }
  .shot {
    min-height: 100vh;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 40px 28px;
  }
  .win {
    background: #0c0c0c;
    border: 1px solid #2b2b2b;
    border-radius: 6px;
    box-shadow: 0 18px 60px rgba(0,0,0,.5);
    overflow: hidden;
  }
  .chrome {
    height: 32px;
    background: #1a1a1a;
    color: #9a9a9a;
    font: 12px/32px "NT Mono", Consolas, monospace;
    letter-spacing: .14em;
    padding: 0 14px;
    border-bottom: 1px solid #2b2b2b;
  }
  pre.term {
    margin: 0;
    padding: 12px 18px 16px;
    background: #0c0c0c;
    color: #b2b2b2;
    font-family: "NT Mono", Consolas, "Lucida Console", monospace;
    font-size: 14px;
    line-height: 1.05;
    font-weight: 400;
    font-variant-ligatures: none;
    font-kerning: none;
    font-feature-settings: "liga" 0, "calt" 0;
    letter-spacing: 0;
    white-space: pre;
    tab-size: 8;
  }
  pre.term b, pre.term strong { font-weight: 400; }
  #login pre.term { min-width: 102ch; }
</style>
</head>
<body>
<section class="shot" id="home"><div class="win">
  <div class="chrome">NULLTRACE</div>
  <pre class="term">` + home + `</pre>
</div></section>
<section class="shot" id="login"><div class="win">
  <div class="chrome">NULLTRACE</div>
  <pre class="term">` + login + `</pre>
</div></section>
</body>
</html>
`
}

func ansiHTML(s string) string {
	var b strings.Builder
	fg, bg := "", ""
	style := func() string {
		var parts []string
		if fg != "" {
			parts = append(parts, "color:"+fg)
		}
		if bg != "" {
			parts = append(parts, "background:"+bg)
		}
		if len(parts) == 0 {
			return ""
		}
		return ` style="` + strings.Join(parts, ";") + `"`
	}
	open := false
	ensure := func() {
		if open {
			return
		}
		b.WriteString("<span" + style() + ">")
		open = true
	}
	closeSpan := func() {
		if open {
			b.WriteString("</span>")
			open = false
		}
	}
	i := 0
	for i < len(s) {
		if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '[' {
			j := i + 2
			for j < len(s) && (s[j] < 0x40 || s[j] > 0x7e) {
				j++
			}
			if j < len(s) && s[j] == 'm' {
				closeSpan()
				for _, p := range strings.Split(s[i+2:j], ";") {
					if p == "" {
						p = "0"
					}
					n, _ := strconv.Atoi(p)
					switch {
					case n == 0:
						fg, bg = "", ""
					case n == 39:
						fg = ""
					case n == 49:
						bg = ""
					case n == 1, n == 22:
						// Ignore bold — a heavier glyph width shreds the FIGlet banner.
					}
				}
				// Re-parse for 38/48;5;n
				codes := strings.Split(s[i+2:j], ";")
				for k := 0; k < len(codes); k++ {
					c := codes[k]
					if c == "" {
						c = "0"
					}
					n, _ := strconv.Atoi(c)
					switch n {
					case 0:
						fg, bg = "", ""
					case 39:
						fg = ""
					case 49:
						bg = ""
					case 38:
						if k+2 < len(codes) && codes[k+1] == "5" {
							idx, _ := strconv.Atoi(codes[k+2])
							fg = xterm256(idx)
							k += 2
						}
					case 48:
						if k+2 < len(codes) && codes[k+1] == "5" {
							idx, _ := strconv.Atoi(codes[k+2])
							bg = xterm256(idx)
							k += 2
						}
					}
				}
				i = j + 1
				continue
			}
		}
		ensure()
		r, size := utf8.DecodeRuneInString(s[i:])
		switch r {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		default:
			b.WriteString(html.EscapeString(string(r)))
		}
		i += size
	}
	closeSpan()
	return b.String()
}

func xterm256(n int) string {
	if n < 0 {
		n = 0
	}
	if n > 255 {
		n = 255
	}
	if n < 16 {
		c16 := []string{
			"#000000", "#c91b00", "#00c200", "#c7c400",
			"#0037da", "#881798", "#3a96dd", "#cccccc",
			"#767676", "#e74856", "#16c60c", "#f9f1a5",
			"#3b78ff", "#b4009e", "#61d6d6", "#e8e8e8",
		}
		return c16[n]
	}
	if n >= 232 {
		v := 8 + (n-232)*10
		return fmt.Sprintf("#%02x%02x%02x", v, v, v)
	}
	n -= 16
	levels := []int{0, 95, 135, 175, 215, 255}
	r := levels[n/36]
	g := levels[(n%36)/6]
	b := levels[n%6]
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}
