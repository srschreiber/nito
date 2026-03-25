package components

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/qeesung/image2ascii/convert"
	"github.com/srschreiber/nito/shellapp/commands"
	"github.com/srschreiber/nito/shellapp/connection"
	"github.com/srschreiber/nito/shellapp/history"
	"github.com/srschreiber/nito/shellapp/styles"
	"github.com/srschreiber/nito/shellapp/types"
)

const maxCmdHistory = 20

// chatOpDef describes a chat op for autocomplete.
type chatOpDef struct {
	name    string // e.g. ".image"
	argHint string // e.g. "<filename>"
}

var chatOps = []chatOpDef{
	{name: ".image", argHint: "<filename> [-h <height>]"},
}

// completeChatOp returns the first op whose name starts with prefix, or nil.
func completeChatOp(prefix string) *chatOpDef {
	if !strings.HasPrefix(prefix, ".") || strings.Contains(prefix, " ") {
		return nil
	}
	for i := range chatOps {
		if strings.HasPrefix(chatOps[i].name, prefix) {
			return &chatOps[i]
		}
	}
	return nil
}

type cursorBlinkMsg struct{ gen int }

const (
	placeholderCmd  = "Type a command... (try: wcid)"
	placeholderChat = "Chat (/cmd to return to command mode)"
)

type CommandComponent struct {
	Placeholder    string
	focused        bool
	chatMode       bool
	passwordMode   bool
	textFieldValue string
	cursorPos      int
	cursorVisible  bool
	blinkGen       int
	width          int
	// command history (up/down navigation)
	cmdHistory []string
	historyIdx int    // -1 = not navigating
	draftText  string // saved input before navigating history
}

func NewCommandComponent(width int) *CommandComponent {
	return &CommandComponent{
		Placeholder:   placeholderCmd,
		cursorVisible: true,
		historyIdx:    -1,
		width:         width,
		cmdHistory:    history.Load(),
	}
}

func (c *CommandComponent) SetWidth(width int) {
	c.width = width
}

func (l *CommandComponent) newBlinkCmd() tea.Cmd {
	gen := l.blinkGen
	return tea.Tick(time.Millisecond*530, func(time.Time) tea.Msg {
		return cursorBlinkMsg{gen: gen}
	})
}

// resetCursor makes the cursor immediately visible and restarts the blink cycle.
func (l *CommandComponent) resetCursor() tea.Cmd {
	l.cursorVisible = true
	l.blinkGen++
	return l.newBlinkCmd()
}

func (l *CommandComponent) Init() tea.Cmd {
	return l.newBlinkCmd()
}

func (l *CommandComponent) SetFocused(focused bool) {
	l.focused = focused
}

