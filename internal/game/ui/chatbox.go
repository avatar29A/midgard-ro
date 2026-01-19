// Package ui provides game user interface components.
package ui

import (
	"fmt"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
)

// ChatMessage represents a single chat message.
type ChatMessage struct {
	Timestamp time.Time
	Channel   ChatChannel
	Sender    string
	Message   string
	Color     imgui.Vec4
}

// ChatChannel represents the type of chat channel.
type ChatChannel uint8

const (
	ChatChannelNormal ChatChannel = iota // White - normal chat
	ChatChannelParty                     // Green - party chat
	ChatChannelGuild                     // Yellow - guild chat
	ChatChannelWhisper                   // Pink - whisper
	ChatChannelGlobal                    // Orange - global/world chat
	ChatChannelSystem                    // Red - system messages
	ChatChannelAnnounce                  // Blue - server announcements
)

// ChatBox renders the game chat interface.
type ChatBox struct {
	// Messages
	messages    []ChatMessage
	maxMessages int

	// Input
	inputBuffer  string
	inputFocused bool

	// Display settings
	ShowTimestamp bool
	ShowChannel   bool
	AutoScroll    bool
	Opacity       float32

	// Channel filter
	ActiveChannel ChatChannel
	ShowAll       bool

	// Callbacks
	OnSendMessage func(channel ChatChannel, message string)

	// State
	scrollToBottom bool
}

// NewChatBox creates a new chat box.
func NewChatBox() *ChatBox {
	return &ChatBox{
		messages:      make([]ChatMessage, 0, 100),
		maxMessages:   100,
		ShowTimestamp: false,
		ShowChannel:   true,
		AutoScroll:    true,
		Opacity:       0.8,
		ActiveChannel: ChatChannelNormal,
		ShowAll:       true,
	}
}

// AddMessage adds a new message to the chat.
func (cb *ChatBox) AddMessage(channel ChatChannel, sender, message string) {
	msg := ChatMessage{
		Timestamp: time.Now(),
		Channel:   channel,
		Sender:    sender,
		Message:   message,
		Color:     cb.channelColor(channel),
	}

	cb.messages = append(cb.messages, msg)

	// Remove old messages if exceeding limit
	if len(cb.messages) > cb.maxMessages {
		cb.messages = cb.messages[1:]
	}

	if cb.AutoScroll {
		cb.scrollToBottom = true
	}
}

// AddSystemMessage adds a system message.
func (cb *ChatBox) AddSystemMessage(message string) {
	cb.AddMessage(ChatChannelSystem, "System", message)
}

// Clear removes all messages.
func (cb *ChatBox) Clear() {
	cb.messages = cb.messages[:0]
}

// Render renders the chat box at the specified position and size.
func (cb *ChatBox) Render(x, y, width, height float32) {
	imgui.SetNextWindowPos(imgui.NewVec2(x, y))
	imgui.SetNextWindowSize(imgui.NewVec2(width, height))

	flags := imgui.WindowFlagsNoMove | imgui.WindowFlagsNoResize |
		imgui.WindowFlagsNoCollapse

	imgui.PushStyleVarFloat(imgui.StyleVarWindowRounding, 5)
	imgui.SetNextWindowBgAlpha(cb.Opacity)

	if imgui.BeginV("Chat###ChatBox", nil, flags) {
		// Channel tabs
		cb.renderChannelTabs()

		imgui.Separator()

		// Messages area
		cb.renderMessages(height - 85) // Reserve space for input

		imgui.Separator()

		// Input field
		cb.renderInput()
	}
	imgui.End()

	imgui.PopStyleVar()
}

