// Command simulator stands in for a GPS tracker device: it walks a fake track
// around Jakarta and reports each fix to an OsmAnd-protocol HTTP endpoint
// (Traccar-compatible query parameters).
//
//	go run .            # interactive TUI
//	go run . -selftest  # logic checks, no network, no TUI
//
// Keys act immediately — no Enter — because the terminal is put in raw mode.
// Text fields (IMEI, endpoint, custom interval) are typed and submitted with
// Enter. When stdin is not a terminal, input falls back to whole lines so the
// simulator stays scriptable.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"golang.org/x/term"
)

const (
	defaultIMEI     = "357565514527184"
	defaultEndpoint = "http://localhost:5055"
	defaultInterval = time.Second
	logLines        = 12

	seedLat = -6.200000 // Jakarta
	seedLon = 106.816666
	// boxDeg is the half-width of the square the walk is confined to (~1 km).
	boxDeg       = 0.009
	baseSpeedMPS = 8.33 // ~30 km/h
	knotsPerMPS  = 1.943844
	clearScreen  = "\x1b[2J\x1b[H"
)

var intervalPresets = []time.Duration{
	time.Second, 5 * time.Second, 10 * time.Second, 30 * time.Second, time.Minute,
}

type config struct {
	imei     string
	interval time.Duration
	endpoint string
}

// position is the simulated device's internal state. Distances are metres and
// speed is m/s; the wire format converts.
type position struct {
	lat, lon float64
	heading  float64 // degrees
	speedMPS float64
	altitude float64
	accuracy float64
	hdop     float64
	batt     float64
}

func newPosition() *position {
	return &position{
		lat: seedLat, lon: seedLon,
		heading:  rand.Float64() * 360,
		speedMPS: baseSpeedMPS,
		altitude: 12, accuracy: 5, hdop: 0.8, batt: 87,
	}
}

// step advances the walk by one report interval.
func (p *position) step(d time.Duration) {
	p.heading = math.Mod(p.heading+rand.Float64()*30-15+360, 360)
	p.speedMPS = baseSpeedMPS * (0.8 + rand.Float64()*0.4)
	dist := p.speedMPS * d.Seconds()
	rad := p.heading * math.Pi / 180
	p.lat += dist * math.Cos(rad) / 111320
	p.lon += dist * math.Sin(rad) / (111320 * math.Cos(p.lat*math.Pi/180))
	if math.Abs(p.lat-seedLat) > boxDeg || math.Abs(p.lon-seedLon) > boxDeg {
		p.lat = math.Min(math.Max(p.lat, seedLat-boxDeg), seedLat+boxDeg)
		p.lon = math.Min(math.Max(p.lon, seedLon-boxDeg), seedLon+boxDeg)
		p.heading = math.Mod(p.heading+180, 360)
	}
}

// target builds the OsmAnd GET URL for one report.
func (c config) target(p *position, now time.Time) string {
	q := url.Values{}
	q.Set("id", c.imei)
	q.Set("valid", "1")
	q.Set("timestamp", strconv.FormatInt(now.Unix(), 10)) // epoch seconds
	q.Set("lat", strconv.FormatFloat(p.lat, 'f', 6, 64))
	q.Set("lon", strconv.FormatFloat(p.lon, 'f', 6, 64))
	q.Set("speed", strconv.FormatFloat(p.speedMPS*knotsPerMPS, 'f', 2, 64)) // knots
	q.Set("bearing", strconv.FormatFloat(p.heading, 'f', 1, 64))
	q.Set("altitude", strconv.FormatFloat(p.altitude, 'f', 1, 64))
	q.Set("accuracy", strconv.FormatFloat(p.accuracy, 'f', 1, 64))
	q.Set("hdop", strconv.FormatFloat(p.hdop, 'f', 2, 64))
	q.Set("batt", strconv.FormatFloat(p.batt, 'f', 0, 64))
	q.Set("charge", "false")
	u, err := url.Parse(c.endpoint)
	if err != nil {
		return c.endpoint
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// send reports one fix. A non-nil error means the run must stop: either the
// request never reached the server, or the server did not answer 200.
func send(client *http.Client, c config, p *position) error {
	resp, err := client.Get(c.target(p, time.Now()))
	if err != nil {
		// net/http wraps the failure as `Get "http://host?every&param=…": reason`.
		// The URL is already on screen; only the reason is worth reporting.
		var uerr *url.Error
		if errors.As(err, &uerr) && uerr.Err != nil {
			return uerr.Err
		}
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("server returned %s", resp.Status)
	}
	return nil
}

func normalizeEndpoint(s string) (string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", fmt.Errorf("endpoint is empty")
	}
	if !strings.Contains(s, "://") {
		s = "http://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("endpoint %q has no host", s)
	}
	return s, nil
}

// parseInterval accepts whole seconds only: the field is a plain run of digits,
// so no sign, no decimal fraction and no millisecond value.
func parseInterval(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	secs, err := strconv.Atoi(s)
	if err != nil || s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, fmt.Errorf("%q is not a whole number of seconds", s)
	}
	if secs < 1 || secs > 3600 {
		return 0, fmt.Errorf("interval must be 1-3600 seconds")
	}
	return time.Duration(secs) * time.Second, nil
}