func (l *CommandComponent) Update(msg tea.Msg) tea.Cmd {
	switch msg := msg.(type) {
	case types.RoomSelectedMsg:
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: "switched to room " + msg.RoomID + " — use /chat to switch to chat mode", isResponse: true},
			}}
		}
	case cursorBlinkMsg:
		if msg.gen != l.blinkGen {
			return nil // stale tick from before last reset
		}
		l.cursorVisible = !l.cursorVisible
		return l.newBlinkCmd()

	case tea.PasteMsg:
		text := msg.Content
		if text != "" {
			runes := []rune(l.textFieldValue)
			l.textFieldValue = string(runes[:l.cursorPos]) + text + string(runes[l.cursorPos:])
			l.cursorPos += len([]rune(text))
			return l.resetCursor()
		}

	case tea.KeyPressMsg:
		// Every key interaction resets the cursor to visible.
		blink := l.resetCursor()

		var keyCmd tea.Cmd
		switch msg.String() {
		case "left", "ctrl+b":
			if l.cursorPos > 0 {
				l.cursorPos--
			}
		case "right", "ctrl+f":
			if l.cursorPos < len([]rune(l.textFieldValue)) {
				l.cursorPos++
			}
		case "ctrl+a":
			l.cursorPos = 0
		case "ctrl+e":
			l.cursorPos = len([]rune(l.textFieldValue))
		case "ctrl+k":
			l.textFieldValue = string([]rune(l.textFieldValue)[:l.cursorPos])
		case "ctrl+d":
			runes := []rune(l.textFieldValue)
			if l.cursorPos < len(runes) {
				l.textFieldValue = string(append(runes[:l.cursorPos], runes[l.cursorPos+1:]...))
			}
		case "up":
			if len(l.cmdHistory) > 0 {
				if l.historyIdx == -1 {
					l.draftText = l.textFieldValue
					l.historyIdx = len(l.cmdHistory) - 1
				} else if l.historyIdx > 0 {
					l.historyIdx--
				}
				l.textFieldValue = l.cmdHistory[l.historyIdx]
				l.cursorPos = len([]rune(l.textFieldValue))
			}
		case "down":
			if l.historyIdx != -1 {
				if l.historyIdx == len(l.cmdHistory)-1 {
					l.historyIdx = -1
					l.textFieldValue = l.draftText
				} else {
					l.historyIdx++
					l.textFieldValue = l.cmdHistory[l.historyIdx]
				}
				l.cursorPos = len([]rune(l.textFieldValue))
			}
		case "tab":
			if tpl := l.completionTemplate(); tpl != "" {
				l.textFieldValue = tpl
				l.cursorPos = len([]rune(tpl))
			}
		case "enter":
			if l.textFieldValue != "" {
				keyCmd = l.handleEnter()
			}
		case "backspace":
			if l.cursorPos > 0 {
				runes := []rune(l.textFieldValue)
				runes = append(runes[:l.cursorPos-1], runes[l.cursorPos:]...)
				l.textFieldValue = string(runes)
				l.cursorPos--
			}
		default:
			text := msg.Key().Text
			if text != "" {
				runes := []rune(l.textFieldValue)
				l.textFieldValue = string(runes[:l.cursorPos]) + text + string(runes[l.cursorPos:])
				l.cursorPos += len([]rune(text))
			}
		}

		if keyCmd != nil {
			return tea.Batch(keyCmd, blink)
		}
		return blink
	}
	return nil
}

// pendingPasswordSignal tracks which flow the current password prompt belongs to.
var pendingPasswordSignal commands.Signal

func (l *CommandComponent) handlePasswordSubmit(password string) tea.Cmd {
	var out string
	var signal commands.Signal
	var err error

	switch pendingPasswordSignal {
	case commands.SignalNeedRegisterPassword:
		out, signal, err = commands.CompleteRegister(password)
	default:
		out, signal, err = commands.CompleteLogin(password)
	}
	pendingPasswordSignal = commands.SignalNone

	entries := []historyEntry{{text: "> [password]"}}
	if err != nil {
		entries = append(entries, historyEntry{text: err.Error(), isResponse: true})
	} else if out != "" {
		entries = append(entries, historyEntry{text: out, isResponse: true})
	}

	userID := ""
	if s := connection.CurrentSession(); s != nil {
		userID = s.UserID
	}
	connMsg := types.ConnectionStatusMsg{
		Connected: connection.IsConnected(),
		BrokerURL: connection.BrokerURL(),
		UserID:    userID,
	}
	emitConn := func() tea.Msg { return connMsg }

	if signal == commands.SignalConnected {
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.ConnectedMsg{} },
		)
	}
	return tea.Batch(func() tea.Msg { return AppendHistoryMsg{Entries: entries} }, emitConn)
}

