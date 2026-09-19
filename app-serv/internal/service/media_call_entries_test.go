// Package service implements the management-plane use cases of app-serv.
//
// @file      internal/service/media_call_entries_test.go
// @for       The registry the §7.10 media call tests read.
// @uses      internal/registry.
// @reason    The entries are data, not behavior: keeping them beside the doubles
//
//	would push media_stub_test.go past the AGENTS.md §1.1 line budget as
//	G5 ports more adapters, and a test that adds a provider should add one
//	entry here rather than reshape the fixture.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package service

import "github.com/rusmanadodi2598/pannelAI/app-serv/internal/registry"

// mediaCallEntries is the registry the media call tests read.
func mediaCallEntries() []registry.Provider {
	return []registry.Provider{
		{
			ID: "openai", Display: registry.Display{Name: "OpenAI"}, Priority: 1, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.openai.com/v1/audio/speech", AuthType: registry.AuthAPIKey,
					AuthHeader: "bearer", Format: "openai", DefaultModel: "gpt-4o-mini-tts",
					Voices: []registry.MediaVoice{{ID: "alloy", Name: "Alloy"}},
				},
				registry.MediaSTT: {
					BaseURL: "https://api.openai.com/v1/audio/transcriptions", Format: "openai",
				},
				registry.MediaImage: {
					BaseURL: "https://api.openai.com/v1/images/generations", Format: "openai",
					DefaultModel: "gpt-image-1",
				},
			},
		},
		{
			ID: "deepgram", Alias: "dg", Display: registry.Display{Name: "Deepgram"}, Priority: 2, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaSTT: {
					BaseURL: "https://api.deepgram.com/v1/listen", AuthType: registry.AuthAPIKey,
					AuthHeader: "token", Format: "deepgram",
				},
			},
		},
		{
			ID: "nvidia", Display: registry.Display{Name: "NVIDIA"}, Priority: 3, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL:  "https://integrate.api.nvidia.com/v1/audio/speech",
					AuthType: registry.AuthAPIKey, AuthHeader: "bearer", Format: "nvidia-tts",
				},
			},
		},
		{
			ID: "cartesia", Alias: "cartesia", Display: registry.Display{Name: "Cartesia"}, Priority: 4, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.cartesia.ai/tts/bytes", AuthType: registry.AuthAPIKey,
					AuthHeader: "x-api-key", Format: "cartesia",
					Models: []registry.MediaModel{{ID: "sonic-2", Name: "Sonic 2"}, {ID: "sonic-3", Name: "Sonic 3"}},
				},
			},
		},
		{
			ID: "elevenlabs", Display: registry.Display{Name: "ElevenLabs"}, Priority: 5, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.elevenlabs.io/v1/text-to-speech", AuthType: registry.AuthAPIKey,
					AuthHeader: "xi-api-key", Format: "elevenlabs", DefaultModel: "eleven_multilingual_v2",
				},
			},
		},
		{
			// A format the gateway has no adapter for yet (G21): the refusal
			// must name it rather than send a payload its API rejects. Edge TTS
			// needs a scraped token, so it is not a single-request adapter.
			ID: "edge-tts", Display: registry.Display{Name: "Edge TTS"}, Priority: 6, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "edge-tts", Format: "edge-tts", DefaultModel: "en-US-AriaNeural",
				},
			},
		},
		{
			ID: "minimax", Display: registry.Display{Name: "MiniMax"}, Priority: 7, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.minimax.io/v1/t2a_v2", AuthType: registry.AuthAPIKey,
					AuthHeader: "bearer", Format: "minimax-tts",
				},
			},
		},
		{
			ID: "minimax-cn", Display: registry.Display{Name: "MiniMax CN"}, Priority: 8, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.minimaxi.com/v1/t2a_v2", AuthType: registry.AuthAPIKey,
					AuthHeader: "bearer", Format: "minimax-tts",
				},
			},
		},
		{
			ID: "inworld", Display: registry.Display{Name: "Inworld"}, Priority: 9, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.inworld.ai/tts/v1/voice", AuthType: registry.AuthAPIKey,
					AuthHeader: "basic", Format: "inworld",
				},
			},
		},
		{
			ID: "playht", Display: registry.Display{Name: "PlayHT"}, Priority: 10, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://api.play.ht/api/v2/tts/stream", AuthType: registry.AuthAPIKey,
					AuthHeader: "playht", Format: "playht",
				},
			},
		},
		{
			ID: "coqui", Display: registry.Display{Name: "Coqui"}, Priority: 11, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "http://localhost:5002/api/tts", AuthType: registry.AuthNone,
					AuthHeader: "none", Format: "coqui",
				},
			},
		},
		{
			ID: "tortoise", Display: registry.Display{Name: "Tortoise"}, Priority: 12, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "http://localhost:5000/api/tts", AuthType: registry.AuthNone,
					AuthHeader: "none", Format: "tortoise",
				},
			},
		},
		{
			ID: "gemini", Display: registry.Display{Name: "Gemini"}, Priority: 13, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL:  "https://generativelanguage.googleapis.com/v1beta/models",
					AuthType: registry.AuthAPIKey, AuthHeader: "key", Format: "gemini-tts",
				},
			},
		},
		{
			ID: "selfhosted", Display: registry.Display{Name: "Self Hosted"}, Priority: 3, Category: "media",
			Media: registry.MediaConfigs{
				registry.MediaImage: {Format: "openai", DefaultModel: "sd-xl"},
			},
		},
		{
			ID: "brave-search", Display: registry.Display{Name: "Brave"}, Priority: 4, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaWebSearch: {
					BaseURL: "https://api.search.brave.com/res/v1", Method: "GET",
					QueryParam: "q", MaxResultsParam: "count",
					DefaultMaxResults: 5, MaxMaxResults: 20, CostPerQuery: 0.005,
				},
			},
		},
		{
			// A provider whose own model ids carry a slash, which is why the
			// speech route does not read a voice out of the model string.
			ID: "openrouter", Display: registry.Display{Name: "OpenRouter"}, Priority: 5, Category: "apikey",
			Media: registry.MediaConfigs{
				registry.MediaTTS: {
					BaseURL: "https://openrouter.ai/api/v1/chat/completions", Format: "openai",
					DefaultModel: "openai/gpt-4o-mini-tts",
				},
			},
		},
	}
}
