package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"agy-tele/internal/renderer"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// Rich message transport via Telegram Bot API 10.1+.
//
// The pinned library (go-telegram-bot-api v5.5.1) predates Rich Messages, so
// we call sendRichMessage / editMessageText{rich_message} over plain HTTPS
// with the same bot token. All sends are fail-closed to the legacy HTML path:
// any rejection (old server, parser error, oversize) falls back to NewMessage
// / NewEditMessageText so the reply is never lost.

// richHTTPTimeout bounds every rich API call; streaming edits must not hang.
const richHTTPTimeout = 20 * time.Second

// richInputMessage is the InputRichMessage payload: exactly one of
// markdown / html / blocks must be set. We always use the markdown field
// with Rich Markdown syntax (GFM-compatible + arbitrary HTML).
type richInputMessage struct {
	Markdown string `json:"markdown"`
}

type richSendRequest struct {
	ChatID              interface{}      `json:"chat_id"`
	RichMessage         richInputMessage `json:"rich_message"`
	ReplyMarkup         interface{}      `json:"reply_markup,omitempty"`
	DisableNotification bool             `json:"disable_notification,omitempty"`
}

type richEditRequest struct {
	ChatID      interface{}      `json:"chat_id"`
	MessageID   int              `json:"message_id"`
	RichMessage richInputMessage `json:"rich_message"`
	ReplyMarkup interface{}      `json:"reply_markup,omitempty"`
}

type richAPIResponse struct {
	OK          bool            `json:"ok"`
	Description string          `json:"description,omitempty"`
	Result      json.RawMessage `json:"result,omitempty"`
}

// richEnabled reports whether native Rich Messages may be attempted.
func (r *Router) richEnabled() bool {
	return r.cfg.Telegram.RichMessages
}

// richPost performs a raw Bot API call and returns the decoded envelope.
func (r *Router) richPost(method string, payload interface{}) (*richAPIResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.telegram.org/bot%s/%s", r.cfg.Telegram.BotToken, method)
	client := &http.Client{Timeout: richHTTPTimeout}
	resp, err := client.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	var env richAPIResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", method, err)
	}
	return &env, nil
}

// sendRichMessageRaw sends pre-normalized Rich Markdown. Returns the sent
// message id on success, 0 + error otherwise (caller falls back to HTML).
func (r *Router) sendRichMessageRaw(chatID int64, richMarkdown string, kb *tgbotapi.InlineKeyboardMarkup) (int, error) {
	var markup interface{}
	if kb != nil {
		markup = kb
	}
	env, err := r.richPost("sendRichMessage", richSendRequest{
		ChatID:      chatID,
		RichMessage: richInputMessage{Markdown: richMarkdown},
		ReplyMarkup: markup,
	})
	if err != nil {
		return 0, err
	}
	if !env.OK {
		return 0, fmt.Errorf("sendRichMessage rejected: %s", env.Description)
	}
	var msg struct {
		MessageID int `json:"message_id"`
	}
	if err := json.Unmarshal(env.Result, &msg); err != nil {
		return 0, fmt.Errorf("decode sendRichMessage result: %w", err)
	}
	return msg.MessageID, nil
}

// editRichMessageRaw replaces a message's content via
// editMessageText{rich_message}. Returns nil on success.
func (r *Router) editRichMessageRaw(chatID int64, messageID int, richMarkdown string, kb *tgbotapi.InlineKeyboardMarkup) error {
	var markup interface{}
	if kb != nil {
		markup = kb
	}
	env, err := r.richPost("editMessageText", richEditRequest{
		ChatID:      chatID,
		MessageID:   messageID,
		RichMessage: richInputMessage{Markdown: richMarkdown},
		ReplyMarkup: markup,
	})
	if err != nil {
		return err
	}
	if !env.OK {
		// Inline messages and some message types cannot carry rich content.
		if strings.Contains(env.Description, "message can't be edited") ||
			strings.Contains(env.Description, "message to edit not found") {
			return fmt.Errorf("editMessageText rich not applicable: %s", env.Description)
		}
		return fmt.Errorf("editMessageText rich rejected: %s", env.Description)
	}
	return nil
}