func (l *CommandComponent) handleChatOp(input string) tea.Cmd {
	parts := strings.SplitN(input, " ", 2)
	op := parts[0]
	switch op {
	case ".image":
		if len(parts) < 2 || strings.TrimSpace(parts[1]) == "" {
			return func() tea.Msg {
				return AppendHistoryMsg{Entries: []historyEntry{
					{text: ".image: usage: .image <filename> [-h <height>]", isResponse: true},
				}}
			}
		}
		// Parse: .image <filename> [-h|-height <n>]
		tokens := strings.Fields(parts[1])
		filename := ""
		height := 0
		for i := 0; i < len(tokens); i++ {
			if (tokens[i] == "-h" || tokens[i] == "--height") && i+1 < len(tokens) {
				n, err := strconv.Atoi(tokens[i+1])
				if err == nil {
					height = n
				}
				i++
			} else if filename == "" {
				filename = tokens[i]
			}
		}
		if filename == "" {
			return func() tea.Msg {
				return AppendHistoryMsg{Entries: []historyEntry{
					{text: ".image: usage: .image <filename> [-h <height>]", isResponse: true},
				}}
			}
		}
		return imageOp(filename, height)
	default:
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: "unknown op: " + op, isResponse: true},
			}}
		}
	}
}

// imageOp loads an image from ~/.nito/images/<filename>, converts it to ASCII
// art scaled to fit within maxW×maxH (preserving aspect ratio), and appends it
// to the conversation history. height overrides the default max height (capped
// at 100); pass 0 to use the default.
func imageOp(filename string, height int) tea.Cmd {
	home, err := os.UserHomeDir()
	if err != nil {
		errMsg := err.Error()
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: ".image: " + errMsg, isResponse: true},
			}}
		}
	}

	// Resolve image path: try ~/.nito/images/<basename> first, then cwd/.nito/images/<basename>.
	base := filepath.Base(filename)
	nitoPath := filepath.Join(home, ".nito", "images", base)
	var imagePath string
	if _, statErr := os.Stat(nitoPath); statErr == nil {
		imagePath = nitoPath
	} else {
		cwd, cwdErr := os.Getwd()
		if cwdErr != nil {
			errMsg := cwdErr.Error()
			return func() tea.Msg {
				return AppendHistoryMsg{Entries: []historyEntry{
					{text: ".image: " + errMsg, isResponse: true},
				}}
			}
		}
		imagePath = filepath.Join(cwd, ".nito", "images", base)
	}

	// Read image dimensions to compute aspect-ratio-preserving fit within 50x50.
	f, err := os.Open(imagePath)
	if err != nil {
		errMsg := err.Error()
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: ".image: " + errMsg, isResponse: true},
			}}
		}
	}
	cfg, _, err := image.DecodeConfig(f)
	f.Close()
	if err != nil {
		errMsg := err.Error()
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: ".image: decode config: " + errMsg, isResponse: true},
			}}
		}
	}

	const defaultMax = 100
	maxW := defaultMax
	maxH := defaultMax
	if height > 0 {
		if height > 100 {
			height = 100
		}
		maxH = height
	}
	w, h := fitAspect(cfg.Width, cfg.Height, maxW, maxH)

	converter := convert.NewImageConverter()
	options := convert.DefaultOptions
	options.FixedWidth = w
	options.FixedHeight = h
	options.Colored = true

	ascii := converter.ImageFile2ASCIIString(imagePath, &options)
	if strings.TrimSpace(ascii) == "" {
		return func() tea.Msg {
			return AppendHistoryMsg{Entries: []historyEntry{
				{text: ".image: failed to convert '" + filename + "' (check it exists in ~/.nito/images/)", isResponse: true},
			}}
		}
	}

	text := ascii
	return func() tea.Msg {
		return AppendHistoryMsg{Entries: []historyEntry{
			{text: "> .image " + filename},
			{text: text, isRaw: true},
		}}
	}
}

// fitAspect returns width and height that fit within maxW×maxH while preserving
// the original aspect ratio.
func fitAspect(srcW, srcH, maxW, maxH int) (int, int) {
	if srcW <= 0 || srcH <= 0 {
		return maxW, maxH
	}
	scaleW := float64(maxW) / float64(srcW)
	scaleH := float64(maxH) / float64(srcH)
	scale := scaleW
	if scaleH < scale {
		scale = scaleH
	}
	w := int(float64(srcW) * scale)
	h := int(float64(srcH) * scale)
	if w < 1 {
		w = 1
	}
	if h < 1 {
		h = 1
	}
	return w, h
}

