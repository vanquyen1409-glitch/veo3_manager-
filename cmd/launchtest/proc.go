package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func chromesUsingProfile(profile string) int {
	// Match by the leaf folder name (e.g. "chromedata") to avoid backslash
	// escaping issues; that's still unique enough since launcher only adds
	// the user-data-dir flag for our profile.
	leaf := filepath.Base(profile)
	cmd := fmt.Sprintf(
		`(Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" | Where-Object { $_.CommandLine -like "*%s*" } | Measure-Object).Count`,
		leaf,
	)
	out, err := exec.Command("powershell", "-NoProfile", "-Command", cmd).Output()
	if err != nil {
		return -1
	}
	n, _ := strconv.Atoi(strings.TrimSpace(string(out)))
	return n
}

func killProfileChromes(profile string) {
	leaf := filepath.Base(profile)
	cmd := fmt.Sprintf(
		`Get-CimInstance Win32_Process -Filter "Name='chrome.exe'" | Where-Object { $_.CommandLine -like "*%s*" } | ForEach-Object { Stop-Process -Id $_.ProcessId -Force -ErrorAction SilentlyContinue }`,
		leaf,
	)
	_ = exec.Command("powershell", "-NoProfile", "-Command", cmd).Run()
}
