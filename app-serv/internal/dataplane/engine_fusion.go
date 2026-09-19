// Package dataplane routes a client request through the gateway: it resolves the
// model string, picks an upstream endpoint and key, translates the wire format,
// and performs the outbound call.
//
// @file      internal/dataplane/engine_fusion.go
// @for       The fusion combo strategy: fan the prompt out to every member, then
//
//	let the judge model synthesize one final answer (SPEC-API-001 §7.7).
//
// @uses      context, sync.
// @reason    SPEC-API-001 §7.7 defines fusion as fan-out plus synthesis, and the
//
//	reference fixes three semantics worth keeping: panel calls are
//	non-streaming with tools withdrawn so each member answers in prose the
//	judge can read, the judge keeps the client's stream flag and tools so
//	the served answer is what the client asked for, and a panel that
//	returns one answer is not fused. The request shaping those decisions
//	need lives beside this file, in fusion_prompt.go.
//
// @author    Dodi Rusmana <rusmanadodi@kentangtech.com>
// @layer     service
// @stability experimental
// @since     2026-09-19
package dataplane

import (
	"context"
	"strings"
	"sync"

	"github.com/rusmanadodi2598/pannelAI/app-serv/internal/domain"
)

// relayFusion serves a request addressed to a fusion combo.
//
// Latency is reported as the whole panel-and-judge duration the client waited,
// not the judge call alone: a fan-out hides its cost from a field that measures
// one upstream call.
func (e *Engine) relayFusion(ctx context.Context, in Request, resolution Resolution, sink FrameSink) (Outcome, error) {
	started := e.clock()
	members, resolveErr := e.fusionMembers(ctx, resolution)
	if len(members) == 0 {
		return Outcome{}, resolveErr
	}

	panel, err := panelRequest(in)
	if err != nil {
		return Outcome{}, err
	}
	answers, answerErr := e.fanOut(ctx, panel, resolution.Combo, members)

	var outcome Outcome
	switch len(answers) {
	case 0:
		return Outcome{}, answerErr
	case 1:
		// One model answered, so there is nothing to fuse: answering directly
		// keeps its own identity and spends no judge call. A client that asked
		// for a stream needs a real one, and the panel call was prose-only and
		// non-streamed, so the surviving member is asked again with the
		// client's own request.
		if !in.Stream {
			outcome = answers[0].outcome
			break
		}
		outcome, err = e.relayOnce(ctx, in, answers[0].member, sink)
		if err != nil {
			return Outcome{}, err
		}
	default:
		outcome, err = e.judge(ctx, in, resolution, answers, sink)
		if err != nil {
			return Outcome{}, err
		}
	}
	outcome.LatencyMS = e.clock().Sub(started).Milliseconds()
	if outcome.LatencyMS < 0 {
		outcome.LatencyMS = 0
	}
	return outcome, nil
}

// fusionMembers resolves every reference of the combo. A reference that no
// longer resolves is skipped rather than failing the panel: the remaining
// members can still answer, and the panel is where a broken reference is
// reported by omission.
func (e *Engine) fusionMembers(ctx context.Context, resolution Resolution) ([]Resolution, error) {
	refs := resolution.Combo.Refs()
	members := make([]Resolution, 0, len(refs))
	var lastErr error
	for _, ref := range refs {
		member, err := e.resolver.Resolve(ctx, ref)
		if err != nil {
			lastErr = err
			continue
		}
		members = append(members, member)
	}
	return members, lastErr
}

// fusionAnswer is one panel member's served answer and the text the judge reads.
// The member is carried too, so the answer can be re-issued to the same model
// when the client's request needs a shape the panel call did not have.
type fusionAnswer struct {
	member  Resolution
	outcome Outcome
	text    string
}

// fanOut calls every panel member in parallel and collects the prose answers, in
// member order so the judge's source labels are stable.
//
// Each member writes only its own result slot, and each goroutine recovers from
// a panic: AGENTS.md §1.6 makes the recovery non-negotiable, and a member that
// panicked must cost its own answer rather than the whole process.
func (e *Engine) fanOut(ctx context.Context, in Request, combo domain.Combo, members []Resolution) ([]fusionAnswer, error) {
	type result struct {
		answer fusionAnswer
		err    error
	}
	results := make([]result, len(members))
	var wg sync.WaitGroup
	for index, member := range members {
		wg.Add(1)
		go func(index int, member Resolution) {
			defer wg.Done()
			defer func() {
				if recovered := recover(); recovered != nil {
					results[index] = result{err: internalError("a fusion panel member panicked", nil)}
				}
			}()
			member.Combo = combo
			outcome, err := e.relayOnce(ctx, in, member, nil)
			if err != nil {
				results[index] = result{err: err}
				return
			}
			text := strings.TrimSpace(answerText(outcome.Body, in.ClientFormat))
			if text == "" {
				results[index] = result{err: dataPlaneError(CodeUpstreamError,
					"the panel member returned no text to fuse")}
				return
			}
			results[index] = result{answer: fusionAnswer{member: member, outcome: outcome, text: text}}
		}(index, member)
	}
	wg.Wait()

	answers := make([]fusionAnswer, 0, len(results))
	var firstErr error
	for _, result := range results {
		if result.err != nil {
			if firstErr == nil {
				firstErr = result.err
			}
			continue
		}
		answers = append(answers, result.answer)
	}
	return answers, firstErr
}

// judge asks the combo's judge model to synthesize one answer from the panel.
// The judge receives the client's own request plus one appended user turn, so
// its stream flag and tool declarations survive to the served call.
func (e *Engine) judge(ctx context.Context, in Request, resolution Resolution, answers []fusionAnswer, sink FrameSink) (Outcome, error) {
	judgeResolution, err := e.resolver.Resolve(ctx, resolution.Combo.JudgeModel())
	if err != nil {
		return Outcome{}, err
	}
	judgeResolution.Combo = resolution.Combo
	request, err := judgeRequest(in, buildJudgePrompt(answers))
	if err != nil {
		return Outcome{}, err
	}
	return e.relayOnce(ctx, request, judgeResolution, sink)
}
