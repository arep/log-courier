package codecs

import (
	"github.com/driskell/log-courier/src/lc-lib/core"
	"sync"
	"testing"
	"time"
	"unicode"
)

func createMultilineCodec(unused map[string]interface{}, callback core.CodecCallbackFunc, t *testing.T) core.Codec {
	config := core.NewConfig()
	config.General.MaxLineBytes = 1048576
	config.General.SpoolMaxBytes = 10485760

	factory, err := NewMultilineCodecFactory(config, "", unused, "multiline")
	if err != nil {
		t.Errorf("Failed to create multiline codec: %s", err)
		t.FailNow()
	}

	return factory.NewCodec(callback, 0)
}

type checkMultilineExpect struct {
	start, end int64
	text string
}

type checkMultiline struct {
	expect []checkMultilineExpect
	t *testing.T

	mutex sync.Mutex
	lines int
}

func (c *checkMultiline) formatPrintable(text string) []rune {
	runes := []rune(text)
	for i, char := range runes {
		if unicode.IsPrint(char) {
			runes[i] = char
		} else {
			runes[i] = '.'
		}
	}
	return runes
}

func (c *checkMultiline) incorrectLineCount(lines int, message string) {
	c.t.Error(message)
	c.t.Errorf("Got:      %d", lines)
	c.t.Errorf("Expected: %d", len(c.expect))
}

func (c *checkMultiline) EventCallback(start_offset int64, end_offset int64, text string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	line := c.lines + 1

	if line > len(c.expect) {
		c.incorrectLineCount(line, "Too many lines received")
		c.t.FailNow()
	}

	if start_offset != c.expect[c.lines].start {
		c.t.Error("Start offset incorrect for line: ", line)
		c.t.Errorf("Got:      %d", start_offset)
		c.t.Errorf("Expected: %d", c.expect[c.lines].start)
	}

	if end_offset != c.expect[c.lines].end {
		c.t.Error("End offset incorrect for line: ", line)
		c.t.Errorf("Got:      %d", end_offset)
		c.t.Errorf("Expected: %d", c.expect[c.lines].end)
	}

	if text != c.expect[c.lines].text {
		c.t.Error("Text incorrect for line: ", line)
		c.t.Errorf("Got:      [%s]", c.formatPrintable(text))
		c.t.Errorf("Expected: [%s]", c.formatPrintable(c.expect[c.lines].text))
	}

	c.lines = line
}

func (c *checkMultiline) CheckCurrentCount(count int, message string) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	if c.lines != count {
		c.incorrectLineCount(c.lines, message)
	}
}

func (c *checkMultiline) CheckFinalCount() {
	c.CheckCurrentCount(len(c.expect), "Incorrect line count received")
}

func TestMultilinePrevious(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0, 5, "DEBUG First line\nNEXT line\nANOTHER line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern": "^(ANOTHER|NEXT) ",
			"what":    "previous",
			"negate":  false,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Event(4, 5, "ANOTHER line")
	codec.Event(6, 7, "DEBUG Next line")

	check.CheckFinalCount()

	if offset := codec.Teardown(); offset != 5 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilinePreviousNegate(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0, 5, "DEBUG First line\nNEXT line\nANOTHER line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern": "^DEBUG ",
			"what":    "previous",
			"negate":  true,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Event(4, 5, "ANOTHER line")
	codec.Event(6, 7, "DEBUG Next line")

	check.CheckFinalCount()

	if offset := codec.Teardown(); offset != 5 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilinePreviousTimeout(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0, 5, "DEBUG First line\nNEXT line\nANOTHER line"},
			{6, 7, "DEBUG Next line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern":          "^(ANOTHER|NEXT) ",
			"what":             "previous",
			"negate":           false,
			"previous timeout": "3s",
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Event(4, 5, "ANOTHER line")
	codec.Event(6, 7, "DEBUG Next line")

	// Allow a second
	time.Sleep(time.Second)

	check.CheckCurrentCount(1, "Timeout triggered too early")

	// Allow 5 seconds
	time.Sleep(5 * time.Second)

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 7 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilineNext(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0, 5, "DEBUG First line\nNEXT line\nANOTHER line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern": "^(DEBUG|NEXT) ",
			"what":    "next",
			"negate":  false,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Event(4, 5, "ANOTHER line")
	codec.Event(6, 7, "DEBUG Next line")

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 5 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilineNextNegate(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0, 5, "DEBUG First line\nNEXT line\nANOTHER line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern": "^ANOTHER ",
			"what":    "next",
			"negate":  true,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Event(4, 5, "ANOTHER line")
	codec.Event(6, 7, "DEBUG Next line")

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 5 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilineMaxBytes(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0,  32, "DEBUG First line\nsecond line\nthi"},
			{32, 39, "rd line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"max multiline bytes": int64(32),
			"pattern":             "^DEBUG ",
			"negate":              true,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0,  16, "DEBUG First line")
	codec.Event(17, 28, "second line")
	codec.Event(29, 39, "third line")
	codec.Event(40, 55, "DEBUG Next line")

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 39 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilineMaxBytesOverflow(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{0,  10, "START67890"},
			{10, 20, "abcdefg\n12"},
			{20, 30, "34567890ab"},
			{30, 39, "\nc1234567"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"max multiline bytes": int64(10),
			"pattern":             "^START",
			"negate":              true,
		},
		check.EventCallback,
		t,
	)

	// Ensure we reset state after each split (issue #188)
	// Also ensure we can split a single long line multiple times (issue #188)
	// Lastly, ensure we flush immediately if we receive max multiline bytes
	// rather than carrying over a full buffer and then crashing (issue #118)
	codec.Event(0,  17, "START67890abcdefg")
	codec.Event(18, 30, "1234567890ab")
	codec.Event(31, 39, "c1234567")
	codec.Event(40, 45, "START")

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 39 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}

func TestMultilineReset(t *testing.T) {
	check := &checkMultiline{
		expect: []checkMultilineExpect{
			{4, 7, "DEBUG Next line\nANOTHER line"},
		},
		t: t,
	}

	codec := createMultilineCodec(
		map[string]interface{}{
			"pattern": "^(ANOTHER|NEXT) ",
			"what":    "previous",
			"negate":  false,
		},
		check.EventCallback,
		t,
	)

	// Send some data
	codec.Event(0, 1, "DEBUG First line")
	codec.Event(2, 3, "NEXT line")
	codec.Reset()
	codec.Event(4, 5, "DEBUG Next line")
	codec.Event(6, 7, "ANOTHER line")
	codec.Event(8, 9, "DEBUG Last line")

	check.CheckFinalCount()

	offset := codec.Teardown()
	if offset != 7 {
		t.Error("Teardown returned incorrect offset: ", offset)
	}
}