func validIMEI(s string) bool {
	if len(s) != 15 {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// out is where the UI writes. Raw mode switches off the terminal's own newline
// translation, so a bare "\n" would only move the cursor down and leave it at
// the end of the previous line, staircasing the screen to the right. newInput
// swaps in crlfWriter to put the carriage returns back.
var out io.Writer = os.Stdout

type crlfWriter struct{ w io.Writer }

func (c crlfWriter) Write(p []byte) (int, error) {
	buf := make([]byte, 0, len(p)+8)
	for i, b := range p {
		if b == '\n' && (i == 0 || p[i-1] != '\r') {
			buf = append(buf, '\r')
		}
		buf = append(buf, b)
	}
	if _, err := c.w.Write(buf); err != nil {
		return 0, err
	}
	return len(p), nil
}

// The screen is redrawn by clearing it, so it must never draw more rows than
// the window has: an overfull screen scrolls and the top of the menu scrolls
// away. Lines reaching the last column wrap and smear the layout, so they are
// cut short instead. Both sizes are read from the terminal at startup.
var (
	uiWidth  = 100
	uiHeight = 30
)

// fit cuts one line to the terminal width, leaving the last column free.
func fit(s string, width int) string {
	limit := width - 1
	if limit < 8 {
		return s
	}
	r := []rune(s)
	if len(r) <= limit {
		return s
	}
	return string(r[:limit-1]) + "…"
}

func fitLines(body string, width int) string {
	lines := strings.Split(body, "\n")
	for i, l := range lines {
		lines[i] = fit(l, width)
	}
	return strings.Join(lines, "\n")
}

// logCapacity is how many log rows fit under the run screen's other furniture.
func logCapacity() int {
	n := uiHeight - 9
	if n < 3 {
		n = 3
	}
	if n > logLines {
		n = logLines
	}
	return n
}

// input delivers key presses. On a terminal, stdin is put in raw mode so one
// key acts at once; off a terminal (piped input) it reads whole lines instead.
type input struct {
	tty  bool
	keys chan string
	old  *term.State
	fd   int
}

func newInput() *input {
	in := &input{fd: int(os.Stdin.Fd()), keys: make(chan string, 16)}
	if term.IsTerminal(in.fd) {
		if old, err := term.MakeRaw(in.fd); err == nil {
			in.tty, in.old = true, old
			out = crlfWriter{os.Stdout}
			if w, h, err := term.GetSize(in.fd); err == nil && w > 0 && h > 0 {
				uiWidth, uiHeight = w, h
			}
			go in.readKeys()
			return in
		}
	}
	go in.readLines()
	return in
}

func (in *input) readKeys() {
	defer close(in.keys)
	buf := make([]byte, 1)
	for {
		n, err := os.Stdin.Read(buf)
		if n > 0 {
			in.keys <- string(buf[:n])
		}
		if err != nil {
			return
		}
	}
}

func (in *input) readLines() {
	defer close(in.keys)
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		in.keys <- strings.TrimSpace(sc.Text())
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "reading stdin: %v\n", err)
	}
}

// key waits for one key press, or for one whole line when not on a terminal.
func (in *input) key() (string, bool) {
	k, ok := <-in.keys
	return k, ok
}

