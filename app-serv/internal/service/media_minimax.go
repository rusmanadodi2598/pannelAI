// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_minimax.go
// @for       The MiniMax speech adapter in the §7.10 media plane.
// @uses      internal/dataplane, internal/schema, encoding/hex, encoding/json,
//
//	strings.
//
// @reason    MiniMax answers hex-encoded audio inside a JSON envelope that
//
//	carries its own status code, so a 200 can be a failure. The
//	adapter keeps the request shape and the answer reading together,
//	and the answer reader is what tells the shared pipeline that the
//	call failed (media_perform.go).
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import (
	"encoding/hex"
	"encoding/json"
	"strings"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/dataplane"
	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/schema"
)

// MiniMax's documented defaults, which the reference sends too. The route asks
// for mp3 because that is the label it reports for this provider.
const (
	minimaxDefaultVoice = "English_expressive_narrator"
	minimaxSampleRate   = 32000
	minimaxBitrate      = 128000
)

// minimaxSpeechBody is the non-streaming request MiniMax's T2A endpoint accepts.
type minimaxSpeechBody struct {
	Model         string             `json:"model"`
	Text          string             `json:"text"`
	Stream        bool               `json:"stream"`
	LanguageBoost string             `json:"language_boost"`
	OutputFormat  string             `json:"output_format"`
	VoiceSetting  minimaxVoice       `json:"voice_setting"`
	AudioSetting  minimaxAudioOutput `json:"audio_setting"`
}

// minimaxVoice keeps the nested voice shape typed at the JSON boundary
// (AGENTS.md §1.4).
type minimaxVoice struct {
	VoiceID string  `json:"voice_id"`
	Speed   float64 `json:"speed"`
	Volume  float64 `json:"vol"`
	Pitch   float64 `json:"pitch"`
}

// minimaxAudioOutput fixes the container the route reports: mp3 at the
// reference's sample rate and bitrate.
type minimaxAudioOutput struct {
	SampleRate int    `json:"sample_rate"`
	Bitrate    int    `json:"bitrate"`
	Format     string `json:"format"`
	Channel    int    `json:"channel"`
}

// minimaxSpeechRequest builds the provider's body. It asks for hex because that
// is the envelope the reference decodes, and `stream: false` because the route
// answers with one buffered body rather than a stream.
func minimaxSpeechRequest(req schema.SpeechRequest, model string) minimaxSpeechBody {
	voice := strings.TrimSpace(req.Voice)
	if voice == "" {
		voice = minimaxDefaultVoice
	}
	return minimaxSpeechBody{
		Model: model, Text: req.Input, Stream: false, LanguageBoost: "auto", OutputFormat: "hex",
		VoiceSetting: minimaxVoice{VoiceID: voice, Speed: 1, Volume: 1, Pitch: 0},
		AudioSetting: minimaxAudioOutput{
			SampleRate: minimaxSampleRate, Bitrate: minimaxBitrate, Format: "mp3", Channel: 1,
		},
	}
}

// minimaxSpeechAnswer decodes the hex audio a non-streaming MiniMax answer
// carries.
//
// MiniMax reports a failure inside a 200 (`base_resp.status_code`), so a
// non-zero status is an upstream rejection rather than a served call: recording
// it as served would put a failure the client saw into the Usage screen as a
// success.
func minimaxSpeechAnswer(status int, body []byte) ([]byte, error) {
	var answer struct {
		Data struct {
			Audio string `json:"audio"`
		} `json:"data"`
		BaseResp struct {
			StatusCode int    `json:"status_code"`
			StatusMsg  string `json:"status_msg"`
		} `json:"base_resp"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		return nil, dataplane.InternalError("the speech answer could not be read", err)
	}
	if answer.BaseResp.StatusCode != 0 {
		return nil, dataplane.UpstreamRejected(status, minimaxFailureMessage(answer.BaseResp.StatusMsg))
	}
	audio, err := hex.DecodeString(strings.TrimSpace(answer.Data.Audio))
	if err != nil {
		return nil, dataplane.UpstreamRejected(status, "the speech upstream returned an unreadable audio payload")
	}
	if len(audio) == 0 {
		return nil, dataplane.UpstreamRejected(status, "the speech upstream returned no audio")
	}
	return audio, nil
}

// minimaxFailureMessage names the provider's own message, or a generic one when
// it sent none, so the log row always says why.
func minimaxFailureMessage(message string) string {
	if trimmed := strings.TrimSpace(message); trimmed != "" {
		return trimmed
	}
	return "the speech upstream rejected the request"
}