func (l *CommandComponent) handleEnter() tea.Cmd {
	input := l.textFieldValue
	l.textFieldValue = ""
	l.cursorPos = 0

	if l.passwordMode {
		l.passwordMode = false
		l.Placeholder = placeholderCmd
		return l.handlePasswordSubmit(input)
	}

	l.cmdHistory = append(l.cmdHistory, input)
	if len(l.cmdHistory) > maxCmdHistory {
		l.cmdHistory = l.cmdHistory[1:]
	}
	l.historyIdx = -1

	// Mode-switch commands are intercepted before anything else.
	if input == "/chat" {
		l.chatMode = true
		l.Placeholder = placeholderChat
		return tea.Batch(
			func() tea.Msg {
				return AppendHistoryMsg{Entries: []historyEntry{
					{text: "> " + input},
					{text: "Switched to chat mode. Type messages and press enter to send.", isResponse: true},
				}}
			},
			func() tea.Msg { return ModeChangedMsg{ChatMode: true} },
		)
	}
	if input == "/cmd" {
		l.chatMode = false
		l.Placeholder = placeholderCmd
		return tea.Batch(
			func() tea.Msg {
				return AppendHistoryMsg{Entries: []historyEntry{
					{text: "> " + input},
					{text: "Switched to command mode.", isResponse: true},
				}}
			},
			func() tea.Msg { return ModeChangedMsg{ChatMode: false} },
		)
	}

	// In chat mode, plain input is sent as a room message.
	if l.chatMode {
		if strings.HasPrefix(input, ".") {
			return l.handleChatOp(input)
		}
		entries := []historyEntry{{text: "[you]: " + input}}
		if err := commands.SendRoomMessage(input); err != nil {
			entries = append(entries, historyEntry{text: err.Error(), isResponse: true})
		}
		return func() tea.Msg { return AppendHistoryMsg{Entries: entries} }
	}

	entries := []historyEntry{
		{text: "> " + input},
	}

	output, signal, err := commands.ExecCommand(input)
	if err != nil {
		entries = append(entries, historyEntry{text: err.Error(), isResponse: true})
	} else if output != "" {
		entries = append(entries, historyEntry{text: output, isResponse: true})
	}

	userID := ""
	if s := connection.CurrentSession(); s != nil {
		userID = s.UserID
	}
	connMsg := types.ConnectionStatusMsg{
		Connected: connection.IsConnected(),
		BrokerURL: connection.BrokerURL(),
		UserID:    userID,
	}
	emitConn := func() tea.Msg { return connMsg }

	switch signal {
	case commands.SignalClear:
		return tea.Batch(func() tea.Msg { return ClearHistoryMsg{} }, emitConn)
	case commands.SignalExit:
		_ = history.Save(l.cmdHistory)
		return tea.Quit
	case commands.SignalRefreshRooms:
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.RoomsFetchMsg{} },
		)
	case commands.SignalNeedPassword, commands.SignalNeedRegisterPassword:
		pendingPasswordSignal = signal
		l.passwordMode = true
		l.Placeholder = "Password:"
		entries = append(entries, historyEntry{text: "Password:", isResponse: true})
		return tea.Batch(func() tea.Msg { return AppendHistoryMsg{Entries: entries} }, emitConn)
	case commands.SignalConnected:
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.ConnectedMsg{} },
		)
	case commands.SignalRoomSelected:
		roomID := connection.GetSessionRoomID()
		if roomID == nil {
			break
		}
		id := *roomID
		entries = append(entries, historyEntry{text: "use /chat to switch to chat mode", isResponse: true})
		return tea.Batch(
			func() tea.Msg { return AppendHistoryMsg{Entries: entries} },
			emitConn,
			func() tea.Msg { return types.RoomSelectedMsg{RoomID: id} },
		)
	case commands.SignalJump:
		line := commands.JumpLine
		return func() tea.Msg { return JumpScrollMsg{Line: line} }
	}

	return tea.Batch(func() tea.Msg { return AppendHistoryMsg{Entries: entries} }, emitConn)
}