// line reads one line of text, echoing it. Raw mode turns off the kernel's line
// editing, so the editing is done here.
func (in *input) line() (string, bool) {
	if !in.tty {
		return in.key()
	}
	var b strings.Builder
	for {
		k, ok := <-in.keys
		if !ok {
			return "", false
		}
		switch k {
		case "\r", "\n":
			fmt.Fprint(out, "\r\n")
			return b.String(), true
		case "\x03", "\x04": // Ctrl-C, Ctrl-D
			return "", false
		case "\x7f", "\b": // Backspace
			if s := b.String(); s != "" {
				b.Reset()
				b.WriteString(s[:len(s)-1])
				fmt.Fprint(out, "\b \b")
			}
		default:
			if k >= " " && k <= "~" {
				b.WriteString(k)
				fmt.Fprint(out, k)
			}
		}
	}
}

// close restores the terminal. Safe to call twice.
func (in *input) close() {
	if in.tty {
		_ = term.Restore(in.fd, in.old)
		in.tty = false
	}
}

// printable reports whether a key is worth complaining about; escape sequences
// from arrow keys are not.
func printable(k string) bool {
	return len(k) > 1 || (k >= " " && k <= "~")
}

func presetList(current time.Duration) string {
	parts := make([]string, len(intervalPresets))
	for i, d := range intervalPresets {
		s := d.String()
		if d == current {
			s = "[" + s + "]"
		}
		parts[i] = s
	}
	return strings.Join(parts, " ")
}

func screen(c config, status string) string {
	var b strings.Builder
	b.WriteString("\n  OsmAnd GPS device simulator\n")
	b.WriteString("  ────────────────────────────────────────────────────────\n\n")
	fmt.Fprintf(&b, "  [1] IMEI      : %s\n", c.imei)
	fmt.Fprintf(&b, "  [2] Interval  : %s\n", c.interval)
	fmt.Fprintf(&b, "                  %s\n", presetList(c.interval))
	fmt.Fprintf(&b, "  [3] Endpoint  : %s\n", c.endpoint)
	b.WriteString("  [4] Start\n\n")
	fmt.Fprintf(&b, "  %s\n", status)
	b.WriteString("\n  [2] cycles presets   [c] custom seconds   [4] start   [q] quit\n")
	b.WriteString("  keys act at once\n")
	return b.String()
}

type menuResult struct {
	cfg    config
	quit   bool
	start  bool
	status string
}

func menuIMEI(res menuResult, c config, in *input) menuResult {
	fmt.Fprintf(out, "\n  IMEI (15 digits) [%s]: ", c.imei)
	v, ok := in.line()
	if !ok {
		res.quit = true
	} else if v == "" {
		res.status = "IMEI unchanged"
	} else if !validIMEI(v) {
		res.status = "rejected: IMEI must be exactly 15 digits"
	} else {
		res.cfg.imei = v
		res.status = "IMEI = " + v
	}
	return res
}

func menuInterval(res menuResult, c config) menuResult {
	next := 0
	for i, d := range intervalPresets {
		if d == c.interval {
			next = (i + 1) % len(intervalPresets)
			break
		}
	}
	res.cfg.interval = intervalPresets[next]
	res.status = "interval = " + res.cfg.interval.String() + "  (press [c] to type a custom value)"
	return res
}

func menuCustomInterval(res menuResult, in *input) menuResult {
	fmt.Fprint(out, "\n  Interval in whole seconds (1-3600): ")
	v, ok := in.line()
	if !ok {
		res.quit = true
		return res
	}
	d, err := parseInterval(v)
	if err != nil {
		res.status = "rejected: " + err.Error()
		return res
	}
	res.cfg.interval = d
	res.status = "interval = " + d.String()
	return res
}

func menuEndpoint(res menuResult, c config, in *input) menuResult {
	fmt.Fprintf(out, "\n  Endpoint [%s]: ", c.endpoint)
	v, ok := in.line()
	if !ok {
		res.quit = true
	} else if v == "" {
		res.status = "endpoint unchanged"
	} else if e, err := normalizeEndpoint(v); err != nil {
		res.status = "rejected: " + err.Error()
	} else {
		res.cfg.endpoint = e
		res.status = "endpoint = " + e
	}
	return res
}