// sendRichOrHTML is the single choke point for rich-capable final answers:
// try native Rich first (tables, headings, task lists, details, math render
// natively), fall back to the legacy HTML renderer on any failure.
// Returns the sent message id (0 when even fallback failed) and whether the
// message actually went out over the rich path (false when any chunk fell
// back to HTML — the reply is still delivered, just not native).
func (r *Router) sendRichOrHTML(chatID int64, rawMarkdown string, kb *tgbotapi.InlineKeyboardMarkup) (int, bool) {
	if !r.richEnabled() || !renderer.ShouldUseRich(rawMarkdown) {
		return r.sendHTML(chatID, renderer.FormatMarkdownForTelegram(rawMarkdown), kb), false
	}
	rich := renderer.NormalizeForRich(rawMarkdown)
	chunks := renderer.SplitRichChunks(rich, renderer.RichMaxChars)
	firstID := 0
	richUsed := true
	for i, ch := range chunks {
		if id, err := r.sendRichMessageRaw(chatID, ch, onlyLastKB(kb, i, len(chunks))); err == nil {
			if firstID == 0 {
				firstID = id
			}
			continue
		} else {
			log.Printf("[agy-tele] rich send failed (chunk %d/%d), falling back to HTML: %v", i+1, len(chunks), err)
		}
		richUsed = false
		// Fallback: legacy HTML for this chunk.
		html := renderer.FormatMarkdownForTelegram(ch)
		if id := r.sendHTML(chatID, html, onlyLastKB(kb, i, len(chunks))); id != 0 && firstID == 0 {
			firstID = id
		}
	}
	return firstID, richUsed
}

// sendHTML sends legacy HTML with plain-text fallback. Returns message id.
func (r *Router) sendHTML(chatID int64, htmlText string, kb *tgbotapi.InlineKeyboardMarkup) int {
	msg := tgbotapi.NewMessage(chatID, htmlText)
	msg.ParseMode = "HTML"
	if kb != nil {
		msg.ReplyMarkup = kb
	}
	if sent, err := r.bot.Send(msg); err == nil {
		return sent.MessageID
	}
	plain := tgbotapi.NewMessage(chatID, renderer.StripHTML(htmlText))
	if kb != nil {
		plain.ReplyMarkup = kb
	}
	if sent, err := r.bot.Send(plain); err == nil {
		return sent.MessageID
	} else {
		log.Printf("[agy-tele] HTML+plain send failed: %v", err)
	}
	return 0
}

// editRichOrHTML replaces message content: rich first, HTML fallback.
func (r *Router) editRichOrHTML(chatID int64, messageID int, rawMarkdown string, kb *tgbotapi.InlineKeyboardMarkup) bool {
	if r.richEnabled() && renderer.ShouldUseRich(rawMarkdown) {
		rich := renderer.NormalizeForRich(rawMarkdown)
		if len(rich) <= renderer.RichMaxChars {
			if err := r.editRichMessageRaw(chatID, messageID, rich, kb); err == nil {
				return true
			} else {
				log.Printf("[agy-tele] rich edit failed, falling back to HTML: %v", err)
			}
		}
	}
	html := renderer.FormatMarkdownForTelegram(rawMarkdown)
	edit := tgbotapi.NewEditMessageText(chatID, messageID, html)
	edit.ParseMode = "HTML"
	if kb != nil {
		edit.ReplyMarkup = kb
	}
	if _, err := r.bot.Send(edit); err == nil {
		return true
	}
	editPlain := tgbotapi.NewEditMessageText(chatID, messageID, renderer.StripHTML(html))
	if kb != nil {
		editPlain.ReplyMarkup = kb
	}
	_, err := r.bot.Send(editPlain)
	return err == nil
}

// finalizeRich upgrades a streamed answer to native Rich: the complete
// markdown is sent via sendRichMessage (native table/heading/task
// rendering) as a fresh message, then the partial streaming message is
// deleted so the chat shows one clean final message. On any failure it
// returns 0 WITHOUT touching the streaming message, and the caller falls
// back to the legacy HTML finalize (which edits the streaming message
// in place).
// The footer (duration/tokens/version) is appended as markdown italic —
// Rich Markdown has no footer block, and it stays readable in fallback too.
func (r *Router) finalizeRich(chatID int64, buf interface {
	Stop() string
	MessageID() int
}, rawMarkdown, footer string, fallbackActions []string, kb *tgbotapi.InlineKeyboardMarkup) int {
	rich := renderer.NormalizeForRich(rawMarkdown)
	if footer != "" {
		rich = rich + "\n" + footerRichLine(footer)
	}
	if id, _ := r.sendRichOrHTML(chatID, rich, kb); id != 0 {
		// Rich landed: halt the streaming editor, then remove the partial.
		buf.Stop()
		if msgID := buf.MessageID(); msgID != 0 {
			_, _ = r.bot.Send(tgbotapi.NewDeleteMessage(chatID, msgID))
		}
		return id
	}
	return 0
}

// footerRichLine converts the legacy HTML footer (an <i>(...)</i> line) into
// Rich Markdown italic. The footer contains no links, only emoji + text.
func footerRichLine(footerHTML string) string {
	s := renderer.StripHTML(footerHTML)
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	return "_" + s + "_"
}

// onlyLastKB attaches the keyboard to the final chunk only so inline buttons
// are not duplicated across split messages.
func onlyLastKB(kb *tgbotapi.InlineKeyboardMarkup, i, n int) *tgbotapi.InlineKeyboardMarkup {
	if kb != nil && i == n-1 {
		return kb
	}
	return nil
}