func (cb *ChatBox) renderChannelTabs() {
	if imgui.BeginTabBar("ChatTabs") {
		if imgui.BeginTabItem("All") {
			cb.ShowAll = true
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Normal") {
			cb.ShowAll = false
			cb.ActiveChannel = ChatChannelNormal
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Party") {
			cb.ShowAll = false
			cb.ActiveChannel = ChatChannelParty
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Guild") {
			cb.ShowAll = false
			cb.ActiveChannel = ChatChannelGuild
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("Whisper") {
			cb.ShowAll = false
			cb.ActiveChannel = ChatChannelWhisper
			imgui.EndTabItem()
		}
		imgui.EndTabBar()
	}
}

func (cb *ChatBox) renderMessages(height float32) {
	// Child region for scrolling
	imgui.BeginChildStrV("ChatMessages", imgui.NewVec2(0, height), imgui.ChildFlagsNone, imgui.WindowFlagsHorizontalScrollbar)

	for _, msg := range cb.messages {
		// Filter by channel
		if !cb.ShowAll && msg.Channel != cb.ActiveChannel {
			continue
		}

		cb.renderMessage(msg)
	}

	// Auto-scroll to bottom
	if cb.scrollToBottom {
		imgui.SetScrollHereYV(1.0)
		cb.scrollToBottom = false
	}

	imgui.EndChild()
}

func (cb *ChatBox) renderMessage(msg ChatMessage) {
	var text string

	// Timestamp
	if cb.ShowTimestamp {
		text = fmt.Sprintf("[%s] ", msg.Timestamp.Format("15:04"))
	}

	// Channel prefix
	if cb.ShowChannel {
		switch msg.Channel {
		case ChatChannelParty:
			text += "[P] "
		case ChatChannelGuild:
			text += "[G] "
		case ChatChannelWhisper:
			text += "[W] "
		case ChatChannelGlobal:
			text += "[!] "
		case ChatChannelSystem:
			text += "[S] "
		case ChatChannelAnnounce:
			text += "[A] "
		}
	}

	// Sender and message
	if msg.Sender != "" && msg.Channel != ChatChannelSystem && msg.Channel != ChatChannelAnnounce {
		text += fmt.Sprintf("%s: %s", msg.Sender, msg.Message)
	} else {
		text += msg.Message
	}

	imgui.TextColored(msg.Color, text)
}

func (cb *ChatBox) renderInput() {
	// Input field takes most of the width
	inputWidth := imgui.ContentRegionAvail().X - 60

	imgui.PushItemWidth(inputWidth)

	// Input text field
	if imgui.InputTextWithHint("###ChatInput", "Type message...", &cb.inputBuffer, imgui.InputTextFlagsEnterReturnsTrue, nil) {
		cb.sendMessage()
	}

	imgui.PopItemWidth()

	imgui.SameLine()

	// Send button
	if imgui.Button("Send") {
		cb.sendMessage()
	}
}

func (cb *ChatBox) sendMessage() {
	if cb.inputBuffer == "" {
		return
	}

	message := cb.inputBuffer
	cb.inputBuffer = ""

	// Parse channel prefix
	channel := ChatChannelNormal
	if len(message) > 1 {
		switch message[0] {
		case '%':
			channel = ChatChannelParty
			message = message[1:]
		case '$':
			channel = ChatChannelGuild
			message = message[1:]
		case '!':
			channel = ChatChannelGlobal
			message = message[1:]
		}
	}

	// Check for whisper format: /w name message
	if len(message) > 3 && message[0:3] == "/w " {
		channel = ChatChannelWhisper
		// TODO: Parse whisper target
	}

	// Call the send callback
	if cb.OnSendMessage != nil {
		cb.OnSendMessage(channel, message)
	}
}

func (cb *ChatBox) channelColor(channel ChatChannel) imgui.Vec4 {
	switch channel {
	case ChatChannelNormal:
		return imgui.NewVec4(1.0, 1.0, 1.0, 1.0) // White
	case ChatChannelParty:
		return imgui.NewVec4(0.5, 1.0, 0.5, 1.0) // Green
	case ChatChannelGuild:
		return imgui.NewVec4(1.0, 1.0, 0.5, 1.0) // Yellow
	case ChatChannelWhisper:
		return imgui.NewVec4(1.0, 0.6, 0.8, 1.0) // Pink
	case ChatChannelGlobal:
		return imgui.NewVec4(1.0, 0.7, 0.3, 1.0) // Orange
	case ChatChannelSystem:
		return imgui.NewVec4(1.0, 0.4, 0.4, 1.0) // Red
	case ChatChannelAnnounce:
		return imgui.NewVec4(0.5, 0.7, 1.0, 1.0) // Blue
	default:
		return imgui.NewVec4(1.0, 1.0, 1.0, 1.0)
	}
}

// FocusInput sets focus to the chat input field.
func (cb *ChatBox) FocusInput() {
	cb.inputFocused = true
	// Note: Actual focus needs to be set via imgui.SetKeyboardFocusHere()
	// in the render loop after the input field is created
}

// IsFocused returns whether the chat input is focused.
func (cb *ChatBox) IsFocused() bool {
	return cb.inputFocused
}