// menu draws the menu and applies exactly one key press.
func menu(c config, status string, in *input) menuResult {
	res := menuResult{cfg: c, status: status}
	fmt.Fprint(out, clearScreen, fitLines(screen(c, status), uiWidth))
	choice, ok := in.key()
	if !ok {
		res.quit = true
		return res
	}
	if choice == "\x03" || choice == "\x04" { // Ctrl-C, Ctrl-D
		res.quit = true
		return res
	}
	switch strings.ToLower(choice) {
	case "1":
		return menuIMEI(res, c, in)
	case "2":
		return menuInterval(res, c)
	case "c":
		return menuCustomInterval(res, in)
	case "3":
		return menuEndpoint(res, c, in)
	case "4":
		res.start = true
	case "q":
		res.quit = true
	default:
		if printable(choice) {
			res.status = fmt.Sprintf("unknown key %q", choice)
		}
	}
	return res
}

func runScreen(c config, lines []string, sent, failed int) string {
	var b strings.Builder
	b.WriteString("\n  RUNNING\n  ────────────────────────────────────────────────────────\n\n")
	fmt.Fprintf(&b, "  GET %s\n", c.endpoint)
	fmt.Fprintf(&b, "  imei %s   interval %s   sent %d   failed %d\n\n", c.imei, c.interval, sent, failed)
	for _, l := range lines {
		fmt.Fprintf(&b, "  %s\n", l)
	}
	b.WriteString("\n  [s] stop   [q] quit\n")
	return b.String()
}

// run sends fixes until the user stops it or a report fails.
func run(c config, in *input) (summary string, quit bool) {
	client := &http.Client{Timeout: 5 * time.Second}
	p := newPosition()
	var lines []string
	sent, failed := 0, 0

	addLog := func(s string) {
		lines = append(lines, time.Now().Format("15:04:05")+"  "+s)
		if len(lines) > logCapacity() {
			lines = lines[len(lines)-logCapacity():]
		}
	}
	draw := func(sent, failed int) {
		fmt.Fprint(out, clearScreen, fitLines(runScreen(c, lines, sent, failed), uiWidth))
	}

	draw(0, 0)
	for {
		if err := send(client, c, p); err != nil {
			failed++
			addLog("FAILED: " + err.Error())
			addLog("stopped")
			draw(sent, failed)
			return fmt.Sprintf("run stopped after %d sent, %d failed: %v", sent, failed, err), false
		}
		sent++
		addLog(fmt.Sprintf("200 OK   lat %.6f  lon %.6f  speed %.1f kn", p.lat, p.lon, p.speedMPS*knotsPerMPS))
		draw(sent, failed)

		p.step(c.interval)
		select {
		case <-time.After(c.interval):
		case key, ok := <-in.keys:
			if !ok {
				return fmt.Sprintf("stdin closed after %d sent, %d failed", sent, failed), true
			}
			switch strings.ToLower(key) {
			case "s":
				return fmt.Sprintf("stopped by user after %d sent, %d failed", sent, failed), false
			case "q", "\x03", "\x04": // q, Ctrl-C, Ctrl-D
				return fmt.Sprintf("quit after %d sent, %d failed", sent, failed), true
			case "":
			default:
				if printable(key) {
					addLog("unknown key " + strconv.Quote(key) + " (use s or q)")
					draw(sent, failed)
				}
			}
		}
	}
}

func main() {
	selftestFlag := flag.Bool("selftest", false, "run built-in logic checks and exit")
	flag.Parse()
	if *selftestFlag {
		runSelftest()
		return
	}

	in := newInput()
	defer in.close()
	// Without raw mode a stray Ctrl-C would leave the terminal unusable.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)
	go func() {
		<-sig
		in.close()
		fmt.Fprint(out, "\r\n")
		os.Exit(0)
	}()

	c := config{imei: defaultIMEI, interval: defaultInterval, endpoint: defaultEndpoint}
	status := "ready — press [4] to start"
	for {
		res := menu(c, status, in)
		if res.quit {
			fmt.Fprint(out, "\r\n")
			return
		}
		c, status = res.cfg, res.status
		if res.start {
			summary, quit := run(c, in)
			if quit {
				fmt.Fprint(out, "\r\n")
				return
			}
			status = summary
		}
	}
}

