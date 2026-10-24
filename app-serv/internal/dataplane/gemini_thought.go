// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/gemini_thought.go
// @for       Which Google product a Gemini wire is addressed to, and the thought signature a replayed tool call is answered with.
// @uses      internal/dataplane Resolution, internal/registry Provider.
// @reason    Gemini 3 and newer reject a history functionCall part that carries no thoughtSignature, and a client on any of the OpenAI wires has nowhere to keep the signature the model sent: it echoes the call id and the arguments, and the signature is gone by the next turn. The reference answers by signing the first call of every model turn with a filler the vendor accepts, and leaving the siblings unsigned; this file names that filler per product because the two Cloud Code Assist products only accept their own.
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-10-18
package dataplane

// geminiProduct names the Google product a Gemini request is addressed to. The
// three share one wire, so the format does not distinguish them, and the
// signature they accept is per product rather than per model.
type geminiProduct string

const (
	geminiProductAPI         geminiProduct = "gemini"
	geminiProductAntigravity geminiProduct = "antigravity"
	geminiProductGeminiCLI   geminiProduct = "gemini-cli"
)

// geminiProductFor maps a registry entry to its product. An id the Google family
// does not name (a custom node on the Gemini wire) reads as the API, which is what
// the reference's standard-API path signs with.
func geminiProductFor(providerID string) geminiProduct {
	switch providerID {
	case string(geminiProductGeminiCLI):
		return geminiProductGeminiCLI
	case string(geminiProductAntigravity):
		return geminiProductAntigravity
	default:
		return geminiProductAPI
	}
}

// Vendor-captured fillers. They are opaque tokens Google issued for a real
// thinking turn, not values this repo derived, and both are the reference's own
// constants (open-sse/config/defaultThinkingSignature.js, the values
// openaiToGeminiRequest and openaiToGeminiCLIRequest pass). Draft 061 §8 carries
// the live round that has to confirm Google still accepts them.
const (
	geminiThoughtSignatureAPI       = "EuwGCukGAXLI2nxwZIq54WWSoL/YN0P3TsDZ7zRnLi8g0S4aVr2HUGxvaHKySuY6HAVzcE0GPGjXrytLIldxthSvfxgUlJh6Qa9Z+Oj5QZBlYdg6HaJ6yuY5R7waE6rdwBsRf7Ft2j3DJ9rMi9qhWFqApewYtPhls3VHtuvND3l8Rm09+lbAXQs6KKWEWrxNLKTBkfpMgXhRERc/TQRMZu1twAablm6/Zk1tsYRvfWKLsNbeKF+CCojJdXJKvnR/8Ouuoa+Y2Ti20hcW7aZIIjZDFYPU//k6Ybmhg69J/imbFai2ckhfLaisqdDkdoIiBJScTOUvYqP6AE9d4MsydSC+UlhIMk4hoP76R8vUSCZRMkjOaDXstf/QoVZKbt94wyRZgAJ1G0BqI8L5ow86kLpA4wJEtxsRGymOE4bKUvApveBakYDNM9APkf+LbtbzWSseGjoZcSlycF9iN8Q2XNYKRrHbv3Lr5Y8JjdH/5y/6SHkNehTEZugaeGnSPSyCTWto1kQgHpxdWmhkLfJGNUGLmue7Mesj4TSms4J33mRpYVhNB/J333FCqIP0hr/E7BkkjEn7yZ4X7SQlh+xKPurapsnHRwiKmtsilmEFrnTE9iQr+pMr6M29qqFNv1tr5yumbaJw8JW9sB15tNsRv+dW6BjNanbsKz7HCgKUBc8tGy+7YuhXzAfViyRefcjK7eZW0Fbyt7AbybJTKz78W8NH7ye6LAwzOebXpeZ4D43fNIt8bKh26qgduSQv/7o+pAflkuqHZ99YWgHQ8h8OkZFi3eOiSYjsjhdZ/czWOdoPI/OnqIldzMPF5YlrKBLFX8VhRKVmqgsmWf5PHGulHhMkVlS+XG2UIseGy69ARa93D78Gsa+1n1kJr7EEB7Rh+27vUMxVYLdz1yMSvE5nalTAlg/ZeG8+XQ0cHuAI3KbQpHW2Q++RdXfm5JzD5WdJZUU+Zn8t8UUn85BH4RxZLeE0qJikgSsKoYVBc6YhiMjhPgkR95ReimY4Z0xCJdRo1gjexOFeODZMpQF6Yxnoic7IrdgsFA3iePTbFnPp3IAM1fAThWhXJUn3QInUOTd5o1qmTmn6REbL15g/JQNl+dqUoPkhleeb2V3kjqp1okmO3wMZbPknR3S1LZNmlS72/iBQUm+n2b/RCn4PjmM2"
	geminiThoughtSignatureGeminiCLI = "CiQBjz1rX/AlslZWMe5RgBt4Tv9j4+YNZTTez+JH2/+5oAlICygKXgGPPWtf7/Sux9eLYap/bmYAdPqFThLXj+l7o0DLu/hdgU98MA9ZrlRDNHXx+T0tuY8AcnjPZbiDyOq2bE11Fjhsk6p5axqayaapC/Pt9GczcgIQf1z15WTxCeKWAPYKYQGPPWtfDYj0nlNFNoTlU39RC91Z16xFKJ2MLEmkm+NvimsoOJ6be3g2BssNPtJ/9BKDXRA5cVs17tBeeW72lH8TMB5999udtxHM2SiUsnWsrHlfVuGSCpNQQ+5REw8HNvEKkgEBjz1rXzBNWrqZGbjun55K+vgYPBhJO2qZ67uRWXUA5/qcU12U/mbi5XoA3swoxYE8LEXfZvFFC9WG/W28QNCA0Qd4Trk/WkWiAwZmB8a84Fs14rkv3wqyxwFavPkJorqurAfd2XzGiFy0sB0ITCOPYi1HzDGV5WfXk6b9k+jT66/RuzGa8EcSOWo/QtC3Bkhgowo4AY89a1/f/tw8A02zjIoK7JVDAbf8W4UfmbApJJhwXIiGtu1M0JItObx7g2reYqT+HHL2Q/R4VDc="
)

// thoughtSignature is the filler this product's backend accepts on a replayed
// tool call. Every Gemini entry answers with one; an unsigned call is what the
// model refuses.
func (p geminiProduct) thoughtSignature() string {
	if p == geminiProductGeminiCLI {
		return geminiThoughtSignatureGeminiCLI
	}
	return geminiThoughtSignatureAPI
}

// geminiProduct names the Google product the resolved entry answers as. One wire
// serves all three, so the entry id is the only thing that tells them apart.
func (r Resolution) geminiProduct() geminiProduct {
	return geminiProductFor(r.Provider.ID)
}