// ghostSuffix returns the grey inline suggestion text to display after the
// current input, or "" if there is nothing to suggest. In command mode it
// completes command names; in chat mode it completes .op names.
func (l *CommandComponent) ghostSuffix() string {
	text := l.textFieldValue
	if len([]rune(text)) < 1 || strings.Contains(text, " ") {
		return ""
	}
	if l.cursorPos != len([]rune(text)) {
		return ""
	}
	if l.chatMode {
		op := completeChatOp(text)
		if op == nil {
			return ""
		}
		return op.name[len(text):] + " " + op.argHint
	}
	if strings.HasPrefix(text, "/") {
		return ""
	}
	def := commands.CompletePrefix(text)
	if def == nil {
		return ""
	}
	suffix := def.Name[len(text):]
	for _, arg := range def.Args {
		flag := arg.Long
		if flag == "" {
			flag = arg.Short
		}
		suffix += fmt.Sprintf(" --%s <%s>", flag, flag)
	}
	return suffix
}

// HasSuggestion reports whether there is an active inline autocomplete suggestion.
func (l *CommandComponent) HasSuggestion() bool {
	return l.ghostSuffix() != ""
}

// completionTemplate returns the full completed string to use on Tab, or "".
func (l *CommandComponent) completionTemplate() string {
	text := l.textFieldValue
	if len([]rune(text)) < 1 || strings.Contains(text, " ") {
		return ""
	}
	if l.chatMode {
		op := completeChatOp(text)
		if op == nil {
			return ""
		}
		return op.name + " " + op.argHint
	}
	if strings.HasPrefix(text, "/") {
		return ""
	}
	def := commands.CompletePrefix(text)
	if def == nil {
		return ""
	}
	result := def.Name
	for _, arg := range def.Args {
		flag := arg.Long
		if flag == "" {
			flag = arg.Short
		}
		result += fmt.Sprintf(" --%s <%s>", flag, flag)
	}
	return result
}

func (l *CommandComponent) Render() string {
	prompt := styles.PromptStyle.Render("> ")
	runes := []rune(l.textFieldValue)
	if l.passwordMode {
		runes = []rune(strings.Repeat("*", len(runes)))
	}
	ghost := l.ghostSuffix()

	var render string
	if l.focused && l.cursorVisible {
		before := string(runes[:l.cursorPos])
		if l.cursorPos < len(runes) {
			underCursor := styles.CursorHighlightStyle.Render(string(runes[l.cursorPos]))
			render = prompt + before + underCursor + string(runes[l.cursorPos+1:])
		} else if ghost != "" {
			// Use the first ghost char as the cursor highlight so the ghost
			// text never shifts when the cursor blinks.
			gr := []rune(ghost)
			render = prompt + before + styles.CursorHighlightStyle.Render(string(gr[0])) + styles.Grey.Render(string(gr[1:]))
		} else if len(runes) > 0 {
			render = prompt + before + styles.CursorHighlightStyle.Render(" ")
		} else {
			// Empty input — highlight the first char of the placeholder in place
			pr := []rune(l.Placeholder)
			render = prompt + styles.CursorHighlightStyle.Render(string(pr[0])) + styles.Grey.Render(string(pr[1:]))
		}
	} else {
		if len(runes) > 0 {
			render = prompt + string(runes) + styles.Grey.Render(ghost)
		} else {
			render = prompt + styles.Grey.Render(l.Placeholder)
		}
	}

	style := styles.UnfocusedBorderStyle
	if l.focused {
		style = styles.FocusedBorderStyle
	}
	if l.width > 0 {
		style = style.Width(l.width)
	}
	return style.Render(render)
}