// runSelftest exercises the pure logic: no network, no TUI.
func runSelftest() {
	assert := func(ok bool, format string, a ...any) {
		if !ok {
			fmt.Printf("SELFTEST FAIL: "+format+"\n", a...)
			os.Exit(1)
		}
	}

	got, err := normalizeEndpoint("localhost:5055")
	assert(err == nil && got == "http://localhost:5055", "normalizeEndpoint bare host = %q, %v", got, err)
	got, err = normalizeEndpoint("http://example.com:1/osmand")
	assert(err == nil && got == "http://example.com:1/osmand", "normalizeEndpoint full url = %q, %v", got, err)
	_, err = normalizeEndpoint("http://")
	assert(err != nil, "normalizeEndpoint should reject missing host")

	for in, want := range map[string]time.Duration{"1": time.Second, "10": 10 * time.Second, " 30 ": 30 * time.Second, "3600": time.Hour} {
		d, err := parseInterval(in)
		assert(err == nil && d == want, "parseInterval(%q) = %v, %v", in, d, err)
	}
	// Whole seconds only: no millisecond values, no decimal fractions.
	for _, bad := range []string{"0", "-1", "3601", "abc", "", "0.5", "1.5", "500ms", "1e3", "1s", "+5"} {
		_, err := parseInterval(bad)
		assert(err != nil, "parseInterval(%q) should fail", bad)
	}

	var nl strings.Builder
	_, werr := crlfWriter{&nl}.Write([]byte("a\nb\r\nc"))
	assert(werr == nil && nl.String() == "a\r\nb\r\nc", "crlfWriter produced %q", nl.String())

	long := strings.Repeat("x", 200)
	assert(len([]rune(fit(long, 40))) == 39, "fit should cut to width-1, got %d runes", len([]rune(fit(long, 40))))
	assert(fit("short", 40) == "short", "fit should leave short lines alone")
	assert(strings.HasSuffix(fit(long, 40), "…"), "fit should mark the cut")

	assert(validIMEI(defaultIMEI), "default IMEI should be valid")
	assert(!validIMEI("35756551452718"), "14-digit IMEI should be invalid")
	assert(!validIMEI("35756551452718a"), "non-digit IMEI should be invalid")

	c := config{imei: defaultIMEI, interval: time.Second, endpoint: defaultEndpoint}
	p := newPosition()
	raw := c.target(p, time.Unix(1700000000, 0))
	u, err := url.Parse(raw)
	assert(err == nil, "target is not a URL: %q", raw)
	q := u.Query()
	assert(u.Host == "localhost:5055", "target host = %q", u.Host)
	assert(u.Path == "", "target path = %q, want empty", u.Path)
	assert(q.Get("id") == defaultIMEI, "target id = %q", q.Get("id"))
	assert(q.Get("timestamp") == "1700000000", "target timestamp = %q, want epoch seconds", q.Get("timestamp"))
	assert(q.Get("charge") == "false", "target charge = %q", q.Get("charge"))
	lat, _ := strconv.ParseFloat(q.Get("lat"), 64)
	lon, _ := strconv.ParseFloat(q.Get("lon"), 64)
	assert(math.Abs(lat-seedLat) < 1e-9 && math.Abs(lon-seedLon) < 1e-9, "first fix should start at the seed, got %v,%v", lat, lon)
	kts, _ := strconv.ParseFloat(q.Get("speed"), 64)
	assert(kts > 10 && kts < 25, "speed %v kn out of plausible range for ~30 km/h", kts)
	for _, k := range []string{"valid", "bearing", "altitude", "accuracy", "hdop", "batt"} {
		assert(q.Get(k) != "", "target missing parameter %q", k)
	}

	for i := 0; i < 20000; i++ {
		p.step(time.Second)
		assert(math.Abs(p.lat-seedLat) <= boxDeg+1e-9 && math.Abs(p.lon-seedLon) <= boxDeg+1e-9,
			"walk escaped the box at step %d: %.6f,%.6f", i, p.lat, p.lon)
		assert(p.heading >= 0 && p.heading < 360, "heading out of range at step %d: %v", i, p.heading)
	}
	assert(p.lat != seedLat || p.lon != seedLon, "walk never moved")

	fmt.Println("selftest ok")
}
